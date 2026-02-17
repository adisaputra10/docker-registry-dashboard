package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"docker-registry-dashboard/internal/models"
	"docker-registry-dashboard/internal/registry"
)

// GetRetentionPolicy retrieves the retention policy for a registry
func (h *Handler) GetRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid registry ID")
		return
	}

	policy, err := h.db.GetRetentionPolicy(id)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get retention policy: %v", err))
		return
	}
	h.successResponse(w, policy)
}

// SaveRetentionPolicy saves the retention policy
func (h *Handler) SaveRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid registry ID")
		return
	}

	var policy models.RetentionPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	policy.RegistryID = id
	if err := h.db.SaveRetentionPolicy(&policy); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save policy: %v", err))
		return
	}

	h.successResponse(w, policy)
}

// RunRetention executes the retention policy
func (h *Handler) RunRetention(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid registry ID")
		return
	}

	dryRunStr := r.URL.Query().Get("dry_run")

	// Get current policy
	policy, err := h.db.GetRetentionPolicy(id)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Failed to load policy")
		return
	}

	// Override dry_run if provided in query param
	if dryRunStr == "true" {
		policy.DryRun = true
	} else if dryRunStr == "false" {
		policy.DryRun = false
	}

	reg, err := h.db.GetRegistry(id)
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, "Registry not found")
		return
	}

	logs, err := registry.RunRetention(reg, policy)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Retention run failed: %v", err))
		return
	}

	// Update last run timestamp if successful
	if !policy.DryRun {
		h.db.UpdateRetentionLastRun(id)
	}

	h.successResponse(w, logs)
}
