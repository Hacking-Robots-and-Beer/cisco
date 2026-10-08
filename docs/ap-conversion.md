# From a lightweight Cisco AP to a managed autonomous AP

This guide covers the whole process, from a factory-lightweight (CAPWAP) Cisco AP that does
nothing without a WLC to an AP that runs autonomous IOS and is managed by this controller's GUI.

1. [Identify the AP](#1-identify-the-ap)
2. [Get the firmware](#2-get-the-firmware)
3. [Build an isolated recovery network](#3-build-an-isolated-recovery-network)
4. [Run the TFTP server](#4-run-the-tftp-server)
5. [Flash the AP with the MODE button](#5-flash-the-ap-with-the-mode-button)
6. [First login and hardening](#6-first-login-and-hardening)
7. [Run the controller and add the AP](#7-run-the-controller-and-add-the-ap)
8. [Troubleshooting](#8-troubleshooting)

Tested on 2026-10-07 with two APs:

| AP | Before | After | Flash time |
|---|---|---|---|
| AIR-CAP2602I-E-K9 | AP3G2-K9W8-M 15.3(3)JC15 (lightweight) | AP3G2-K9W7-M 15.3(3)JF12, AIR-SAP2602I-E-K9 | ~21 min |
| AIR-CAP2702I-Z-K9 | lightweight | AP3G2-K9W7-M 15.3(3)JF12, AIR-SAP2702I-Z-K9 | ~13 min |

---

## 1. Identify the AP

Plug the AP into any switch port with DHCP and PoE. It gets an address, but every management port
(22, 23, 80, 443) refuses connections. A lightweight AP only broadcasts discovery for a WLC.

Read what it runs from CDP on your switch or router. On RouterOS that is `/ip neighbor print detail`.

| CDP field | Lightweight (needs this guide) | Autonomous (ready for the controller) |
|---|---|---|
| platform | `AIR-CAP2602I-…` (**C**AP) | `AIR-SAP2602I-…` (**S**AP) |
| version | `AP3G2-K9W8-M …JC…` (**K9W8**) | `AP3G2-K9W7-M …JF…` (**K9W7**) |

Note the AP's MAC address. You need it to find the AP's DHCP lease after the flash.

## 2. Get the firmware

The 1700, 2600, 2700 and 3600/3700 series all use the same **`ap3g2`** image family. We used:

| File | Size | MD5 | SHA-256 |
|---|---|---|---|
| `ap3g2-k9w7-tar.153-3.JF12.tar` | 13,864,960 B | `c13ca4c7bde027509e128eccf2597887` | `6a1bac2d2f506fbb4e317c5e5ea7e7c3a9418c4a7f82a7fbe68af6b7a0d24125` |

The image is Cisco-licensed software and is **not** in this repository. Get it from
[Cisco Software Download](https://software.cisco.com/download/) (requires a Cisco account), then check it
against the checksums above. `k9w7` is autonomous; `k9w8` is lightweight. Make sure you have `k9w7`.

ROMMON asks for one fixed filename, so copy the tar under that name:

```bash
cp ap3g2-k9w7-tar.153-3.JF12.tar recovery/tftp/ap3g2-k9w7-tar.default
md5sum recovery/tftp/ap3g2-k9w7-tar.default
```

`.gitignore` excludes `recovery/tftp/*`, so the image can't be committed by accident.

## 3. Build an isolated recovery network

In MODE-button recovery the AP **always** takes `10.0.0.1/24` and asks for the image from **`10.0.0.2`**.
You can't change these addresses. If `10.0.0.1` is already a router on your network, which is common,
the recovery must run on a separate layer-2 segment. Choose one:

- **Simplest:** connect the AP to the TFTP host with a direct cable or a dumb switch, using a PoE
  injector if needed. Give the host `10.0.0.2/24` and skip to step 4.
- **What we did:** add a dedicated VLAN on the switch. One switch port becomes the "recovery port", and the
  TFTP server runs in Docker on a normal LAN host, tagged into that VLAN. You don't need a spare NIC,
  router changes or sudo.

```
 AP (10.0.0.1) ──untagged── switch port ether13 [VLAN 99]
                                    │ tagged VLAN 99
                             trunk to the host
                                    │
                 host eno1 ── eno1.99 (made by dockerd) ── macvlan ── container cisco-tftp (10.0.0.2)
```

### Switch (MikroTik RouterOS 7, VLAN-filtering bridge)

`ether13` is the AP port and `sfp-sfpplus4` is the uplink toward the TFTP host. Replace these with your own
port names. Take a backup first.

```routeros
/system backup save name=pre-ap-recovery
/interface bridge vlan add bridge=bridge vlan-ids=99 untagged=ether13 tagged=sfp-sfpplus4,bridge \
    comment="AP recovery VLAN"
/interface bridge port set [find interface=ether13] pvid=99
```

If `ether13` is also listed as `untagged` in another VLAN, for example your IoT VLAN, remove it there.
Use `find vlan-ids=11 dynamic=no`, because a plain `vlan-ids=11` also matches the dynamic pvid entry and
fails with "invalid internal item number".

A switch between that uplink and the host must pass VLAN 99 through. A MikroTik CSS610 with its ports in
"optional" VLAN mode does this without changes.

### Host (Linux + Docker)

A macvlan network with a dotted parent makes dockerd create the VLAN sub-interface itself:

```bash
docker network create -d macvlan --subnet 10.0.0.0/24 -o parent=eno1.99 cisco-recovery
```

Replace `eno1` with the host NIC that carries the tagged VLAN. Note that a macvlan container can't reach
its own host, but that isn't needed here.

### Undo afterwards

To turn the port back into a normal access port, for example VLAN 11:

```routeros
/interface bridge port set [find interface=ether13] pvid=11
:local v [/interface bridge vlan find vlan-ids=11 dynamic=no]
/interface bridge vlan set $v untagged=([/interface bridge vlan get $v untagged], "ether13")
/interface bridge vlan remove [find vlan-ids=99]
```

To flash more APs later, run the three switch commands above again. The host side can stay as it is.

## 4. Run the TFTP server

```bash
cd recovery
docker compose up -d
docker logs -f cisco-tftp       # "listening on :69, serving /srv/tftp"
```

This runs `recovery/tftp_server.py`, a small read-only TFTP server, at `10.0.0.2`.
**Don't swap in `tftpd-hpa` or another standard server.** The AP3G2 ROMMON TFTP client is broken in two ways:

1. **It broadcasts its ACKs.** The read request goes to `255.255.255.255:69`, which is fine. After that,
   every ACK also goes to `255.255.255.255:<server port>` instead of to the server. `tftpd-hpa` binds its
   transfer socket to `10.0.0.2`, so the kernel never delivers those ACKs and the transfer stalls on
   block 1 every time. A longer timeout doesn't help. `tftp_server.py` binds its transfer sockets to
   `0.0.0.0` and so receives them.
2. **It loses data packets.** When it misses a block, it re-ACKs the previous block every 5 s for about
   30 s and then aborts. `tftp_server.py` resends as soon as it sees such a duplicate ACK, and uses a
   fixed per-packet deadline so the repeated ACKs can't keep resetting its timeout. Expect roughly 50 to
   100 resends per flash; they're normal.

ROMMON also goes silent for **about 30 seconds after block 1** while it erases flash. That's expected too.

You can test the server from another container on the same network before you involve an AP:

```bash
# The recovery network has no internet, so build the client image first
printf 'FROM alpine:3.20\nRUN apk add --no-cache tftp-hpa\n' | docker build -q -t tftp-client -
docker run --rm --network cisco-recovery --ip 10.0.0.3 tftp-client \
  sh -c 'cd /tmp && tftp -m binary 10.0.0.2 -c get ap3g2-k9w7-tar.default && md5sum ap3g2-k9w7-tar.default'
# expect: c13ca4c7bde027509e128eccf2597887
```

The standard client ACKs normally, so this test checks the file and the network path but not the broadcast-ACK
handling.

## 5. Flash the AP with the MODE button

1. Plug the AP into the recovery port, using PoE from the switch.
2. **Remove power.** Hold the **MODE** button and restore power. With PoE, disable and re-enable the port
   (`/interface ethernet poe set ether13 poe-out=off`, then `=auto-on`), or unplug the cable.
3. Keep holding until the status LED turns **red** (about 20 to 30 s), then release.
4. Watch the server:

   ```
   10.0.0.1:1024 RRQ ap3g2-k9w7-tar.default (13864960 bytes)
   10.0.0.1: duplicate ACK on block 463, resend 1        <- normal
   10.0.0.1: 7% (2000 blocks)
   ...
   10.0.0.1: DONE, sent 13864960 bytes in 27081 blocks
   ```

5. The download takes 10 to 25 minutes at about 9 KB/s. After `DONE`, the AP writes flash and reboots on
   its own, which takes a few more minutes. **Don't remove power during this time.**
6. Check the result: CDP now reports `AP3G2-K9W7-M` and a **SAP** platform, and the hostname is `ap`.

If no RRQ appears, the AP didn't enter recovery or can't reach the server. See
[Troubleshooting](#8-troubleshooting).

## 6. First login and hardening

Move the AP to the network it will live on, such as a normal access VLAN with DHCP. Find it by its MAC
address in the DHCP leases. The factory login is user `Cisco`, password `Cisco`, enable password `Cisco`,
over **telnet**, since SSH isn't set up yet.

**Step 1:** set a name, add an admin user and enable SSH, while telnet still works:

```
configure terminal
hostname AP-Garage
ip domain-name example.lan
username admin privilege 15 secret 0 <strong-password>
enable secret 0 <strong-password>
crypto key generate rsa general-keys modulus 2048
ip ssh version 2
ip ssh time-out 60
ip ssh authentication-retries 3
line vty 0 4
 login local
 transport input ssh telnet
 exit
end
write memory
```

The user **must be `privilege 15`**. The controller expects to land straight at the `#` prompt.

**Step 2:** log in over SSH as `admin`, then lock the AP down:

```
configure terminal
no username Cisco
no ip http server
no ip http secure-server
line vty 0 4
 transport input ssh
 exit
end
write memory
```

⚠️ `no username Cisco` asks `[confirm]`. If you paste or script these lines, the next line becomes the
answer to that prompt and is lost. Answer the prompt with Enter before you send anything else.

OpenSSH needs legacy algorithms for IOS 15.3. The controller's Go client doesn't.

```bash
ssh -o HostKeyAlgorithms=+ssh-rsa -o KexAlgorithms=+diffie-hellman-group14-sha1 admin@<ap-ip>
```

The radios stay **administratively down** until an SSID is configured. That's normal, and the controller
brings them up when it pushes the first SSID.

Reserve the AP's address on your DHCP server. The controller connects to the address you register.

## 7. Run the controller and add the AP

### Start it

`docker-compose.yml` works as-is for a quick try, using a dev encryption key and UI on port 3000. For real
use, keep secrets out of the repo with an override file:

```bash
openssl rand -hex 32 > ~/.config/cisco-ap/encryption.key && chmod 600 ~/.config/cisco-ap/encryption.key

cat > ~/.config/cisco-ap/compose.override.yml <<EOF
services:
  postgres:
    ports: !reset []              # don't publish the database
  api:
    environment:
      ENCRYPTION_KEY: "$(cat ~/.config/cisco-ap/encryption.key)"
  ui:
    ports: !override ["3090:3000"]  # if 3000 is taken
EOF
chmod 600 ~/.config/cisco-ap/compose.override.yml

docker compose -p cisco-aps -f docker-compose.yml -f ~/.config/cisco-ap/compose.override.yml up -d --build
```

- UI: http://localhost:3090, or :3000 without the override
- API: http://localhost:8080/healthz

The `ENCRYPTION_KEY` encrypts the AP passwords stored in Postgres. If you lose it, you'll have to
re-enter every AP password. The host running the API must be able to reach the APs on TCP/22.

### Add the AP

In the GUI, click **Add AP** and enter a name, the AP's IP, port 22, `admin` and its password.
Or use the API:

```bash
curl -X POST http://localhost:8080/api/v1/aps -H "Content-Type: application/json" \
  -d '{"name":"AP-Garage","hostname":"10.100.0.39","ssh_port":22,"username":"admin","password":"<ap-password>"}'
```

Within 30 s the reconciler logs in, and the AP shows **online** with the model and firmware it read
(for example `AIR-SAP2602I-E-K9`, `15.3(3)JF12`). From there you can manage SSIDs, radios and clients in
the GUI. See the [README](../README.md) for the API.

## 8. Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Ports 22/23/80/443 all refused | AP is lightweight (K9W8) | This guide, from step 1 |
| No RRQ in `docker logs cisco-tftp` | Not in recovery mode, or no L2 path | Hold MODE from power-on until the LED is red. Check that the port has pvid 99 and that VLAN 99 is tagged all the way to the host. |
| RRQ appears, then stuck at block 1 forever | A standard TFTP server that ignores broadcast ACKs | Use `recovery/tftp_server.py` |
| Gap of about 30 s after block 1 | ROMMON is erasing flash | Wait |
| "duplicate ACK … resend" lines | ROMMON lost a packet | Normal; the server recovers |
| "giving up at block N" | No ACK for 2 minutes | Power-cycle and repeat step 5; the transfer starts over |
| Login prompt eats a command | `[confirm]` after `no username Cisco` | Answer it with Enter first |
| SSH: "no matching host key type" | OpenSSH vs. old IOS | Add `-o HostKeyAlgorithms=+ssh-rsa -o KexAlgorithms=+diffie-hellman-group14-sha1` |
| `crypto key generate rsa` fails | No hostname or domain set | Set `hostname` and `ip domain-name` first |
| Controller shows the AP as error / timeout at prompt | User isn't privilege 15 | `username admin privilege 15 …` |
| GUI says "fetch failed" | UI container can't reach the API | `API_INTERNAL_URL` must point to the API service (`http://api:8080` in compose) |

To debug the transfer at packet level, capture inside the TFTP container's network namespace (no sudo needed):

```bash
docker run --rm --net container:cisco-tftp nicolaka/netshoot tcpdump -U -n -e -i eth0 udp
```

### Reverting to lightweight

Run this on the autonomous AP, with a `k9w8` image on a reachable TFTP server:

```
archive download-sw /force-reload /overwrite tftp://<server>/ap3g2-k9w8-tar.153-3.JF12.tar
```
