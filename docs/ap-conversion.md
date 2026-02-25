# Cisco AIR-CAP2602 — Lightweight to Autonomous IOS Conversion

## Overview

Cisco AIR-CAP2602 access points ship in **lightweight (CAPWAP)** mode, designed to be managed by a Wireless LAN Controller (WLC). This guide converts them to **autonomous IOS** mode so they can be managed independently via SSH.

**Target firmware:** `ap3g2-k9w7-tar.153-3.JF11.tar`

---

## Method 1: MODE Button (No WLC Required)

Use this method when the AP has never been registered with a WLC, or when the WLC is unavailable.

### Prerequisites

- TFTP server running on a machine at **10.0.0.2** (or any address reachable from 10.0.0.0/24)
- Rename the firmware image to `ap3g2-k9w7-tar.default` on your TFTP server
- Connect a laptop directly to the AP's Ethernet port with a static IP of **10.0.0.2/24**

### Procedure

1. **Power off** the AP.

2. While holding the **MODE button** on the back of the AP, apply power.

3. Hold the MODE button for **20–30 seconds** until the LED turns **red**, then release.
   - The AP boots into ROMMON mode with IP **10.0.0.1**.

4. Verify connectivity from your TFTP server machine:
   ```
   ping 10.0.0.1
   ```

5. The AP will automatically download `ap3g2-k9w7-tar.default` from the TFTP server at **10.0.0.2** and flash it.
   - LED cycles through colours during the process (~5–10 minutes)
   - AP reboots automatically when complete

6. After reboot, the AP starts in autonomous IOS mode. Default login:
   - Username: `Cisco`
   - Password: `Cisco`

### TFTP Server Setup (Linux)

```bash
# Install tftpd-hpa
sudo apt install tftpd-hpa

# Copy firmware
sudo cp ap3g2-k9w7-tar.153-3.JF11.tar /srv/tftp/ap3g2-k9w7-tar.default
sudo chmod 644 /srv/tftp/ap3g2-k9w7-tar.default

# Start service
sudo systemctl start tftpd-hpa
sudo systemctl enable tftpd-hpa
```

---

## Method 2: Via WLC CLI

Use this method if the AP is currently registered to a Cisco WLC.

### Procedure

1. SSH into the WLC:
   ```
   ssh admin@<wlc-ip>
   ```

2. Find the AP name:
   ```
   (WLC)> show ap summary
   ```

3. Download the autonomous firmware directly from the AP:
   ```
   (WLC)> config ap tftp-downgrade <tftp-server-ip> ap3g2-k9w7-tar.153-3.JF11.tar <ap-name>
   ```

4. Wait for the AP to reboot (~10 minutes). It will come back in autonomous mode.

**Alternative — from the AP itself via WLC console:**
```
(WLC)> debug ap enable <ap-name>
(WLC)> debug ap command "archive download-sw /force-reload /overwrite tftp://<tftp-server>/ap3g2-k9w7-tar.153-3.JF11.tar" <ap-name>
```

---

## Post-Conversion Initial Configuration

After converting to autonomous mode, connect via console or Telnet (default) and apply this initial config:

```
! Enable SSH (disable Telnet)
configure terminal

! Set hostname
hostname AP-Office-1

! Configure management interface with static IP
interface BVI1
 ip address 192.168.1.10 255.255.255.0
 no shutdown

! Default gateway
ip default-gateway 192.168.1.1

! Create local user
username admin privilege 15 secret 0 <strong-password>

! Enable SSH v2
ip domain-name local.lan
crypto key generate rsa modulus 2048
ip ssh version 2
ip ssh time-out 60
ip ssh authentication-retries 3

! Disable Telnet on vty lines (SSH only)
line vty 0 4
 transport input ssh
 login local

! Disable HTTP server (use SSH only)
no ip http server
no ip http secure-server

! Save configuration
end
write memory
```

### Verify SSH Access

From your management machine:
```bash
ssh admin@192.168.1.10
```

Expected prompt:
```
AP-Office-1#
```

---

## Initial SSID Configuration (Autonomous Mode)

Example: Create a WPA2-PSK SSID on both 2.4 GHz and 5 GHz radios.

```
configure terminal

! Define SSID
dot11 ssid MyNetwork
 vlan 1
 authentication open
 authentication key-management wpa version 2
 wpa-psk ascii MySecurePassphrase

! Apply to 2.4 GHz radio (Radio 0)
interface Dot11Radio0
 ssid MyNetwork
 no shutdown

! Apply to 5 GHz radio (Radio 1)
interface Dot11Radio1
 ssid MyNetwork
 no shutdown

end
write memory
```

---

## Useful Show Commands

```
show version                          # Firmware version, uptime
show running-config                   # Full running config
show dot11 associations               # Connected clients
show interfaces dot11Radio 0          # 2.4 GHz radio status (channel, power)
show interfaces dot11Radio 1          # 5 GHz radio status
show ip interface brief               # IP addresses
show dot11 network-map                # Nearby APs (site survey)
```

---

## Reverting to Lightweight Mode

To convert back to CAPWAP/lightweight mode (for use with a WLC):

```bash
# Download the lightweight firmware via TFTP
archive download-sw /force-reload /overwrite tftp://<tftp-server>/ap3g2-k9w8-tar.153-3.JF11.tar
```

Note: `k9w8` = lightweight (CAPWAP), `k9w7` = autonomous.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| AP doesn't enter ROMMON | MODE button released too early | Hold until LED is red (>20s) |
| TFTP transfer fails | Firewall blocking UDP/69 | Open port 69 UDP on TFTP server host |
| SSH connection refused | SSH not configured | Use console cable, configure SSH |
| `crypto key generate rsa` fails | Hostname/domain not set | Set `hostname` and `ip domain-name` first |
| AP loops rebooting after flash | Wrong firmware file | Verify `k9w7` (autonomous) filename |
