package models

import "time"

// Registry represents a Docker Registry V2 connection
type Registry struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Username  string    `json:"username,omitempty"`
	Password  string    `json:"password,omitempty"`
	Insecure  bool      `json:"insecure"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StorageConfig represents storage backend configuration
type StorageConfig struct {
	ID   int64  `json:"id"`
	Type string `json:"type"` // local, s3, sftp

	// Local storage
	LocalPath string `json:"local_path,omitempty"`

	// S3 / Object Storage
	S3Endpoint  string `json:"s3_endpoint,omitempty"`
	S3Bucket    string `json:"s3_bucket,omitempty"`
	S3Region    string `json:"s3_region,omitempty"`
	S3AccessKey string `json:"s3_access_key,omitempty"`
	S3SecretKey string `json:"s3_secret_key,omitempty"`
	S3UseSSL    bool   `json:"s3_use_ssl"`

	// SFTP
	SFTPHost       string `json:"sftp_host,omitempty"`
	SFTPPort       int    `json:"sftp_port,omitempty"`
	SFTPUser       string `json:"sftp_user,omitempty"`
	SFTPPassword   string `json:"sftp_password,omitempty"`
	SFTPPrivateKey string `json:"sftp_private_key,omitempty"`
	SFTPPath       string `json:"sftp_path,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RetentionPolicy defines rules for image cleanup
type RetentionPolicy struct {
	ID            int64     `json:"id"`
	RegistryID    int64     `json:"registry_id"`
	KeepLastCount int       `json:"keep_last_count"` // Keep last N images
	KeepDays      int       `json:"keep_days"`       // Keep images newer than N days
	DryRun        bool      `json:"dry_run"`         // If true, don't actually delete
	LastRunAt     time.Time `json:"last_run_at"`
	FilterRepos   string    `json:"filter_repos"`  // Regex to select specific repos (empty=all)
	ExcludeRepos  string    `json:"exclude_repos"` // Regex to exclude specific repos
	ExcludeTags   string    `json:"exclude_tags"`  // Regex to exclude specific tags (e.g. "latest")
}

// ScanPolicy defines rules for vulnerability scanning
type ScanPolicy struct {
	ID            int64     `json:"id"`
	RegistryID    int64     `json:"registry_id"`
	Enabled       bool      `json:"enabled"`
	IntervalHours int       `json:"interval_hours"` // Run every N hours
	NextRunAt     time.Time `json:"next_run_at"`
	LastRunAt     time.Time `json:"last_run_at"`
	FilterRepos   string    `json:"filter_repos"` // Regex to include repos
	FilterTags    string    `json:"filter_tags"`  // Regex to include tags
}

// VulnerabilityScan represents a trivy scan result
type VulnerabilityScan struct {
	ID         int64     `json:"id"`
	RegistryID int64     `json:"registry_id"`
	Repository string    `json:"repository"`
	Tag        string    `json:"tag"`
	Digest     string    `json:"digest"`
	Status     string    `json:"status"`  // pending, scanning, completed, failed
	Summary    string    `json:"summary"` // JSON string of severity counts
	Report     string    `json:"report"`  // Full JSON report (compressed/text)
	ScannedAt  time.Time `json:"scanned_at"`
}

// RetentionLog represents the result of a retention run
type RetentionLog struct {
	Repository string    `json:"repository"`
	Tag        string    `json:"tag"`
	Digest     string    `json:"digest"`
	Created    time.Time `json:"created"`
	Action     string    `json:"action"` // "kept" or "deleted" (or "would_delete")
	Reason     string    `json:"reason"`
}

// Repository represents a Docker image repository
type Repository struct {
	Name     string `json:"name"`
	TagCount int    `json:"tag_count,omitempty"`
}

// Tag represents a Docker image tag
type Tag struct {
	Name   string `json:"name"`
	Digest string `json:"digest,omitempty"`
}

// ImageManifest represents manifest details
type ImageManifest struct {
	SchemaVersion int             `json:"schemaVersion"`
	MediaType     string          `json:"mediaType"`
	Digest        string          `json:"digest"`
	TotalSize     int64           `json:"totalSize"`
	Layers        []ManifestLayer `json:"layers,omitempty"`
	Config        *ManifestConfig `json:"config,omitempty"`
	Platform      *Platform       `json:"platform,omitempty"`
}

// ManifestLayer represents a layer in the manifest
type ManifestLayer struct {
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Digest    string `json:"digest"`
}

// ManifestConfig represents the config descriptor
type ManifestConfig struct {
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Digest    string `json:"digest"`
}

// Platform represents OS/architecture info
type Platform struct {
	Architecture string `json:"architecture,omitempty"`
	OS           string `json:"os,omitempty"`
}

// DashboardStats for the overview page
type DashboardStats struct {
	TotalRegistries  int                    `json:"total_registries"`
	TotalImages      int                    `json:"total_images"`
	TotalTags        int                    `json:"total_tags"`
	StorageType      string                 `json:"storage_type"`
	Registries       []RegistryStats        `json:"registries"`
	EmbeddedRegistry map[string]interface{} `json:"embedded_registry,omitempty"`
}

// RegistryStats per-registry statistics
type RegistryStats struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	ImageCount int    `json:"image_count"`
	Status     string `json:"status"` // online, offline, error
}

// APIResponse standard API response wrapper
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}
