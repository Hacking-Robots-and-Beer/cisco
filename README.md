# Cisco AP Controller

Centralized controller for **Cisco AIR-CAP2602** access points running in autonomous (standalone) IOS mode. Replaces a Cisco WLC by SSH-ing into each AP and managing them via the IOS CLI.

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   Next.js UI    │────▶│  Go API Server   │────▶│   PostgreSQL    │
│  (port 3000)    │     │  (port 8080)     │     │                 │
└─────────────────┘     └────────┬─────────┘     └─────────────────┘
                                 │ reconciler (every 30s)
                    ┌────────────▼─────────────┐
                    │    SSH (port 22)          │
                    │  AP-1  AP-2  AP-N ...     │
                    │  (Cisco autonomous IOS)   │
                    └──────────────────────────┘
```

The **reconciler** runs in the background inside the API pod, connecting to each registered AP every 30 seconds, pushing desired SSID and radio configuration, and collecting connected clients.

## Prerequisites

APs must be converted from lightweight (CAPWAP) to **autonomous IOS** mode before registration. See [`docs/ap-conversion.md`](docs/ap-conversion.md) for the full procedure using either the MODE button or an existing WLC.

**Target firmware:** `ap3g2-k9w7-tar.153-3.JF11.tar`

## Project Structure

```
cisco/
├── api/                     # Go API service (Gin + pgx + golang.org/x/crypto/ssh)
│   ├── cmd/main.go
│   ├── internal/
│   │   ├── ap/              # Cisco IOS SSH client + command generators + output parsers
│   │   ├── reconciler/      # Background 30s sync loop
│   │   ├── handler/         # Gin HTTP handlers
│   │   ├── service/         # Business logic
│   │   ├── repository/      # PostgreSQL (pgx/v5)
│   │   ├── crypto/          # AES-256-GCM password encryption
│   │   ├── model/           # Domain types
│   │   └── migrations/      # Embedded SQL migrations
│   └── Dockerfile
├── ui/                      # Next.js 16 frontend (App Router + Tailwind CSS 4)
│   ├── src/app/             # Dashboard, AP detail, forms
│   ├── src/lib/api.ts       # Typed fetch client
│   └── Dockerfile
├── chart/                   # Helm chart
├── docs/
│   └── ap-conversion.md     # AP firmware flash guide
└── docker-compose.yml       # Local dev stack
```

## Local Development

### Requirements

- Docker + Docker Compose
- Go 1.23+ (for API development)
- Node.js 20+ (for UI development)

### Start

```bash
cp .env.example .env
# Edit .env — set a real ENCRYPTION_KEY (64 hex chars, 32 bytes)

docker compose up
```

- UI: http://localhost:3000
- API: http://localhost:8080
- Health: http://localhost:8080/healthz

### First run

```bash
# Generate a secure encryption key
openssl rand -hex 32
# Paste into .env as ENCRYPTION_KEY
```

## API Reference

```
GET    /api/v1/aps                    List all APs
POST   /api/v1/aps                    Register AP
GET    /api/v1/aps/:id               AP details + current status
PUT    /api/v1/aps/:id               Update AP settings
DELETE /api/v1/aps/:id               Remove AP
POST   /api/v1/aps/:id/sync          Trigger immediate sync

GET    /api/v1/aps/:id/ssids         List desired SSIDs
POST   /api/v1/aps/:id/ssids         Add SSID
PUT    /api/v1/aps/:id/ssids/:sid    Update SSID
DELETE /api/v1/aps/:id/ssids/:sid    Remove SSID

GET    /api/v1/aps/:id/radios        Get radio configs
PUT    /api/v1/aps/:id/radios/:band  Update radio (2.4ghz or 5ghz)

GET    /api/v1/aps/:id/clients       Connected clients (from last poll)

GET    /healthz                      Health check
```

### Example: Register an AP

```bash
curl -X POST http://localhost:8080/api/v1/aps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "AP-Office-1",
    "hostname": "192.168.1.10",
    "ssh_port": 22,
    "username": "admin",
    "password": "your-ap-password"
  }'
```

### Example: Add a WPA2 SSID

```bash
curl -X POST http://localhost:8080/api/v1/aps/<id>/ssids \
  -H "Content-Type: application/json" \
  -d '{
    "name": "MyNetwork",
    "radio": "both",
    "security": "wpa2-psk",
    "password": "MySecurePassphrase",
    "vlan": 1
  }'
```

The reconciler will push this to the AP within 30 seconds. Verify with:

```
AP-Office-1# show running-config | section dot11
```

## Kubernetes Deployment

```bash
# Install
helm install cisco-aps chart/ \
  --set secret.POSTGRES_PASSWORD=<strong-password> \
  --set secret.ENCRYPTION_KEY=<hex-32-bytes> \
  --set secret.DATABASE_URL="postgres://cisco:<password>@cisco-aps-postgres:5432/cisco?sslmode=disable" \
  --set ingress.hosts[0]=cisco-aps.yourdomain.com \
  --set ui.env.NEXT_PUBLIC_API_URL=https://cisco-aps.yourdomain.com

# If APs are only reachable from the node's physical network:
helm install cisco-aps chart/ --set api.hostNetwork=true ...
```

### Network Note

The Go API pod must have network access to the APs' IP addresses. Options:

1. **Default** — APs reachable from the pod CIDR (e.g. routed LAN)
2. **`hostNetwork: true`** — API pod uses the node's network stack directly; set `api.hostNetwork: true` in `values.yaml`

## Configuration

| Environment Variable | Description | Required |
|---|---|---|
| `DATABASE_URL` | PostgreSQL connection string | Yes |
| `ENCRYPTION_KEY` | 64-char hex (32 bytes) AES key for stored AP passwords | Yes |
| `GIN_MODE` | `debug` or `release` | No (default: debug) |
| `NEXT_PUBLIC_API_URL` | API base URL visible to the browser | Yes (UI) |

## CI/CD

On push to `main`:

1. **Docker Publish** (`docker-image.yml`) — builds and pushes `api` and `ui` images to GHCR in parallel
2. **Deploy** (`deploy.yaml`) — runs on `arc-runner-set-hrb`, templates the Helm chart with the new SHA tags, commits the manifest to [Hacking-Robots-and-Beer/deploy](https://github.com/Hacking-Robots-and-Beer/deploy)

Images:
- `ghcr.io/hacking-robots-and-beer/cisco/api:<sha>`
- `ghcr.io/hacking-robots-and-beer/cisco/ui:<sha>`
