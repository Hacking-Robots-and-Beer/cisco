"""Minimal read-only TFTP server for Cisco AP ROMMON (MODE-button) recovery.

Why not tftpd-hpa: the AP3G2 ROMMON client sends its ACKs to 255.255.255.255
instead of to the server. tftpd-hpa binds each transfer socket to the server's
own address, so the kernel never delivers those broadcast ACKs and the transfer
stalls on block 1. Here every transfer socket binds to 0.0.0.0, which receives
both unicast and broadcast datagrams for its port.
"""
import os
import socket
import struct
import sys
import threading
import time

ROOT = os.environ.get("TFTP_ROOT", "/srv/tftp")
BLOCK = 512
TIMEOUT = 5.0
RETRIES = 24  # x TIMEOUT = 2 min: ROMMON pauses ~30s to erase flash mid-transfer
RRQ, DATA, ACK, ERROR = 1, 3, 4, 5


def log(msg):
    print(msg, flush=True)


def send_error(sock, addr, code, text):
    sock.sendto(struct.pack("!HH", ERROR, code) + text.encode() + b"\0", addr)


def send_until_acked(sock, client, packet, block):
    """Send one DATA packet until its ACK arrives. Returns False if the transfer is dead.

    A repeated ACK for the previous block means the client lost this DATA packet:
    ROMMON re-ACKs every 5s for ~30s and then gives up, so resend immediately.
    The timeout is a fixed deadline so those duplicate ACKs cannot keep resetting it.
    """
    want, prev = block & 0xFFFF, (block - 1) & 0xFFFF
    for attempt in range(RETRIES):
        sock.sendto(packet, client)
        deadline = time.monotonic() + TIMEOUT
        resend = False
        while not resend:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                break
            sock.settimeout(remaining)
            try:
                data, src = sock.recvfrom(1024)
            except socket.timeout:
                break
            # The ACK may arrive as a broadcast; match on the client's IP+port.
            if src != client or len(data) < 4:
                continue
            op, num = struct.unpack("!HH", data[:4])
            if op == ERROR:
                log(f"{client[0]}: client aborted: {data[4:].rstrip(b'\0')!r}")
                return False
            if op == ACK and num == want:
                return True
            if op == ACK and num == prev:
                resend = True
        log(f"{client[0]}: {'duplicate ACK' if resend else 'timeout'} on block {block}, resend {attempt + 1}")
    log(f"{client[0]}: giving up at block {block}")
    return False


def serve(filename, client):
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_BROADCAST, 1)
    sock.bind(("0.0.0.0", 0))
    path = os.path.realpath(os.path.join(ROOT, filename.lstrip("/")))
    if not path.startswith(os.path.realpath(ROOT) + os.sep) or not os.path.isfile(path):
        log(f"{client[0]}: file not found: {filename}")
        send_error(sock, client, 1, "File not found")
        return
    size = os.path.getsize(path)
    log(f"{client[0]}:{client[1]} RRQ {filename} ({size} bytes)")
    with open(path, "rb") as f:
        block = 1
        while True:
            chunk = f.read(BLOCK)
            packet = struct.pack("!HH", DATA, block & 0xFFFF) + chunk
            if not send_until_acked(sock, client, packet, block):
                return
            if block % 2000 == 0:
                log(f"{client[0]}: {block * BLOCK * 100 // max(size, 1)}% ({block} blocks)")
            if len(chunk) < BLOCK:
                log(f"{client[0]}: DONE, sent {size} bytes in {block} blocks")
                return
            block += 1


def main():
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_BROADCAST, 1)
    sock.bind(("0.0.0.0", 69))
    log(f"listening on :69, serving {ROOT}")
    active = set()
    while True:
        data, client = sock.recvfrom(1024)
        if len(data) < 4 or struct.unpack("!H", data[:2])[0] != RRQ:
            continue
        if client in active:  # duplicate RRQ for a transfer already running
            continue
        filename = data[2:].split(b"\0")[0].decode(errors="replace")

        def run(fn=filename, c=client):
            active.add(c)
            try:
                serve(fn, c)
            finally:
                active.discard(c)

        threading.Thread(target=run, daemon=True).start()


if __name__ == "__main__":
    sys.exit(main())
