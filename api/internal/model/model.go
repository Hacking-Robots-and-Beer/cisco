package model

import (
	"time"

	"github.com/google/uuid"
)

// AP represents a Cisco autonomous access point.
type AP struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	Hostname        string     `json:"hostname"`
	SSHPort         int        `json:"ssh_port"`
	Username        string     `json:"username"`
	Password        string     `json:"password,omitempty"` // plaintext in memory, encrypted in DB
	Model           *string    `json:"model,omitempty"`
	FirmwareVersion *string    `json:"firmware_version,omitempty"`
	Status          string     `json:"status"` // unknown/online/offline/syncing/error
	LastSeenAt      *time.Time `json:"last_seen_at,omitempty"`
	LastSyncAt      *time.Time `json:"last_sync_at,omitempty"`
	SyncError       *string    `json:"sync_error,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// SSID represents a desired SSID configuration on an AP.
type SSID struct {
	ID        uuid.UUID `json:"id"`
	APID      uuid.UUID `json:"ap_id"`
	Name      string    `json:"name"`
	VLAN      int       `json:"vlan"`
	Radio     string    `json:"radio"`    // 2.4ghz / 5ghz / both
	Security  string    `json:"security"` // open / wpa2-psk
	Password  *string   `json:"password,omitempty"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RadioConfig represents the desired radio configuration for one band on an AP.
type RadioConfig struct {
	ID         uuid.UUID `json:"id"`
	APID       uuid.UUID `json:"ap_id"`
	Band       string    `json:"band"`        // 2.4ghz / 5ghz
	Channel    int       `json:"channel"`     // 0 = auto
	TxPowerDBm int       `json:"tx_power_dbm"` // 0 = auto
	Enabled    bool      `json:"enabled"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Client represents a wireless client currently associated with an AP.
type Client struct {
	ID         int64     `json:"id"`
	APID       uuid.UUID `json:"ap_id"`
	MACAddress string    `json:"mac_address"`
	IPAddress  *string   `json:"ip_address,omitempty"`
	SSID       *string   `json:"ssid,omitempty"`
	Radio      *string   `json:"radio,omitempty"`
	SignalDBm  *int      `json:"signal_dbm,omitempty"`
	SeenAt     time.Time `json:"seen_at"`
}

// APStatus values
const (
	StatusUnknown  = "unknown"
	StatusOnline   = "online"
	StatusOffline  = "offline"
	StatusSyncing  = "syncing"
	StatusError    = "error"
)
