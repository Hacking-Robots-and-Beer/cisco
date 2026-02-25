CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE aps (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name             TEXT NOT NULL,
    hostname         TEXT NOT NULL,
    ssh_port         INTEGER NOT NULL DEFAULT 22,
    username         TEXT NOT NULL,
    password         TEXT NOT NULL,       -- AES-256-GCM encrypted, base64 encoded
    model            TEXT,
    firmware_version TEXT,
    status           TEXT NOT NULL DEFAULT 'unknown',  -- unknown/online/offline/syncing/error
    last_seen_at     TIMESTAMPTZ,
    last_sync_at     TIMESTAMPTZ,
    sync_error       TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ssids (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ap_id      UUID NOT NULL REFERENCES aps(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    vlan       INTEGER NOT NULL DEFAULT 1,
    radio      TEXT NOT NULL CHECK (radio IN ('2.4ghz', '5ghz', 'both')),
    security   TEXT NOT NULL CHECK (security IN ('open', 'wpa2-psk')),
    password   TEXT,
    enabled    BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE radio_configs (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ap_id        UUID NOT NULL REFERENCES aps(id) ON DELETE CASCADE,
    band         TEXT NOT NULL CHECK (band IN ('2.4ghz', '5ghz')),
    channel      INTEGER NOT NULL DEFAULT 0,       -- 0 = auto
    tx_power_dbm INTEGER NOT NULL DEFAULT 0,       -- 0 = auto
    enabled      BOOLEAN NOT NULL DEFAULT true,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (ap_id, band)
);

CREATE TABLE clients (
    id          BIGSERIAL PRIMARY KEY,
    ap_id       UUID NOT NULL REFERENCES aps(id) ON DELETE CASCADE,
    mac_address TEXT NOT NULL,
    ip_address  TEXT,
    ssid        TEXT,
    radio       TEXT,
    signal_dbm  INTEGER,
    seen_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_clients_ap_id ON clients (ap_id);
CREATE INDEX idx_clients_seen_at ON clients (seen_at);
CREATE INDEX idx_ssids_ap_id ON ssids (ap_id);
CREATE INDEX idx_radio_configs_ap_id ON radio_configs (ap_id);
