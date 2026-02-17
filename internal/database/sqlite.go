package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"docker-registry-dashboard/internal/models"

	_ "modernc.org/sqlite"
)

// DB wraps the SQL database connection
type DB struct {
	conn *sql.DB
}

// New creates a new database connection and initializes schema
func New(dbPath string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better performance
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS registries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		url TEXT NOT NULL,
		username TEXT DEFAULT '',
		password TEXT DEFAULT '',
		insecure INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS storage_configs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL DEFAULT 'local',
		local_path TEXT DEFAULT '',
		s3_endpoint TEXT DEFAULT '',
		s3_bucket TEXT DEFAULT '',
		s3_region TEXT DEFAULT '',
		s3_access_key TEXT DEFAULT '',
		s3_secret_key TEXT DEFAULT '',
		s3_use_ssl INTEGER DEFAULT 0,
		sftp_host TEXT DEFAULT '',
		sftp_port INTEGER DEFAULT 22,
		sftp_user TEXT DEFAULT '',
		sftp_password TEXT DEFAULT '',
		sftp_private_key TEXT DEFAULT '',
		sftp_path TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS retention_policies (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		registry_id INTEGER NOT NULL UNIQUE,
		keep_last_count INTEGER DEFAULT 0,
		keep_days INTEGER DEFAULT 0,
		dry_run INTEGER DEFAULT 1,
		last_run_at DATETIME,
		filter_repos TEXT DEFAULT '',
		exclude_repos TEXT DEFAULT '',
		exclude_tags TEXT DEFAULT '',
		FOREIGN KEY(registry_id) REFERENCES registries(id) ON DELETE CASCADE
	);
	`
	if _, err := db.conn.Exec(schema); err != nil {
		return err
	}

	// Migrations for existing tables (ignore errors if columns exist)
	// We use a simple way: try to add column, ignore error.
	// In Go sqlite driver, we can't easily suppress specific errors without parsing string.
	// But Exec will return error if column exists. We can ignore it.

	scanPolicySchema := `
	CREATE TABLE IF NOT EXISTS scan_policies (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		registry_id INTEGER NOT NULL UNIQUE,
		enabled BOOLEAN DEFAULT 0,
		interval_hours INTEGER DEFAULT 24,
		next_run_at DATETIME,
		last_run_at DATETIME,
		filter_repos TEXT DEFAULT '',
		filter_tags TEXT DEFAULT '',
		FOREIGN KEY(registry_id) REFERENCES registries(id) ON DELETE CASCADE
	);
	`
	if _, err := db.conn.Exec(scanPolicySchema); err != nil {
		return err
	}
	db.conn.Exec("ALTER TABLE scan_policies ADD COLUMN filter_tags TEXT DEFAULT ''")
	db.conn.Exec("ALTER TABLE retention_policies ADD COLUMN filter_repos TEXT DEFAULT ''")
	db.conn.Exec("ALTER TABLE retention_policies ADD COLUMN exclude_repos TEXT DEFAULT ''")
	db.conn.Exec("ALTER TABLE retention_policies ADD COLUMN exclude_tags TEXT DEFAULT ''")
	db.conn.Exec("ALTER TABLE scan_policies ADD COLUMN filter_tags TEXT DEFAULT ''")

	// Vulnerability Scans table
	_, err := db.conn.Exec(`CREATE TABLE IF NOT EXISTS vuln_scans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		registry_id INTEGER,
		repository TEXT,
		tag TEXT,
		digest TEXT,
		status TEXT,
		summary TEXT,
		report TEXT,
		scanned_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(registry_id) REFERENCES registries(id) ON DELETE CASCADE
	)`)
	if err != nil {
		return err
	}

	return nil
}

// ... (existing code omitted) ...

// --- Vulnerability Scans CRUD ---

// SaveScan saves or updates a scan result
func (db *DB) SaveScan(s *models.VulnerabilityScan) error {
	// Check if exists for same repo:tag
	var id int64
	err := db.conn.QueryRow("SELECT id FROM vuln_scans WHERE registry_id=? AND repository=? AND tag=?", s.RegistryID, s.Repository, s.Tag).Scan(&id)

	if err == nil {
		// Update
		fmt.Printf("📝 Updating scan for %s:%s. Report size: %d, Summary size: %d, Status: %s\n", s.Repository, s.Tag, len(s.Report), len(s.Summary), s.Status)
		_, err = db.conn.Exec(`
			UPDATE vuln_scans SET digest=?, status=?, summary=?, report=?, scanned_at=?
			WHERE id=?
		`, s.Digest, s.Status, s.Summary, s.Report, s.ScannedAt, id)
		s.ID = id
		if err != nil {
			fmt.Printf("❌ SaveScan UPDATE error: %v\n", err)
			return err
		}
	} else if err == sql.ErrNoRows {
		// Insert new record
		fmt.Printf("➕ Inserting new scan for %s:%s. Report size: %d, Summary size: %d, Status: %s\n", s.Repository, s.Tag, len(s.Report), len(s.Summary), s.Status)
		res, execErr := db.conn.Exec(`
			INSERT INTO vuln_scans (registry_id, repository, tag, digest, status, summary, report, scanned_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, s.RegistryID, s.Repository, s.Tag, s.Digest, s.Status, s.Summary, s.Report, s.ScannedAt)
		if execErr != nil {
			fmt.Printf("❌ SaveScan INSERT error: %v\n", execErr)
			return execErr
		}
		s.ID, _ = res.LastInsertId()
	} else {
		fmt.Printf("❌ SaveScan QueryRow error: %v\n", err)
		return err
	}
	return nil
}

// GetScan returns the latest scan for an image
func (db *DB) GetScan(registryID int64, repo, tag string) (*models.VulnerabilityScan, error) {
	var s models.VulnerabilityScan
	var scannedAt sql.NullTime
	err := db.conn.QueryRow(`
		SELECT id, registry_id, repository, tag, digest, status, summary, report, scanned_at
		FROM vuln_scans WHERE registry_id=? AND repository=? AND tag=?
	`, registryID, repo, tag).Scan(&s.ID, &s.RegistryID, &s.Repository, &s.Tag, &s.Digest, &s.Status, &s.Summary, &s.Report, &scannedAt)

	if err != nil {
		return nil, err
	}
	if scannedAt.Valid {
		s.ScannedAt = scannedAt.Time
	}
	return &s, nil
}

// ListScans returns all scans for a registry
func (db *DB) ListScans(registryID int64) ([]models.VulnerabilityScan, error) {
	rows, err := db.conn.Query(`
		SELECT id, registry_id, repository, tag, digest, status, summary, report, scanned_at
		FROM vuln_scans WHERE registry_id=? ORDER BY scanned_at DESC
	`, registryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scans []models.VulnerabilityScan
	for rows.Next() {
		var s models.VulnerabilityScan
		var scannedAt sql.NullTime
		if err := rows.Scan(&s.ID, &s.RegistryID, &s.Repository, &s.Tag, &s.Digest, &s.Status, &s.Summary, &s.Report, &scannedAt); err != nil {
			continue
		}
		if scannedAt.Valid {
			s.ScannedAt = scannedAt.Time
		}
		scans = append(scans, s)
	}
	return scans, nil
}

// --- Scheduler Policies ---

// GetScanPolicy returns the policy for a registry, or default if not set
func (db *DB) GetScanPolicy(registryID int64) (*models.ScanPolicy, error) {
	row := db.conn.QueryRow(`
		SELECT id, registry_id, enabled, interval_hours, next_run_at, last_run_at, filter_repos, filter_tags 
		FROM scan_policies WHERE registry_id=?`, registryID)

	p := &models.ScanPolicy{RegistryID: registryID, IntervalHours: 24, FilterTags: "latest"}
	var nextRun, lastRun sql.NullTime
	if err := row.Scan(&p.ID, &p.RegistryID, &p.Enabled, &p.IntervalHours, &nextRun, &lastRun, &p.FilterRepos, &p.FilterTags); err != nil {
		if err == sql.ErrNoRows {
			return p, nil
		}
		return nil, err
	}
	if nextRun.Valid {
		p.NextRunAt = nextRun.Time
	}
	if lastRun.Valid {
		p.LastRunAt = lastRun.Time
	}
	return p, nil
}

// SaveScanPolicy creates or updates a policy
func (db *DB) SaveScanPolicy(p *models.ScanPolicy) error {
	_, err := db.conn.Exec(`
		INSERT INTO scan_policies (registry_id, enabled, interval_hours, next_run_at, last_run_at, filter_repos, filter_tags)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(registry_id) DO UPDATE SET
			enabled=excluded.enabled,
			interval_hours=excluded.interval_hours,
			next_run_at=excluded.next_run_at,
			filter_repos=excluded.filter_repos,
			filter_tags=excluded.filter_tags
	`, p.RegistryID, p.Enabled, p.IntervalHours, p.NextRunAt, p.LastRunAt, p.FilterRepos, p.FilterTags)
	return err
}

// ListEnabledScanPolicies returns policies that are enabled
func (db *DB) ListEnabledScanPolicies() ([]models.ScanPolicy, error) {
	rows, err := db.conn.Query(`
		SELECT id, registry_id, enabled, interval_hours, next_run_at, last_run_at, filter_repos, filter_tags
		FROM scan_policies WHERE enabled=1
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []models.ScanPolicy
	for rows.Next() {
		var p models.ScanPolicy
		var nextRun, lastRun sql.NullTime
		if err := rows.Scan(&p.ID, &p.RegistryID, &p.Enabled, &p.IntervalHours, &nextRun, &lastRun, &p.FilterRepos, &p.FilterTags); err != nil {
			continue
		}
		if nextRun.Valid {
			p.NextRunAt = nextRun.Time
		}
		if lastRun.Valid {
			p.LastRunAt = lastRun.Time
		}
		policies = append(policies, p)
	}
	return policies, nil
}

// UpdateNextRunAt updates the next run time after a job execution
func (db *DB) UpdatePolicyRunTime(id int64, lastRun, nextRun time.Time) error {
	_, err := db.conn.Exec("UPDATE scan_policies SET last_run_at=?, next_run_at=? WHERE id=?", lastRun, nextRun, id)
	return err
}

// ListRegistries returns all registries
func (db *DB) ListRegistries() ([]models.Registry, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, url, username, password, insecure, created_at, updated_at
		FROM registries ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var registries []models.Registry
	for rows.Next() {
		var r models.Registry
		var insecure int
		err := rows.Scan(&r.ID, &r.Name, &r.URL, &r.Username, &r.Password, &insecure, &r.CreatedAt, &r.UpdatedAt)
		if err != nil {
			return nil, err
		}
		r.Insecure = insecure == 1
		registries = append(registries, r)
	}
	return registries, nil
}

// GetRegistry returns a single registry by ID
func (db *DB) GetRegistry(id int64) (*models.Registry, error) {
	var r models.Registry
	var insecure int
	err := db.conn.QueryRow(`
		SELECT id, name, url, username, password, insecure, created_at, updated_at
		FROM registries WHERE id = ?
	`, id).Scan(&r.ID, &r.Name, &r.URL, &r.Username, &r.Password, &insecure, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	r.Insecure = insecure == 1
	return &r, nil
}

// CreateRegistry creates a new registry
func (db *DB) CreateRegistry(r *models.Registry) error {
	insecure := 0
	if r.Insecure {
		insecure = 1
	}
	now := time.Now()
	result, err := db.conn.Exec(`
		INSERT INTO registries (name, url, username, password, insecure, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, r.Name, r.URL, r.Username, r.Password, insecure, now, now)
	if err != nil {
		return err
	}
	r.ID, err = result.LastInsertId()
	r.CreatedAt = now
	r.UpdatedAt = now
	return err
}

// UpdateRegistry updates an existing registry
func (db *DB) UpdateRegistry(r *models.Registry) error {
	insecure := 0
	if r.Insecure {
		insecure = 1
	}
	now := time.Now()
	_, err := db.conn.Exec(`
		UPDATE registries SET name=?, url=?, username=?, password=?, insecure=?, updated_at=?
		WHERE id=?
	`, r.Name, r.URL, r.Username, r.Password, insecure, now, r.ID)
	r.UpdatedAt = now
	return err
}

// DeleteRegistry deletes a registry
func (db *DB) DeleteRegistry(id int64) error {
	_, err := db.conn.Exec("DELETE FROM registries WHERE id = ?", id)
	return err
}

// --- Storage Config CRUD ---

// GetStorageConfig returns the current storage configuration
func (db *DB) GetStorageConfig() (*models.StorageConfig, error) {
	var s models.StorageConfig
	var useSSL int
	err := db.conn.QueryRow(`
		SELECT id, type, local_path, s3_endpoint, s3_bucket, s3_region, s3_access_key, s3_secret_key, s3_use_ssl,
		       sftp_host, sftp_port, sftp_user, sftp_password, sftp_private_key, sftp_path, created_at, updated_at
		FROM storage_configs ORDER BY id DESC LIMIT 1
	`).Scan(&s.ID, &s.Type, &s.LocalPath, &s.S3Endpoint, &s.S3Bucket, &s.S3Region, &s.S3AccessKey, &s.S3SecretKey, &useSSL,
		&s.SFTPHost, &s.SFTPPort, &s.SFTPUser, &s.SFTPPassword, &s.SFTPPrivateKey, &s.SFTPPath, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		// Return default config
		return &models.StorageConfig{Type: "local", LocalPath: "/var/lib/registry"}, nil
	}
	if err != nil {
		return nil, err
	}
	s.S3UseSSL = useSSL == 1
	return &s, nil
}

// SaveStorageConfig saves or updates storage configuration
func (db *DB) SaveStorageConfig(s *models.StorageConfig) error {
	now := time.Now()
	useSSL := 0
	if s.S3UseSSL {
		useSSL = 1
	}

	// Delete existing config and insert new one (only keep one config)
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM storage_configs")
	if err != nil {
		return err
	}

	result, err := tx.Exec(`
		INSERT INTO storage_configs (type, local_path, s3_endpoint, s3_bucket, s3_region, s3_access_key, s3_secret_key, s3_use_ssl,
		                             sftp_host, sftp_port, sftp_user, sftp_password, sftp_private_key, sftp_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.Type, s.LocalPath, s.S3Endpoint, s.S3Bucket, s.S3Region, s.S3AccessKey, s.S3SecretKey, useSSL,
		s.SFTPHost, s.SFTPPort, s.SFTPUser, s.SFTPPassword, s.SFTPPrivateKey, s.SFTPPath, now, now)
	if err != nil {
		return err
	}

	s.ID, _ = result.LastInsertId()
	s.CreatedAt = now
	s.UpdatedAt = now

	return tx.Commit()
}

// RegistryEntry is a simplified struct for auto-registration
type RegistryEntry struct {
	Name string
	URL  string
}

// CreateRegistryEntry creates a registry with minimal info (for auto-registration)
func (db *DB) CreateRegistryEntry(entry *RegistryEntry) error {
	now := time.Now()
	_, err := db.conn.Exec(`
		INSERT INTO registries (name, url, username, password, insecure, created_at, updated_at)
		VALUES (?, ?, '', '', 0, ?, ?)
	`, entry.Name, entry.URL, now, now)
	return err
}

// --- Retention Policy CRUD ---

// GetRetentionPolicy retrieves the retention policy for a registry
func (db *DB) GetRetentionPolicy(registryID int64) (*models.RetentionPolicy, error) {
	var p models.RetentionPolicy
	var dryRun int
	var lastRunAt sql.NullTime

	// Ensure we scan all new fields. Use simple query.
	// Note: if columns were just added, they are NULL or default.
	// But Scan might fail if we select columns that don't exist? No, migration runs on init.

	err := db.conn.QueryRow(`
		SELECT id, registry_id, keep_last_count, keep_days, dry_run, last_run_at,
		       COALESCE(filter_repos, ''), COALESCE(exclude_repos, ''), COALESCE(exclude_tags, '')
		FROM retention_policies WHERE registry_id = ?
	`, registryID).Scan(&p.ID, &p.RegistryID, &p.KeepLastCount, &p.KeepDays, &dryRun, &lastRunAt, &p.FilterRepos, &p.ExcludeRepos, &p.ExcludeTags)

	if err == sql.ErrNoRows {
		// Return default policy
		return &models.RetentionPolicy{
			RegistryID:    registryID,
			KeepLastCount: 5, // Default safe keep
			KeepDays:      0,
			DryRun:        true,
			ExcludeTags:   `^latest$|^main$|^master$`, // Default safe exclude (exact match)
		}, nil
	}
	if err != nil {
		return nil, err
	}

	p.DryRun = dryRun == 1
	if lastRunAt.Valid {
		p.LastRunAt = lastRunAt.Time
	}
	return &p, nil
}

// SaveRetentionPolicy saves or updates a retention policy
func (db *DB) SaveRetentionPolicy(p *models.RetentionPolicy) error {
	dryRun := 0
	if p.DryRun {
		dryRun = 1
	}

	// Upsert policy
	_, err := db.conn.Exec(`
		INSERT INTO retention_policies (registry_id, keep_last_count, keep_days, dry_run, filter_repos, exclude_repos, exclude_tags)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(registry_id) DO UPDATE SET
			keep_last_count = excluded.keep_last_count,
			keep_days = excluded.keep_days,
			dry_run = excluded.dry_run,
			filter_repos = excluded.filter_repos,
			exclude_repos = excluded.exclude_repos,
			exclude_tags = excluded.exclude_tags
	`, p.RegistryID, p.KeepLastCount, p.KeepDays, dryRun, p.FilterRepos, p.ExcludeRepos, p.ExcludeTags)

	return err
}

// UpdateRetentionLastRun updates the last run timestamp
func (db *DB) UpdateRetentionLastRun(registryID int64) error {
	_, err := db.conn.Exec(`
		UPDATE retention_policies SET last_run_at = CURRENT_TIMESTAMP WHERE registry_id = ?
	`, registryID)
	return err
}
