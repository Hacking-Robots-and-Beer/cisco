package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hacking-robots-and-beer/cisco/api/internal/crypto"
	"github.com/hacking-robots-and-beer/cisco/api/internal/model"
)

// Repository handles all database operations.
type Repository struct {
	pool          *pgxpool.Pool
	encryptionKey []byte
}

// New creates a new Repository.
func New(pool *pgxpool.Pool, encryptionKey []byte) *Repository {
	return &Repository{pool: pool, encryptionKey: encryptionKey}
}

// RunMigrations applies SQL migration files embedded in the migrations directory.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, sql string) error {
	_, err := pool.Exec(ctx, sql)
	return err
}

// --- AP ---

func (r *Repository) ListAPs(ctx context.Context) ([]*model.AP, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, hostname, ssh_port, username, password,
		       model, firmware_version, status, last_seen_at, last_sync_at,
		       sync_error, created_at, updated_at
		FROM aps ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list aps: %w", err)
	}
	defer rows.Close()

	var aps []*model.AP
	for rows.Next() {
		ap, err := r.scanAP(rows)
		if err != nil {
			return nil, err
		}
		ap.Password = "" // never return password over API
		aps = append(aps, ap)
	}
	return aps, rows.Err()
}

func (r *Repository) GetAP(ctx context.Context, id uuid.UUID) (*model.AP, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, hostname, ssh_port, username, password,
		       model, firmware_version, status, last_seen_at, last_sync_at,
		       sync_error, created_at, updated_at
		FROM aps WHERE id = $1`, id)

	ap, err := r.scanAP(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get ap: %w", err)
	}

	// Decrypt password for internal use
	if ap.Password != "" {
		plain, err := crypto.Decrypt(ap.Password, r.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt password: %w", err)
		}
		ap.Password = plain
	}
	return ap, nil
}

func (r *Repository) CreateAP(ctx context.Context, ap *model.AP) (*model.AP, error) {
	encPwd, err := crypto.Encrypt(ap.Password, r.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt password: %w", err)
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO aps (name, hostname, ssh_port, username, password)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, hostname, ssh_port, username, password,
		          model, firmware_version, status, last_seen_at, last_sync_at,
		          sync_error, created_at, updated_at`,
		ap.Name, ap.Hostname, ap.SSHPort, ap.Username, encPwd)

	created, err := r.scanAP(row)
	if err != nil {
		return nil, fmt.Errorf("create ap: %w", err)
	}
	created.Password = ""
	return created, nil
}

func (r *Repository) UpdateAP(ctx context.Context, ap *model.AP) (*model.AP, error) {
	encPwd, err := crypto.Encrypt(ap.Password, r.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt password: %w", err)
	}

	row := r.pool.QueryRow(ctx, `
		UPDATE aps SET name=$2, hostname=$3, ssh_port=$4, username=$5, password=$6,
		              updated_at=NOW()
		WHERE id=$1
		RETURNING id, name, hostname, ssh_port, username, password,
		          model, firmware_version, status, last_seen_at, last_sync_at,
		          sync_error, created_at, updated_at`,
		ap.ID, ap.Name, ap.Hostname, ap.SSHPort, ap.Username, encPwd)

	updated, err := r.scanAP(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("update ap: %w", err)
	}
	updated.Password = ""
	return updated, nil
}

func (r *Repository) DeleteAP(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM aps WHERE id=$1`, id)
	return err
}

func (r *Repository) UpdateAPStatus(ctx context.Context, id uuid.UUID, status string, syncErr *string) error {
	now := time.Now()
	if status == model.StatusOnline {
		_, err := r.pool.Exec(ctx, `
			UPDATE aps SET status=$2, last_seen_at=$3, last_sync_at=$3,
			              sync_error=NULL, updated_at=NOW()
			WHERE id=$1`, id, status, now)
		return err
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE aps SET status=$2, last_sync_at=$3, sync_error=$4, updated_at=NOW()
		WHERE id=$1`, id, status, now, syncErr)
	return err
}

func (r *Repository) UpdateAPInfo(ctx context.Context, id uuid.UUID, apModel, firmware string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE aps SET model=$2, firmware_version=$3, updated_at=NOW()
		WHERE id=$1`, id, apModel, firmware)
	return err
}

// --- SSID ---

func (r *Repository) ListSSIDs(ctx context.Context, apID uuid.UUID) ([]*model.SSID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, ap_id, name, vlan, radio, security, password, enabled, created_at, updated_at
		FROM ssids WHERE ap_id=$1 ORDER BY name`, apID)
	if err != nil {
		return nil, fmt.Errorf("list ssids: %w", err)
	}
	defer rows.Close()

	var ssids []*model.SSID
	for rows.Next() {
		s := &model.SSID{}
		if err := rows.Scan(&s.ID, &s.APID, &s.Name, &s.VLAN, &s.Radio, &s.Security,
			&s.Password, &s.Enabled, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		ssids = append(ssids, s)
	}
	return ssids, rows.Err()
}

func (r *Repository) GetSSID(ctx context.Context, id uuid.UUID) (*model.SSID, error) {
	s := &model.SSID{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, ap_id, name, vlan, radio, security, password, enabled, created_at, updated_at
		FROM ssids WHERE id=$1`, id).
		Scan(&s.ID, &s.APID, &s.Name, &s.VLAN, &s.Radio, &s.Security,
			&s.Password, &s.Enabled, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

func (r *Repository) CreateSSID(ctx context.Context, s *model.SSID) (*model.SSID, error) {
	created := &model.SSID{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO ssids (ap_id, name, vlan, radio, security, password, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, ap_id, name, vlan, radio, security, password, enabled, created_at, updated_at`,
		s.APID, s.Name, s.VLAN, s.Radio, s.Security, s.Password, s.Enabled).
		Scan(&created.ID, &created.APID, &created.Name, &created.VLAN, &created.Radio,
			&created.Security, &created.Password, &created.Enabled, &created.CreatedAt, &created.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create ssid: %w", err)
	}
	return created, nil
}

func (r *Repository) UpdateSSID(ctx context.Context, s *model.SSID) (*model.SSID, error) {
	updated := &model.SSID{}
	err := r.pool.QueryRow(ctx, `
		UPDATE ssids SET name=$2, vlan=$3, radio=$4, security=$5, password=$6, enabled=$7, updated_at=NOW()
		WHERE id=$1
		RETURNING id, ap_id, name, vlan, radio, security, password, enabled, created_at, updated_at`,
		s.ID, s.Name, s.VLAN, s.Radio, s.Security, s.Password, s.Enabled).
		Scan(&updated.ID, &updated.APID, &updated.Name, &updated.VLAN, &updated.Radio,
			&updated.Security, &updated.Password, &updated.Enabled, &updated.CreatedAt, &updated.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("update ssid: %w", err)
	}
	return updated, nil
}

func (r *Repository) DeleteSSID(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM ssids WHERE id=$1`, id)
	return err
}

// --- RadioConfig ---

func (r *Repository) ListRadioConfigs(ctx context.Context, apID uuid.UUID) ([]*model.RadioConfig, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, ap_id, band, channel, tx_power_dbm, enabled, updated_at
		FROM radio_configs WHERE ap_id=$1 ORDER BY band`, apID)
	if err != nil {
		return nil, fmt.Errorf("list radio configs: %w", err)
	}
	defer rows.Close()

	var configs []*model.RadioConfig
	for rows.Next() {
		rc := &model.RadioConfig{}
		if err := rows.Scan(&rc.ID, &rc.APID, &rc.Band, &rc.Channel, &rc.TxPowerDBm,
			&rc.Enabled, &rc.UpdatedAt); err != nil {
			return nil, err
		}
		configs = append(configs, rc)
	}
	return configs, rows.Err()
}

func (r *Repository) UpsertRadioConfig(ctx context.Context, rc *model.RadioConfig) (*model.RadioConfig, error) {
	updated := &model.RadioConfig{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO radio_configs (ap_id, band, channel, tx_power_dbm, enabled, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (ap_id, band) DO UPDATE
		SET channel=$3, tx_power_dbm=$4, enabled=$5, updated_at=NOW()
		RETURNING id, ap_id, band, channel, tx_power_dbm, enabled, updated_at`,
		rc.APID, rc.Band, rc.Channel, rc.TxPowerDBm, rc.Enabled).
		Scan(&updated.ID, &updated.APID, &updated.Band, &updated.Channel, &updated.TxPowerDBm,
			&updated.Enabled, &updated.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert radio config: %w", err)
	}
	return updated, nil
}

// --- Clients ---

func (r *Repository) ReplaceClients(ctx context.Context, apID uuid.UUID, clients []*model.Client) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Delete stale clients (older than 5 minutes from last poll)
	if _, err := tx.Exec(ctx, `DELETE FROM clients WHERE ap_id=$1`, apID); err != nil {
		return fmt.Errorf("delete old clients: %w", err)
	}

	for _, c := range clients {
		if _, err := tx.Exec(ctx, `
			INSERT INTO clients (ap_id, mac_address, ip_address, ssid, radio, signal_dbm, seen_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())`,
			apID, c.MACAddress, c.IPAddress, c.SSID, c.Radio, c.SignalDBm); err != nil {
			return fmt.Errorf("insert client: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) ListClients(ctx context.Context, apID uuid.UUID) ([]*model.Client, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, ap_id, mac_address, ip_address, ssid, radio, signal_dbm, seen_at
		FROM clients WHERE ap_id=$1 ORDER BY seen_at DESC`, apID)
	if err != nil {
		return nil, fmt.Errorf("list clients: %w", err)
	}
	defer rows.Close()

	var clients []*model.Client
	for rows.Next() {
		c := &model.Client{}
		if err := rows.Scan(&c.ID, &c.APID, &c.MACAddress, &c.IPAddress, &c.SSID,
			&c.Radio, &c.SignalDBm, &c.SeenAt); err != nil {
			return nil, err
		}
		clients = append(clients, c)
	}
	return clients, rows.Err()
}

// --- helpers ---

type scanner interface {
	Scan(dest ...any) error
}

func (r *Repository) scanAP(s scanner) (*model.AP, error) {
	ap := &model.AP{}
	err := s.Scan(
		&ap.ID, &ap.Name, &ap.Hostname, &ap.SSHPort, &ap.Username, &ap.Password,
		&ap.Model, &ap.FirmwareVersion, &ap.Status, &ap.LastSeenAt, &ap.LastSyncAt,
		&ap.SyncError, &ap.CreatedAt, &ap.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return ap, nil
}
