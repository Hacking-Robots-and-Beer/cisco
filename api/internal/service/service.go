package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/hacking-robots-and-beer/cisco/api/internal/model"
	"github.com/hacking-robots-and-beer/cisco/api/internal/repository"
)

// Service provides business logic for AP management.
type Service struct {
	repo *repository.Repository
}

// New creates a new Service.
func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

// --- AP ---

func (s *Service) ListAPs(ctx context.Context) ([]*model.AP, error) {
	return s.repo.ListAPs(ctx)
}

func (s *Service) GetAP(ctx context.Context, id uuid.UUID) (*model.AP, error) {
	ap, err := s.repo.GetAP(ctx, id)
	if err != nil {
		return nil, err
	}
	if ap == nil {
		return nil, nil
	}
	ap.Password = "" // don't leak password via API
	return ap, nil
}

func (s *Service) CreateAP(ctx context.Context, req CreateAPRequest) (*model.AP, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.Hostname == "" {
		return nil, fmt.Errorf("hostname is required")
	}
	if req.Username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if req.Password == "" {
		return nil, fmt.Errorf("password is required")
	}
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}

	ap := &model.AP{
		Name:     req.Name,
		Hostname: req.Hostname,
		SSHPort:  req.SSHPort,
		Username: req.Username,
		Password: req.Password,
	}

	return s.repo.CreateAP(ctx, ap)
}

func (s *Service) UpdateAP(ctx context.Context, id uuid.UUID, req UpdateAPRequest) (*model.AP, error) {
	existing, err := s.repo.GetAP(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Hostname != "" {
		existing.Hostname = req.Hostname
	}
	if req.SSHPort != 0 {
		existing.SSHPort = req.SSHPort
	}
	if req.Username != "" {
		existing.Username = req.Username
	}
	if req.Password != "" {
		existing.Password = req.Password
	}

	return s.repo.UpdateAP(ctx, existing)
}

func (s *Service) DeleteAP(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteAP(ctx, id)
}

// --- SSID ---

func (s *Service) ListSSIDs(ctx context.Context, apID uuid.UUID) ([]*model.SSID, error) {
	return s.repo.ListSSIDs(ctx, apID)
}

func (s *Service) CreateSSID(ctx context.Context, apID uuid.UUID, req CreateSSIDRequest) (*model.SSID, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.Radio == "" {
		req.Radio = "both"
	}
	if req.Security == "" {
		req.Security = "open"
	}
	if req.VLAN == 0 {
		req.VLAN = 1
	}
	if req.Security == "wpa2-psk" && (req.Password == nil || *req.Password == "") {
		return nil, fmt.Errorf("password is required for wpa2-psk security")
	}

	ssid := &model.SSID{
		APID:     apID,
		Name:     req.Name,
		VLAN:     req.VLAN,
		Radio:    req.Radio,
		Security: req.Security,
		Password: req.Password,
		Enabled:  true,
	}
	if req.Enabled != nil {
		ssid.Enabled = *req.Enabled
	}

	return s.repo.CreateSSID(ctx, ssid)
}

func (s *Service) UpdateSSID(ctx context.Context, id uuid.UUID, req UpdateSSIDRequest) (*model.SSID, error) {
	existing, err := s.repo.GetSSID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.VLAN != 0 {
		existing.VLAN = req.VLAN
	}
	if req.Radio != "" {
		existing.Radio = req.Radio
	}
	if req.Security != "" {
		existing.Security = req.Security
	}
	if req.Password != nil {
		existing.Password = req.Password
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	return s.repo.UpdateSSID(ctx, existing)
}

func (s *Service) DeleteSSID(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteSSID(ctx, id)
}

// --- RadioConfig ---

func (s *Service) ListRadioConfigs(ctx context.Context, apID uuid.UUID) ([]*model.RadioConfig, error) {
	return s.repo.ListRadioConfigs(ctx, apID)
}

func (s *Service) UpsertRadioConfig(ctx context.Context, apID uuid.UUID, band string, req UpsertRadioRequest) (*model.RadioConfig, error) {
	if band != "2.4ghz" && band != "5ghz" {
		return nil, fmt.Errorf("band must be '2.4ghz' or '5ghz'")
	}

	rc := &model.RadioConfig{
		APID:       apID,
		Band:       band,
		Channel:    req.Channel,
		TxPowerDBm: req.TxPowerDBm,
		Enabled:    true,
	}
	if req.Enabled != nil {
		rc.Enabled = *req.Enabled
	}

	return s.repo.UpsertRadioConfig(ctx, rc)
}

// --- Clients ---

func (s *Service) ListClients(ctx context.Context, apID uuid.UUID) ([]*model.Client, error) {
	return s.repo.ListClients(ctx, apID)
}

// --- Request types ---

type CreateAPRequest struct {
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	SSHPort  int    `json:"ssh_port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type UpdateAPRequest struct {
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	SSHPort  int    `json:"ssh_port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type CreateSSIDRequest struct {
	Name     string  `json:"name"`
	VLAN     int     `json:"vlan"`
	Radio    string  `json:"radio"`
	Security string  `json:"security"`
	Password *string `json:"password"`
	Enabled  *bool   `json:"enabled"`
}

type UpdateSSIDRequest struct {
	Name     string  `json:"name"`
	VLAN     int     `json:"vlan"`
	Radio    string  `json:"radio"`
	Security string  `json:"security"`
	Password *string `json:"password"`
	Enabled  *bool   `json:"enabled"`
}

type UpsertRadioRequest struct {
	Channel    int   `json:"channel"`
	TxPowerDBm int   `json:"tx_power_dbm"`
	Enabled    *bool `json:"enabled"`
}
