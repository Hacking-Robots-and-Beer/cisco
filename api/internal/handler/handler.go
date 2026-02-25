package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hacking-robots-and-beer/cisco/api/internal/model"
	"github.com/hacking-robots-and-beer/cisco/api/internal/reconciler"
	"github.com/hacking-robots-and-beer/cisco/api/internal/service"
)

// Handler holds all HTTP handlers for the API.
type Handler struct {
	svc *service.Service
	rec *reconciler.Reconciler
}

// New creates a new Handler.
func New(svc *service.Service, rec *reconciler.Reconciler) *Handler {
	return &Handler{svc: svc, rec: rec}
}

// RegisterRoutes attaches all routes to the given Gin engine.
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/healthz", h.Healthz)

	v1 := r.Group("/api/v1")
	{
		// APs
		v1.GET("/aps", h.ListAPs)
		v1.POST("/aps", h.CreateAP)
		v1.GET("/aps/:id", h.GetAP)
		v1.PUT("/aps/:id", h.UpdateAP)
		v1.DELETE("/aps/:id", h.DeleteAP)
		v1.POST("/aps/:id/sync", h.SyncAP)

		// SSIDs
		v1.GET("/aps/:id/ssids", h.ListSSIDs)
		v1.POST("/aps/:id/ssids", h.CreateSSID)
		v1.PUT("/aps/:id/ssids/:sid", h.UpdateSSID)
		v1.DELETE("/aps/:id/ssids/:sid", h.DeleteSSID)

		// Radios
		v1.GET("/aps/:id/radios", h.ListRadioConfigs)
		v1.PUT("/aps/:id/radios/:band", h.UpsertRadioConfig)

		// Clients
		v1.GET("/aps/:id/clients", h.ListClients)
	}
}

func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// --- APs ---

func (h *Handler) ListAPs(c *gin.Context) {
	aps, err := h.svc.ListAPs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if aps == nil {
		aps = make([]*model.AP, 0)
	}
	c.JSON(http.StatusOK, aps)
}

func (h *Handler) CreateAP(c *gin.Context) {
	var req service.CreateAPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	created, err := h.svc.CreateAP(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) GetAP(c *gin.Context) {
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	found, err := h.svc.GetAP(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if found == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, found)
}

func (h *Handler) UpdateAP(c *gin.Context) {
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	var req service.UpdateAPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updated, err := h.svc.UpdateAP(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if updated == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) DeleteAP(c *gin.Context) {
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteAP(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) SyncAP(c *gin.Context) {
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	go func() {
		_ = h.rec.SyncAP(context.Background(), id)
	}()
	c.JSON(http.StatusAccepted, gin.H{"message": "sync triggered"})
}

// --- SSIDs ---

func (h *Handler) ListSSIDs(c *gin.Context) {
	apID, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	ssids, err := h.svc.ListSSIDs(c.Request.Context(), apID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if ssids == nil {
		ssids = make([]*model.SSID, 0)
	}
	c.JSON(http.StatusOK, ssids)
}

func (h *Handler) CreateSSID(c *gin.Context) {
	apID, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	var req service.CreateSSIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ssid, err := h.svc.CreateSSID(c.Request.Context(), apID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ssid)
}

func (h *Handler) UpdateSSID(c *gin.Context) {
	_, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	sid, ok := parseUUID(c, "sid")
	if !ok {
		return
	}
	var req service.UpdateSSIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ssid, err := h.svc.UpdateSSID(c.Request.Context(), sid, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if ssid == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, ssid)
}

func (h *Handler) DeleteSSID(c *gin.Context) {
	_, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	sid, ok := parseUUID(c, "sid")
	if !ok {
		return
	}
	if err := h.svc.DeleteSSID(c.Request.Context(), sid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Radio Configs ---

func (h *Handler) ListRadioConfigs(c *gin.Context) {
	apID, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	configs, err := h.svc.ListRadioConfigs(c.Request.Context(), apID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if configs == nil {
		configs = make([]*model.RadioConfig, 0)
	}
	c.JSON(http.StatusOK, configs)
}

func (h *Handler) UpsertRadioConfig(c *gin.Context) {
	apID, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	band := c.Param("band")
	var req service.UpsertRadioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rc, err := h.svc.UpsertRadioConfig(c.Request.Context(), apID, band, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rc)
}

// --- Clients ---

func (h *Handler) ListClients(c *gin.Context) {
	apID, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	clients, err := h.svc.ListClients(c.Request.Context(), apID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if clients == nil {
		clients = make([]*model.Client, 0)
	}
	c.JSON(http.StatusOK, clients)
}

// --- helpers ---

func parseUUID(c *gin.Context, param string) (uuid.UUID, bool) {
	raw := c.Param(param)
	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + param})
		return uuid.Nil, false
	}
	return id, true
}
