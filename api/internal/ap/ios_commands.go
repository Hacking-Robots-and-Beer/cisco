package ap

import (
	"fmt"
	"strings"

	"github.com/hacking-robots-and-beer/cisco/api/internal/model"
)

// SSIDConfigCmds returns the IOS config-mode commands to create or update an SSID.
func SSIDConfigCmds(s model.SSID) []string {
	cmds := []string{}

	// Define the SSID globally
	cmds = append(cmds, fmt.Sprintf("dot11 ssid %s", s.Name))
	if s.VLAN > 0 && s.VLAN != 1 {
		cmds = append(cmds, fmt.Sprintf(" vlan %d", s.VLAN))
	}
	switch s.Security {
	case "wpa2-psk":
		cmds = append(cmds,
			" authentication open",
			" authentication key-management wpa version 2",
		)
		if s.Password != nil && *s.Password != "" {
			cmds = append(cmds, fmt.Sprintf(" wpa-psk ascii %s", *s.Password))
		}
	default: // open
		cmds = append(cmds, " authentication open")
	}
	cmds = append(cmds, "!")

	// Apply to radio interfaces
	if s.Radio == "2.4ghz" || s.Radio == "both" {
		cmds = append(cmds,
			"interface Dot11Radio0",
			fmt.Sprintf(" ssid %s", s.Name),
			"!",
		)
	}
	if s.Radio == "5ghz" || s.Radio == "both" {
		cmds = append(cmds,
			"interface Dot11Radio1",
			fmt.Sprintf(" ssid %s", s.Name),
			"!",
		)
	}

	return cmds
}

// DeleteSSIDCmds returns the IOS config-mode commands to remove an SSID.
func DeleteSSIDCmds(name string) []string {
	return []string{
		// Remove from radio interfaces first
		"interface Dot11Radio0",
		fmt.Sprintf(" no ssid %s", name),
		"!",
		"interface Dot11Radio1",
		fmt.Sprintf(" no ssid %s", name),
		"!",
		// Then remove the SSID definition
		fmt.Sprintf("no dot11 ssid %s", name),
		"!",
	}
}

// RadioConfigCmds returns the IOS config-mode commands to configure a radio interface.
func RadioConfigCmds(r model.RadioConfig) []string {
	var iface string
	switch r.Band {
	case "2.4ghz":
		iface = "Dot11Radio0"
	case "5ghz":
		iface = "Dot11Radio1"
	default:
		return nil
	}

	cmds := []string{fmt.Sprintf("interface %s", iface)}

	if !r.Enabled {
		cmds = append(cmds, " shutdown")
	} else {
		cmds = append(cmds, " no shutdown")
		if r.Channel > 0 {
			cmds = append(cmds, fmt.Sprintf(" channel %d", r.Channel))
		} else {
			cmds = append(cmds, " channel least-congested")
		}
		if r.TxPowerDBm > 0 {
			// IOS uses 1-8 power levels (1=max, 8=min) or dBm values
			// For simplicity, use dBm directly with "power local"
			cmds = append(cmds, fmt.Sprintf(" power local %d", r.TxPowerDBm))
		}
	}
	cmds = append(cmds, "!")

	return cmds
}

// ParseAssociations parses the output of "show dot11 associations" and returns
// a list of associated clients. The IOS output format varies by version; this
// parser handles the most common autonomous AP format.
func ParseAssociations(output string) []model.Client {
	var clients []model.Client
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Match lines that start with a MAC address pattern: xxxx.xxxx.xxxx
		if len(line) < 14 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		mac := fields[0]
		// Validate MAC: Cisco format is xxxx.xxxx.xxxx
		if !isCiscoMAC(mac) {
			continue
		}

		c := model.Client{
			MACAddress: normalizeMACAddress(mac),
		}

		// Try to extract IP address (second field if present and looks like IP)
		if len(fields) > 1 && isIPAddress(fields[1]) {
			ip := fields[1]
			c.IPAddress = &ip
		}

		clients = append(clients, c)
	}

	return clients
}

// ParseRadioInfo parses a line from "show interfaces dot11Radio 0/1" to extract
// channel and signal information.
func ParseRadioInfo(output, band string) *model.RadioConfig {
	rc := &model.RadioConfig{
		Band:    band,
		Enabled: true,
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)

		if strings.Contains(lower, "administratively down") {
			rc.Enabled = false
		}

		// Channel: "  Channel 6, BW 20..." or "Channel: 6"
		if strings.HasPrefix(lower, "channel") {
			var ch int
			if _, err := fmt.Sscanf(line, "Channel %d", &ch); err == nil {
				rc.Channel = ch
			}
		}
	}

	return rc
}

// ParseVersionOutput extracts model and firmware version from "show version" output.
func ParseVersionOutput(output string) (model string, firmware string) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// "Cisco IOS Software, ap3g2 Software (ap3g2-K9W7-M), Version 15.3(3)JF11"
		if strings.HasPrefix(line, "Cisco IOS Software") {
			if idx := strings.Index(line, "Version "); idx >= 0 {
				version := line[idx+8:]
				if comma := strings.Index(version, ","); comma >= 0 {
					version = version[:comma]
				}
				firmware = strings.TrimSpace(version)
			}
		}

		// "cisco AIR-CAP2602I-E-K9 (PowerPC) processor"
		if strings.HasPrefix(strings.ToLower(line), "cisco air-") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				model = fields[1]
			}
		}
	}
	return model, firmware
}

func isCiscoMAC(s string) bool {
	// Cisco format: xxxx.xxxx.xxxx (4 hex digits, dot, 4 hex digits, dot, 4 hex digits)
	if len(s) != 14 {
		return false
	}
	if s[4] != '.' || s[9] != '.' {
		return false
	}
	hexPart := s[:4] + s[5:9] + s[10:]
	for _, ch := range hexPart {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
	}
	return true
}

func normalizeMACAddress(cisco string) string {
	// Convert xxxx.xxxx.xxxx to xx:xx:xx:xx:xx:xx
	flat := strings.ReplaceAll(cisco, ".", "")
	if len(flat) != 12 {
		return cisco
	}
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		flat[0:2], flat[2:4], flat[4:6], flat[6:8], flat[8:10], flat[10:12])
}

func isIPAddress(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if len(p) == 0 || len(p) > 3 {
			return false
		}
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				return false
			}
		}
	}
	return true
}
