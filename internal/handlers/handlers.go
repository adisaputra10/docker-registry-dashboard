package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"docker-registry-dashboard/internal/database"
	"docker-registry-dashboard/internal/models"
	"docker-registry-dashboard/internal/registry"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	db          *database.DB
	embeddedReg *registry.EmbeddedRegistry
}

// New creates a new Handler
func New(db *database.DB, embeddedReg *registry.EmbeddedRegistry) *Handler {
	return &Handler{db: db, embeddedReg: embeddedReg}
}

// --- Helper methods ---

func (h *Handler) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) successResponse(w http.ResponseWriter, data interface{}) {
	h.jsonResponse(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    data,
	})
}

func (h *Handler) messageResponse(w http.ResponseWriter, message string) {
	h.jsonResponse(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: message,
	})
}

func (h *Handler) errorResponse(w http.ResponseWriter, status int, err string) {
	h.jsonResponse(w, status, models.APIResponse{
		Success: false,
		Error:   err,
	})
}

func (h *Handler) getRegistryID(r *http.Request) (int64, error) {
	idStr := r.PathValue("id")
	return strconv.ParseInt(idStr, 10, 64)
}

// --- Dashboard ---

// GetDashboardStats returns overview statistics
func (h *Handler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	registries, err := h.db.ListRegistries()
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Failed to load registries")
		return
	}

	stats := models.DashboardStats{
		TotalRegistries: len(registries),
	}

	storageConfig, err := h.db.GetStorageConfig()
	if err == nil {
		stats.StorageType = storageConfig.Type
	}

	// Get embedded registry status
	if h.embeddedReg != nil {
		stats.EmbeddedRegistry = h.embeddedReg.Status()
	}

	for _, reg := range registries {
		regStat := models.RegistryStats{
			ID:   reg.ID,
			Name: reg.Name,
			URL:  reg.URL,
		}

		client := registry.NewClientFromRegistry(&reg)
		if err := client.Ping(); err != nil {
			regStat.Status = "offline"
			log.Printf("Registry %s is offline: %v", reg.Name, err)
		} else {
			regStat.Status = "online"
			repos, err := client.ListRepositories()
			if err == nil {
				regStat.ImageCount = len(repos)
				stats.TotalImages += len(repos)

				// Count tags for each repo
				for _, repo := range repos {
					tags, err := client.ListTags(repo.Name)
					if err == nil {
						stats.TotalTags += len(tags)
					}
				}
			}
		}

		stats.Registries = append(stats.Registries, regStat)
	}

	h.successResponse(w, stats)
}

// --- Registry CRUD ---

// ListRegistries returns all registries
func (h *Handler) ListRegistries(w http.ResponseWriter, r *http.Request) {
	registries, err := h.db.ListRegistries()
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Failed to load registries")
		return
	}
	if registries == nil {
		registries = []models.Registry{}
	}
	h.successResponse(w, registries)
}

// CreateRegistry adds a new registry
func (h *Handler) CreateRegistry(w http.ResponseWriter, r *http.Request) {
	var reg models.Registry
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if reg.Name == "" || reg.URL == "" {
		h.errorResponse(w, http.StatusBadRequest, "Name and URL are required")
		return
	}

	// Normalize URL - remove trailing slash
	reg.URL = strings.TrimRight(reg.URL, "/")

	if err := h.db.CreateRegistry(&reg); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Failed to create registry")
		return
	}

	h.jsonResponse(w, http.StatusCreated, models.APIResponse{
		Success: true,
		Data:    reg,
		Message: "Registry created successfully",
	})
}

// UpdateRegistry updates an existing registry
func (h *Handler) UpdateRegistry(w http.ResponseWriter, r *http.Request) {
	id, err := h.getRegistryID(r)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid registry ID")
		return
	}

	var reg models.Registry
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	reg.ID = id
	reg.URL = strings.TrimRight(reg.URL, "/")

	if err := h.db.UpdateRegistry(&reg); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Failed to update registry")
		return
	}

	h.messageResponse(w, "Registry updated successfully")
}

// DeleteRegistry removes a registry
func (h *Handler) DeleteRegistry(w http.ResponseWriter, r *http.Request) {
	id, err := h.getRegistryID(r)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid registry ID")
		return
	}

	if err := h.db.DeleteRegistry(id); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Failed to delete registry")
		return
	}

	h.messageResponse(w, "Registry deleted successfully")
}

// TestRegistryConnection tests the connection to a registry
func (h *Handler) TestRegistryConnection(w http.ResponseWriter, r *http.Request) {
	id, err := h.getRegistryID(r)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid registry ID")
		return
	}

	reg, err := h.db.GetRegistry(id)
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, "Registry not found")
		return
	}

	client := registry.NewClientFromRegistry(reg)
	start := time.Now()
	if err := client.Ping(); err != nil {
		h.errorResponse(w, http.StatusBadGateway, fmt.Sprintf("Connection failed: %v", err))
		return
	}
	duration := time.Since(start)

	h.successResponse(w, map[string]interface{}{
		"status":     "connected",
		"latency_ms": duration.Milliseconds(),
		"registry":   reg.Name,
	})
}

// --- Repository/Image browsing ---

// ListRepositories returns all repositories from a registry
func (h *Handler) ListRepositories(w http.ResponseWriter, r *http.Request) {
	id, err := h.getRegistryID(r)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid registry ID")
		return
	}

	reg, err := h.db.GetRegistry(id)
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, "Registry not found")
		return
	}

	client := registry.NewClientFromRegistry(reg)
	repos, err := client.ListRepositories()
	if err != nil {
		h.errorResponse(w, http.StatusBadGateway, fmt.Sprintf("Failed to list repositories: %v", err))
		return
	}

	// Fetch tag counts for each repo
	for i := range repos {
		tags, err := client.ListTags(repos[i].Name)
		if err == nil {
			repos[i].TagCount = len(tags)
		}
	}

	h.successResponse(w, repos)
}

// ListTags returns all tags for a repository
func (h *Handler) ListTags(w http.ResponseWriter, r *http.Request) {
	id, err := h.getRegistryID(r)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid registry ID")
		return
	}

	repoName := r.URL.Query().Get("repo")
	if repoName == "" {
		h.errorResponse(w, http.StatusBadRequest, "Repository name is required (query param: repo)")
		return
	}

	reg, err := h.db.GetRegistry(id)
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, "Registry not found")
		return
	}

	client := registry.NewClientFromRegistry(reg)
	tags, err := client.ListTags(repoName)
	if err != nil {
		h.errorResponse(w, http.StatusBadGateway, fmt.Sprintf("Failed to list tags: %v", err))
		return
	}

	// Optionally get digest for each tag
	for i := range tags {
		digest, err := client.GetDigestForTag(repoName, tags[i].Name)
		if err == nil {
			tags[i].Digest = digest
		}
	}

	h.successResponse(w, tags)
}

// GetManifest returns the manifest for a specific tag
func (h *Handler) GetManifest(w http.ResponseWriter, r *http.Request) {
	id, err := h.getRegistryID(r)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid registry ID")
		return
	}

	repoName := r.URL.Query().Get("repo")
	tag := r.URL.Query().Get("tag")
	if repoName == "" || tag == "" {
		h.errorResponse(w, http.StatusBadRequest, "Repository name and tag are required")
		return
	}

	reg, err := h.db.GetRegistry(id)
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, "Registry not found")
		return
	}

	client := registry.NewClientFromRegistry(reg)
	manifest, err := client.GetManifest(repoName, tag)
	if err != nil {
		h.errorResponse(w, http.StatusBadGateway, fmt.Sprintf("Failed to get manifest: %v", err))
		return
	}

	h.successResponse(w, manifest)
}

// DeleteTag deletes a tag from a repository
func (h *Handler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	id, err := h.getRegistryID(r)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid registry ID")
		return
	}

	repoName := r.URL.Query().Get("repo")
	tag := r.URL.Query().Get("tag")
	if repoName == "" || tag == "" {
		h.errorResponse(w, http.StatusBadRequest, "Repository name and tag are required")
		return
	}

	reg, err := h.db.GetRegistry(id)
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, "Registry not found")
		return
	}

	client := registry.NewClientFromRegistry(reg)

	// First get the digest for this tag
	digest, err := client.GetDigestForTag(repoName, tag)
	if err != nil {
		h.errorResponse(w, http.StatusBadGateway, fmt.Sprintf("Failed to get digest: %v", err))
		return
	}

	// Delete the manifest by digest
	if err := client.DeleteManifest(repoName, digest); err != nil {
		h.errorResponse(w, http.StatusBadGateway, fmt.Sprintf("Failed to delete tag: %v", err))
		return
	}

	h.messageResponse(w, fmt.Sprintf("Tag %s:%s deleted successfully", repoName, tag))
}

// --- Storage Configuration ---

// GetStorageConfig returns the current storage configuration
func (h *Handler) GetStorageConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.db.GetStorageConfig()
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Failed to load storage config")
		return
	}
	h.successResponse(w, config)
}

// SaveStorageConfig saves the storage configuration and restarts the registry
func (h *Handler) SaveStorageConfig(w http.ResponseWriter, r *http.Request) {
	var config models.StorageConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if config.Type == "" {
		h.errorResponse(w, http.StatusBadRequest, "Storage type is required")
		return
	}

	if err := h.db.SaveStorageConfig(&config); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Failed to save storage config")
		return
	}

	// Restart the embedded registry with new storage config
	restartMsg := ""
	if h.embeddedReg != nil {
		go func() {
			if err := h.embeddedReg.Restart(&config); err != nil {
				log.Printf("⚠️  Failed to restart registry: %v", err)
			}
		}()
		restartMsg = " Registry is restarting with new configuration."
	}

	h.jsonResponse(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    config,
		Message: "Storage configuration saved successfully." + restartMsg,
	})
}

// TestStorageConnection tests the storage backend connection
func (h *Handler) TestStorageConnection(w http.ResponseWriter, r *http.Request) {
	var config models.StorageConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	switch config.Type {
	case "local":
		if config.LocalPath == "" {
			h.errorResponse(w, http.StatusBadRequest, "Local path is required")
			return
		}
		info, err := os.Stat(config.LocalPath)
		if err != nil {
			if os.IsNotExist(err) {
				h.successResponse(w, map[string]interface{}{
					"status":  "warning",
					"message": "Path does not exist yet, but will be created when registry starts",
				})
				return
			}
			h.errorResponse(w, http.StatusBadRequest, fmt.Sprintf("Cannot access path: %v", err))
			return
		}
		if !info.IsDir() {
			h.errorResponse(w, http.StatusBadRequest, "Path exists but is not a directory")
			return
		}
		h.successResponse(w, map[string]string{
			"status":  "connected",
			"message": "Local storage path is accessible",
		})

	case "s3":
		if config.S3Endpoint == "" || config.S3Bucket == "" {
			h.errorResponse(w, http.StatusBadRequest, "S3 endpoint and bucket are required")
			return
		}
		host := config.S3Endpoint
		if !strings.Contains(host, ":") {
			if config.S3UseSSL {
				host += ":443"
			} else {
				host += ":80"
			}
		}
		conn, err := net.DialTimeout("tcp", host, 5*time.Second)
		if err != nil {
			h.errorResponse(w, http.StatusBadGateway, fmt.Sprintf("Cannot connect to S3 endpoint: %v", err))
			return
		}
		conn.Close()
		h.successResponse(w, map[string]string{
			"status":  "connected",
			"message": "S3 endpoint is reachable",
		})

	case "sftp":
		if config.SFTPHost == "" || config.SFTPUser == "" {
			h.errorResponse(w, http.StatusBadRequest, "SFTP host and user are required")
			return
		}
		port := config.SFTPPort
		if port == 0 {
			port = 22
		}
		addr := net.JoinHostPort(config.SFTPHost, strconv.Itoa(port))
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			h.errorResponse(w, http.StatusBadGateway, fmt.Sprintf("Cannot connect to SFTP server: %v", err))
			return
		}
		conn.Close()
		h.successResponse(w, map[string]string{
			"status":  "connected",
			"message": "SFTP server is reachable",
		})

	default:
		h.errorResponse(w, http.StatusBadRequest, "Invalid storage type")
	}
}

// --- Embedded Registry Management ---

// GetEmbeddedRegistryStatus returns the status of the embedded registry
func (h *Handler) GetEmbeddedRegistryStatus(w http.ResponseWriter, r *http.Request) {
	if h.embeddedReg == nil {
		h.successResponse(w, map[string]interface{}{
			"running":          false,
			"docker_available": false,
			"message":          "Embedded registry is not configured",
		})
		return
	}
	h.successResponse(w, h.embeddedReg.Status())
}

// RestartEmbeddedRegistry restarts the embedded registry with current storage config
func (h *Handler) RestartEmbeddedRegistry(w http.ResponseWriter, r *http.Request) {
	if h.embeddedReg == nil {
		h.errorResponse(w, http.StatusServiceUnavailable, "Embedded registry is not available")
		return
	}

	config, err := h.db.GetStorageConfig()
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Failed to load storage config")
		return
	}

	if err := h.embeddedReg.Restart(config); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to restart registry: %v", err))
		return
	}

	h.messageResponse(w, "Registry restarted successfully")
}

// StopEmbeddedRegistry stops the embedded registry
func (h *Handler) StopEmbeddedRegistry(w http.ResponseWriter, r *http.Request) {
	if h.embeddedReg == nil {
		h.errorResponse(w, http.StatusServiceUnavailable, "Embedded registry is not available")
		return
	}

	if err := h.embeddedReg.Stop(); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to stop registry: %v", err))
		return
	}

	h.messageResponse(w, "Registry stopped")
}

// StartEmbeddedRegistry starts the embedded registry
func (h *Handler) StartEmbeddedRegistry(w http.ResponseWriter, r *http.Request) {
	if h.embeddedReg == nil {
		h.errorResponse(w, http.StatusServiceUnavailable, "Embedded registry is not available")
		return
	}

	config, err := h.db.GetStorageConfig()
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Failed to load storage config")
		return
	}

	if err := h.embeddedReg.Start(config); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start registry: %v", err))
		return
	}

	h.messageResponse(w, "Registry started successfully")
}

// GetEmbeddedRegistryLogs returns recent container logs
func (h *Handler) GetEmbeddedRegistryLogs(w http.ResponseWriter, r *http.Request) {
	if h.embeddedReg == nil {
		h.errorResponse(w, http.StatusServiceUnavailable, "Embedded registry is not available")
		return
	}

	logs, err := h.embeddedReg.GetContainerLogs(100)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get logs: %v", err))
		return
	}

	h.successResponse(w, map[string]string{
		"logs": logs,
	})
}
