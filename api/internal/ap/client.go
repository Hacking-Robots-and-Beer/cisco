// Package ap implements an SSH client for Cisco autonomous IOS access points.
package ap

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	// execPromptRe matches the exec-mode prompt: e.g. "AP-Office-1#"
	execPromptRe = regexp.MustCompile(`[\r\n][A-Za-z0-9_\-\.]+#\s*$`)
	// configPromptRe matches the config-mode prompt: e.g. "AP-Office-1(config)#"
	configPromptRe = regexp.MustCompile(`[\r\n][A-Za-z0-9_\-\.]+\([^)]+\)#\s*$`)
)

// Client manages a persistent SSH session to a Cisco autonomous AP.
type Client struct {
	addr    string
	config  *ssh.ClientConfig
	conn    *ssh.Client
	session *ssh.Session
	stdin   *stdinWriter
	dataCh  chan []byte
	closeCh chan struct{}
}

// stdinWriter wraps ssh.StdinPipe to send CRLF line endings expected by IOS.
type stdinWriter struct {
	w interface {
		Write(p []byte) (n int, err error)
	}
}

func (s *stdinWriter) WriteCmd(cmd string) error {
	_, err := fmt.Fprintf(s.w, "%s\n", cmd)
	return err
}

// NewClient creates an AP SSH client. Call Connect() before using it.
func NewClient(hostname string, port int, username, password string) *Client {
	return &Client{
		addr: fmt.Sprintf("%s:%d", hostname, port),
		config: &ssh.ClientConfig{
			User: username,
			Auth: []ssh.AuthMethod{
				ssh.Password(password),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
			Timeout:         15 * time.Second,
		},
	}
}

// Connect establishes the SSH connection and opens an interactive shell.
func (c *Client) Connect() error {
	conn, err := ssh.Dial("tcp", c.addr, c.config)
	if err != nil {
		return fmt.Errorf("ssh dial %s: %w", c.addr, err)
	}
	c.conn = conn

	session, err := conn.NewSession()
	if err != nil {
		conn.Close()
		return fmt.Errorf("new session: %w", err)
	}
	c.session = session

	if err := session.RequestPty("vt100", 24, 200, ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 38400,
		ssh.TTY_OP_OSPEED: 38400,
	}); err != nil {
		conn.Close()
		return fmt.Errorf("request pty: %w", err)
	}

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		conn.Close()
		return fmt.Errorf("stdin pipe: %w", err)
	}
	c.stdin = &stdinWriter{w: stdinPipe}

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		conn.Close()
		return fmt.Errorf("stdout pipe: %w", err)
	}

	c.dataCh = make(chan []byte, 256)
	c.closeCh = make(chan struct{})

	// Start background reader goroutine
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				select {
				case c.dataCh <- data:
				case <-c.closeCh:
					return
				}
			}
			if err != nil {
				close(c.dataCh)
				return
			}
		}
	}()

	if err := session.Shell(); err != nil {
		c.Close()
		return fmt.Errorf("start shell: %w", err)
	}

	// Wait for initial exec prompt
	if _, err := c.readUntilPrompt(execPromptRe, 20*time.Second); err != nil {
		c.Close()
		return fmt.Errorf("wait for initial prompt: %w", err)
	}

	// Disable pagination — critical to avoid --More-- blocking
	if _, err := c.runExecCmd("terminal length 0"); err != nil {
		c.Close()
		return fmt.Errorf("terminal length 0: %w", err)
	}

	return nil
}

// Close releases the SSH connection.
func (c *Client) Close() {
	select {
	case <-c.closeCh:
	default:
		close(c.closeCh)
	}
	if c.session != nil {
		c.session.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

// RunCommand sends an IOS exec command and returns the output.
func (c *Client) RunCommand(cmd string) (string, error) {
	return c.runExecCmd(cmd)
}

// GetRunningConfig returns the full running configuration.
func (c *Client) GetRunningConfig() (string, error) {
	return c.runExecCmd("show running-config")
}

// GetAssociations returns the list of currently associated wireless clients.
func (c *Client) GetAssociations() ([]string, error) {
	out, err := c.runExecCmd("show dot11 associations")
	if err != nil {
		return nil, err
	}
	return strings.Split(out, "\n"), nil
}

// GetVersionInfo returns the raw output of "show version".
func (c *Client) GetVersionInfo() (string, error) {
	return c.runExecCmd("show version")
}

// GetRadioInfo returns the raw output of "show interfaces dot11Radio N".
func (c *Client) GetRadioInfo(radioNum int) (string, error) {
	return c.runExecCmd(fmt.Sprintf("show interfaces dot11Radio %d", radioNum))
}

// Configure enters global config mode, executes the provided commands, then
// exits config mode and saves the configuration.
func (c *Client) Configure(cmds []string) error {
	// Enter config mode
	if err := c.stdin.WriteCmd("configure terminal"); err != nil {
		return fmt.Errorf("send configure terminal: %w", err)
	}
	if _, err := c.readUntilPrompt(configPromptRe, 10*time.Second); err != nil {
		return fmt.Errorf("wait for config prompt: %w", err)
	}

	// Execute each command
	for _, cmd := range cmds {
		if cmd == "!" || cmd == "" {
			continue // comments/blank separators
		}
		if err := c.stdin.WriteCmd(cmd); err != nil {
			return fmt.Errorf("send command %q: %w", cmd, err)
		}
		// Read until either a config prompt or exec prompt (e.g. after "end")
		if _, err := c.readUntilEitherPrompt(5 * time.Second); err != nil {
			return fmt.Errorf("wait after command %q: %w", cmd, err)
		}
	}

	// Exit config mode
	if err := c.stdin.WriteCmd("end"); err != nil {
		return fmt.Errorf("send end: %w", err)
	}
	if _, err := c.readUntilPrompt(execPromptRe, 10*time.Second); err != nil {
		return fmt.Errorf("wait for exec prompt after end: %w", err)
	}

	// Save configuration
	if _, err := c.runExecCmd("write memory"); err != nil {
		return fmt.Errorf("write memory: %w", err)
	}

	return nil
}

// runExecCmd sends an exec-mode command and returns output (trimmed).
func (c *Client) runExecCmd(cmd string) (string, error) {
	if err := c.stdin.WriteCmd(cmd); err != nil {
		return "", fmt.Errorf("send command: %w", err)
	}
	out, err := c.readUntilPrompt(execPromptRe, 30*time.Second)
	if err != nil {
		return "", err
	}
	return cleanOutput(out, cmd), nil
}

// readUntilPrompt collects output until the given prompt pattern is seen.
func (c *Client) readUntilPrompt(prompt *regexp.Regexp, timeout time.Duration) (string, error) {
	var buf bytes.Buffer
	deadline := time.Now().Add(timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return buf.String(), fmt.Errorf("timeout waiting for prompt (got: %q)", buf.String())
		}

		select {
		case data, ok := <-c.dataCh:
			if !ok {
				return buf.String(), fmt.Errorf("connection closed")
			}
			buf.Write(data)
			if prompt.Match(buf.Bytes()) {
				return buf.String(), nil
			}
		case <-time.After(remaining):
			return buf.String(), fmt.Errorf("timeout waiting for prompt (got: %q)", buf.String())
		}
	}
}

// readUntilEitherPrompt reads until an exec or config prompt is seen.
func (c *Client) readUntilEitherPrompt(timeout time.Duration) (string, error) {
	var buf bytes.Buffer
	deadline := time.Now().Add(timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return buf.String(), fmt.Errorf("timeout waiting for prompt")
		}

		select {
		case data, ok := <-c.dataCh:
			if !ok {
				return buf.String(), fmt.Errorf("connection closed")
			}
			buf.Write(data)
			if execPromptRe.Match(buf.Bytes()) || configPromptRe.Match(buf.Bytes()) {
				return buf.String(), nil
			}
		case <-time.After(remaining):
			return buf.String(), fmt.Errorf("timeout waiting for prompt")
		}
	}
}

// cleanOutput strips the echoed command and trailing prompt from IOS output.
func cleanOutput(raw, cmd string) string {
	lines := strings.Split(raw, "\n")
	var result []string
	skipFirst := true
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		// Skip the echoed command line
		if skipFirst && strings.Contains(line, cmd) {
			skipFirst = false
			continue
		}
		// Skip the trailing prompt line
		if execPromptRe.MatchString("\n" + line) {
			continue
		}
		result = append(result, line)
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}
