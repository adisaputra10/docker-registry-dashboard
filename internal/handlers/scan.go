package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"docker-registry-dashboard/internal/models"
	"docker-registry-dashboard/internal/scanner"
)

type ScanRequest struct {
	RegistryID int64  `json:"registry_id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Digest     string `json:"digest"`
	Scanner    string `json:"scanner"` // "trivy" (default) or "osv"
}

// TriggerScan initiates a vulnerability scan
func (h *Handler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Verify registry exists
	registries, err := h.db.ListRegistries()
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Database error")
		return
	}

	var registry *models.Registry
	for _, reg := range registries {
		if reg.ID == req.RegistryID {
			registry = &reg
			break
		}
	}

	if registry == nil {
		h.errorResponse(w, http.StatusNotFound, "Registry not found")
		return
	}

	// Create scan record
	scan := &models.VulnerabilityScan{
		RegistryID: req.RegistryID,
		Repository: req.Repository,
		Tag:        req.Tag,
		Digest:     req.Digest,
		Status:     "scanning",
		ScannedAt:  time.Now(),
	}

	if err := h.db.SaveScan(scan); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create scan record: %v", err))
		return
	}

	// Start async scan
	go func(s *models.VulnerabilityScan, regURL string, scannerType string) {
		var report, summary string
		var err error

		if scannerType == "osv" {
			report, summary, err = scanner.ScanImageOSV(regURL, s.Repository, s.Tag)
		} else {
			if scannerType == "" {
				scannerType = "trivy"
			} // Default
			report, summary, err = scanner.ScanImage(regURL, s.Repository, s.Tag)
		}

		// Fetch existing scan to merge
		existing, errGet := h.db.GetScan(s.RegistryID, s.Repository, s.Tag)
		var existingReport, existingSummary string
		if errGet == nil && existing != nil {
			existingReport = existing.Report
			existingSummary = existing.Summary
		}

		if err != nil {
			// Merge error instead of overwrite
			errorJson := fmt.Sprintf(`{"error": "%s"}`, err.Error())
			s.Report = mergeScanData(existingReport, scannerType, errorJson)
			// Dummy summary for failed scan to ensure key existence
			s.Summary = mergeScanData(existingSummary, scannerType, `{"Unknown":0}`)

			// If other scanner data exists, don't mark as failed completely
			if existingReport != "" && existingReport != "{}" {
				s.Status = "completed"
			} else {
				s.Status = "failed"
			}
		} else {
			fmt.Printf("🎯 Scan successful! Report length: %d, Summary: %s\n", len(report), summary)
			s.Status = "completed"
			s.Report = mergeScanData(existingReport, scannerType, report)
			s.Summary = mergeScanData(existingSummary, scannerType, summary)
			fmt.Printf("📦 After merge - Report length: %d, Summary length: %d\n", len(s.Report), len(s.Summary))
		}
		s.ScannedAt = time.Now()

		// Save result
		if err := h.db.SaveScan(s); err != nil {
			fmt.Printf("❌ Failed to save scan result for scan %d: %v\n", s.ID, err)
		} else {
			fmt.Printf("✅ Scan result saved successfully!\n")
		}
	}(scan, registry.URL, req.Scanner)

	h.successResponse(w, scan)
}

// GetScanResult returns the latest scan for an image
func (h *Handler) GetScanResult(w http.ResponseWriter, r *http.Request) {
	regID := r.URL.Query().Get("registry_id")
	repo := r.URL.Query().Get("repository")
	tag := r.URL.Query().Get("tag")

	if regID == "" || repo == "" || tag == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing parameters")
		return
	}

	// Convert ID
	var id int64
	_, err := fmt.Sscanf(regID, "%d", &id)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid registry ID")
		return
	}

	scan, err := h.db.GetScan(id, repo, tag)
	if err != nil {
		// Not found is okay, return null/404?
		// Better return 404 to indicate no scan exists
		h.errorResponse(w, http.StatusNotFound, "No scan found")
		return
	}

	h.successResponse(w, scan)
}

// ListScans returns all scans for a registry
func (h *Handler) ListScans(w http.ResponseWriter, r *http.Request) {
	regID := r.URL.Query().Get("registry_id")
	if regID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing registry_id")
		return
	}

	var id int64
	_, err := fmt.Sscanf(regID, "%d", &id)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid registry ID")
		return
	}

	scans, err := h.db.ListScans(id)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Database error")
		return
	}
	if scans == nil {
		scans = []models.VulnerabilityScan{}
	}
	h.successResponse(w, scans)
}

// GetScanPolicy returns the scheduler policy
func (h *Handler) GetScanPolicy(w http.ResponseWriter, r *http.Request) {
	// Pattern: /api/registries/{id}/scan-policy

	idStr := r.PathValue("id")
	if idStr == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing registry ID")
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	p, err := h.db.GetScanPolicy(id)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.successResponse(w, p)
}

// SaveScanPolicy updates scheduler policy
func (h *Handler) SaveScanPolicy(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	var p models.ScanPolicy
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	p.RegistryID = id // Ensure ID match

	if err := h.db.SaveScanPolicy(&p); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.successResponse(w, map[string]string{"status": "saved"})
}

// VulnerabilityItem represents a single vulnerability finding
type VulnerabilityItem struct {
	ID               string    `json:"id"`
	Package          string    `json:"package"`
	Version          string    `json:"version"`
	FixedVersion     string    `json:"fixed_version"`
	Severity         string    `json:"severity"`
	Description      string    `json:"description"`
	Scanner          string    `json:"scanner"` // "trivy" or "osv"
	Repository       string    `json:"repository"`
	Tag              string    `json:"tag"`
	Digest           string    `json:"digest"`
	RegistryID       int64     `json:"registry_id"`
	ScannedAt        time.Time `json:"scanned_at"`
}

// ListVulnerabilities returns all vulnerabilities from all scans
func (h *Handler) ListVulnerabilities(w http.ResponseWriter, r *http.Request) {
	regID := r.URL.Query().Get("registry_id")
	if regID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing registry_id")
		return
	}

	var id int64
	_, err := fmt.Sscanf(regID, "%d", &id)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid registry ID")
		return
	}

	scans, err := h.db.ListScans(id)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Database error")
		return
	}

	var vulnerabilities []VulnerabilityItem

	for _, scan := range scans {
		if scan.Status != "completed" || scan.Report == "" {
			continue
		}

		// Parse report - it's wrapped with scanner keys
		var reportWrapper map[string]json.RawMessage
		if err := json.Unmarshal([]byte(scan.Report), &reportWrapper); err != nil {
			continue
		}

		// Extract Trivy vulnerabilities
		if trivyData, ok := reportWrapper["trivy"]; ok {
			trivyVulns := extractTrivyVulnerabilities(trivyData, scan)
			vulnerabilities = append(vulnerabilities, trivyVulns...)
		}

		// Extract OSV vulnerabilities
		if osvData, ok := reportWrapper["osv"]; ok {
			osvVulns := extractOSVVulnerabilities(osvData, scan)
			vulnerabilities = append(vulnerabilities, osvVulns...)
		}
	}

	h.successResponse(w, vulnerabilities)
}

func extractTrivyVulnerabilities(data json.RawMessage, scan models.VulnerabilityScan) []VulnerabilityItem {
	var result []VulnerabilityItem
	
	var trivyReport scanner.TrivyReport
	if err := json.Unmarshal(data, &trivyReport); err != nil {
		return result
	}

	for _, res := range trivyReport.Results {
		for _, vuln := range res.Vulnerabilities {
			item := VulnerabilityItem{
				ID:           vuln.VulnerabilityID,
				Package:      vuln.PkgName,
				Version:      vuln.InstalledVersion,
				FixedVersion: vuln.FixedVersion,
				Severity:     vuln.Severity,
				Description:  vuln.Title,
				Scanner:      "Trivy",
				Repository:   scan.Repository,
				Tag:          scan.Tag,
				Digest:       scan.Digest,
				RegistryID:   scan.RegistryID,
				ScannedAt:    scan.ScannedAt,
			}
			result = append(result, item)
		}
	}

	return result
}

func extractOSVVulnerabilities(data json.RawMessage, scan models.VulnerabilityScan) []VulnerabilityItem {
	var result []VulnerabilityItem
	
	var osvOutput scanner.OSVOutput
	if err := json.Unmarshal(data, &osvOutput); err != nil {
		return result
	}

	for _, res := range osvOutput.Results {
		for _, pkg := range res.Packages {
			for _, vuln := range pkg.Vulnerabilities {
				severity := "UNKNOWN"
				if len(vuln.Severity) > 0 {
					severity = vuln.Severity[0].Score
				}

				item := VulnerabilityItem{
					ID:           vuln.ID,
					Package:      pkg.Package.Name,
					Version:      pkg.Package.Version,
					FixedVersion: "",
					Severity:     severity,
					Description:  vuln.Summary,
					Scanner:      "OSV",
					Repository:   scan.Repository,
					Tag:          scan.Tag,
					Digest:       scan.Digest,
					RegistryID:   scan.RegistryID,
					ScannedAt:    scan.ScannedAt,
				}
				result = append(result, item)
			}
		}
	}

	return result
}

func mergeScanData(originalJSON, key string, newJSON string) string {
	data := make(map[string]json.RawMessage)

	// Try parse original
	var parsedOriginal map[string]json.RawMessage
	if originalJSON != "" {
		if err := json.Unmarshal([]byte(originalJSON), &parsedOriginal); err == nil {
			// Check if it has scanner keys
			_, hasTrivy := parsedOriginal["trivy"]
			_, hasOsv := parsedOriginal["osv"]
			if hasTrivy || hasOsv {
				data = parsedOriginal
			} else {
				// Not wrapped, assume old format is trivy
				data["trivy"] = json.RawMessage(originalJSON)
			}
		} else {
			// Failed to parse as map, maybe it's just a string or broken.
			// Try to treat as raw trivy result
			data["trivy"] = json.RawMessage(originalJSON)
		}
	}

	if newJSON != "" {
		data[key] = json.RawMessage(newJSON)
	}

	b, _ := json.Marshal(data)
	return string(b)
}
