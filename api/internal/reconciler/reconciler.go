package reconciler

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/hacking-robots-and-beer/cisco/api/internal/ap"
	"github.com/hacking-robots-and-beer/cisco/api/internal/model"
	"github.com/hacking-robots-and-beer/cisco/api/internal/repository"
)

// Reconciler periodically syncs desired AP state from DB to the actual APs via SSH.
type Reconciler struct {
	repo     *repository.Repository
	interval time.Duration
	logger   *slog.Logger
}

// New creates a Reconciler. interval is how often to run the sync loop (e.g. 30s).
func New(repo *repository.Repository, interval time.Duration) *Reconciler {
	return &Reconciler{
		repo:     repo,
		interval: interval,
		logger:   slog.Default(),
	}
}

// Run starts the reconciliation loop. It blocks until ctx is cancelled.
func (r *Reconciler) Run(ctx context.Context) {
	r.logger.Info("reconciler started", "interval", r.interval)

	// Run immediately on startup, then on each tick.
	r.syncAll(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.syncAll(ctx)
		case <-ctx.Done():
			r.logger.Info("reconciler stopped")
			return
		}
	}
}

// SyncAP triggers an immediate sync for a single AP (used by the API on demand).
func (r *Reconciler) SyncAP(ctx context.Context, id uuid.UUID) error {
	apModel, err := r.repo.GetAP(ctx, id)
	if err != nil {
		return err
	}
	if apModel == nil {
		return nil
	}
	r.syncOne(ctx, apModel)
	return nil
}

func (r *Reconciler) syncAll(ctx context.Context) {
	aps, err := r.repo.ListAPs(ctx)
	if err != nil {
		r.logger.Error("list aps failed", "error", err)
		return
	}

	// Fetch full AP details (including decrypted passwords) for sync
	for _, summary := range aps {
		full, err := r.repo.GetAP(ctx, summary.ID)
		if err != nil {
			r.logger.Error("get ap failed", "ap", summary.Name, "error", err)
			continue
		}
		if full == nil {
			continue
		}
		r.syncOne(ctx, full)
	}
}

func (r *Reconciler) syncOne(ctx context.Context, apModel *model.AP) {
	logger := r.logger.With("ap", apModel.Name, "host", apModel.Hostname)
	logger.Info("syncing AP")

	// Mark as syncing
	if err := r.repo.UpdateAPStatus(ctx, apModel.ID, model.StatusSyncing, nil); err != nil {
		logger.Error("update status to syncing failed", "error", err)
	}

	client := ap.NewClient(apModel.Hostname, apModel.SSHPort, apModel.Username, apModel.Password)

	if err := client.Connect(); err != nil {
		logger.Error("ssh connect failed", "error", err)
		errStr := err.Error()
		_ = r.repo.UpdateAPStatus(ctx, apModel.ID, model.StatusOffline, &errStr)
		return
	}
	defer client.Close()

	// Collect AP info (model, firmware)
	versionOut, err := client.GetVersionInfo()
	if err != nil {
		logger.Warn("get version failed", "error", err)
	} else {
		apModelStr, firmware := ap.ParseVersionOutput(versionOut)
		if apModelStr != "" || firmware != "" {
			if err := r.repo.UpdateAPInfo(ctx, apModel.ID, apModelStr, firmware); err != nil {
				logger.Warn("update ap info failed", "error", err)
			}
		}
	}

	// Push desired SSID configuration
	ssids, err := r.repo.ListSSIDs(ctx, apModel.ID)
	if err != nil {
		logger.Error("list ssids failed", "error", err)
	} else if len(ssids) > 0 {
		if err := r.pushSSIDs(ctx, client, ssids, logger); err != nil {
			logger.Error("push ssids failed", "error", err)
		}
	}

	// Push desired radio configuration
	radioConfigs, err := r.repo.ListRadioConfigs(ctx, apModel.ID)
	if err != nil {
		logger.Error("list radio configs failed", "error", err)
	} else if len(radioConfigs) > 0 {
		if err := r.pushRadioConfigs(ctx, client, radioConfigs, logger); err != nil {
			logger.Error("push radio configs failed", "error", err)
		}
	}

	// Collect associated clients
	r.collectClients(ctx, client, apModel.ID, logger)

	// Mark as online
	if err := r.repo.UpdateAPStatus(ctx, apModel.ID, model.StatusOnline, nil); err != nil {
		logger.Error("update status to online failed", "error", err)
	}

	logger.Info("AP sync complete")
}

func (r *Reconciler) pushSSIDs(_ context.Context, client *ap.Client, ssids []*model.SSID, logger *slog.Logger) error {
	var cmds []string
	for _, s := range ssids {
		if s.Enabled {
			cmds = append(cmds, ap.SSIDConfigCmds(*s)...)
		} else {
			cmds = append(cmds, ap.DeleteSSIDCmds(s.Name)...)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	logger.Info("applying SSID config", "ssid_count", len(ssids))
	return client.Configure(cmds)
}

func (r *Reconciler) pushRadioConfigs(_ context.Context, client *ap.Client, configs []*model.RadioConfig, logger *slog.Logger) error {
	var cmds []string
	for _, rc := range configs {
		cmds = append(cmds, ap.RadioConfigCmds(*rc)...)
	}
	if len(cmds) == 0 {
		return nil
	}
	logger.Info("applying radio config", "radio_count", len(configs))
	return client.Configure(cmds)
}

func (r *Reconciler) collectClients(ctx context.Context, client *ap.Client, apID uuid.UUID, logger *slog.Logger) {
	lines, err := client.GetAssociations()
	if err != nil {
		logger.Warn("get associations failed", "error", err)
		return
	}

	output := joinLines(lines)
	parsed := ap.ParseAssociations(output)

	clients := make([]*model.Client, len(parsed))
	for i := range parsed {
		c := parsed[i]
		c.APID = apID
		clients[i] = &c
	}

	if err := r.repo.ReplaceClients(ctx, apID, clients); err != nil {
		logger.Warn("replace clients failed", "error", err)
	} else {
		logger.Info("clients updated", "count", len(clients))
	}
}

func joinLines(lines []string) string {
	result := ""
	for _, l := range lines {
		result += l + "\n"
	}
	return result
}
