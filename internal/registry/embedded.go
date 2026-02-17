package registry

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"docker-registry-dashboard/internal/models"
)

const (
	ContainerName = "registry-v2-dashboard"
	DefaultPort   = 5000
)

var registryConfigTmpl = `version: 0.1
log:
  fields:
    service: registry
storage:
{{- if eq .Type "s3"}}
  s3:
    accesskey: "{{ .S3AccessKey }}"
    secretkey: "{{ .S3SecretKey }}"
    region: "{{ .S3Region }}"
    bucket: "{{ .S3Bucket }}"
{{- if .S3Endpoint }}
    regionendpoint: "http{{ if .S3UseSSL }}s{{ end }}://{{ .S3Endpoint }}"
{{- end }}
    secure: {{ .S3UseSSL }}
    rootdirectory: /
{{- else }}
  filesystem:
    rootdirectory: /var/lib/registry
{{- end }}
  delete:
    enabled: true
  maintenance:
    uploadpurging:
      enabled: true
      age: 168h
      interval: 24h
      dryrun: false
http:
  addr: :5000
  headers:
    X-Content-Type-Options: [nosniff]
    Access-Control-Allow-Origin: ['*']
    Access-Control-Allow-Methods: ['HEAD', 'GET', 'OPTIONS', 'DELETE']
    Access-Control-Allow-Headers: ['Authorization', 'Accept', 'Cache-Control']
    Access-Control-Expose-Headers: ['Docker-Content-Digest']
`

// EmbeddedRegistry manages a Docker Registry V2 container
type EmbeddedRegistry struct {
	mu        sync.Mutex
	baseDir   string
	port      int
	configDir string
	dataDir   string
}

// NewEmbeddedRegistry creates a new embedded registry manager
func NewEmbeddedRegistry(baseDir string, port int) *EmbeddedRegistry {
	if port == 0 {
		port = DefaultPort
	}
	return &EmbeddedRegistry{
		baseDir:   baseDir,
		port:      port,
		configDir: filepath.Join(baseDir, "registry-config"),
		dataDir:   filepath.Join(baseDir, "registry-data"),
	}
}

// Port returns the registry port
func (r *EmbeddedRegistry) Port() int {
	return r.port
}

// URL returns the registry URL
func (r *EmbeddedRegistry) URL() string {
	return fmt.Sprintf("http://localhost:%d", r.port)
}

// IsDockerAvailable checks if Docker CLI is available
func (r *EmbeddedRegistry) IsDockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// IsRunning checks if the registry container is running
func (r *EmbeddedRegistry) IsRunning() bool {
	out, err := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", ContainerName).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// generateConfig writes the registry config.yml based on storage settings
func (r *EmbeddedRegistry) generateConfig(config *models.StorageConfig) error {
	if err := os.MkdirAll(r.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	if config == nil {
		config = &models.StorageConfig{Type: "local"}
	}
	if config.Type == "" {
		config.Type = "local"
	}

	tmpl, err := template.New("registry-config").Parse(registryConfigTmpl)
	if err != nil {
		return fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return fmt.Errorf("template exec error: %w", err)
	}

	configPath := filepath.Join(r.configDir, "config.yml")
	if err := os.WriteFile(configPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	log.Printf("📝 Registry config written to %s", configPath)
	return nil
}

// stopContainer removes the existing container
func (r *EmbeddedRegistry) stopContainer() {
	exec.Command("docker", "stop", ContainerName).Run()
	exec.Command("docker", "rm", "-f", ContainerName).Run()
}

// startLocked starts the registry (must hold mu)
func (r *EmbeddedRegistry) startLocked(config *models.StorageConfig) error {
	if !r.IsDockerAvailable() {
		return fmt.Errorf("Docker is not available. Please install and start Docker Desktop")
	}

	// Default to local if nil
	if config == nil {
		config = &models.StorageConfig{Type: "local", LocalPath: "/var/lib/registry"}
	}
	if config.Type == "" {
		config.Type = "local"
	}

	// Generate config
	if err := r.generateConfig(config); err != nil {
		return err
	}

	// Ensure data dir exists
	if err := os.MkdirAll(r.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	// Stop existing container
	r.stopContainer()

	// Pull image if not present
	log.Println("📦 Ensuring registry:2 image is available...")
	pullCmd := exec.Command("docker", "pull", "registry:2")
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr
	pullCmd.Run() // Ignore error, image might already exist

	// Build absolute paths for volume mounts
	configAbs, _ := filepath.Abs(r.configDir)
	dataAbs, _ := filepath.Abs(r.dataDir)

	// Build docker run arguments
	args := []string{
		"run", "-d",
		"--name", ContainerName,
		"-p", fmt.Sprintf("%d:5000", r.port),
		"-v", fmt.Sprintf("%s:/etc/docker/registry", configAbs),
	}

	switch config.Type {
	case "local", "":
		// Mount local data directory
		localPath := dataAbs
		if config.LocalPath != "" && config.LocalPath != "/var/lib/registry" {
			absPath, err := filepath.Abs(config.LocalPath)
			if err == nil {
				localPath = absPath
			}
			os.MkdirAll(localPath, 0755)
		}
		args = append(args, "-v", fmt.Sprintf("%s:/var/lib/registry", localPath))

	case "s3":
		// S3 does not need volume mount, config handles it
		log.Println("☁️  Using S3/Object Storage backend")

	case "sftp":
		// For SFTP, we mount the data dir and note that sshfs should be configured on host
		args = append(args, "-v", fmt.Sprintf("%s:/var/lib/registry", dataAbs))
		log.Println("🔐 SFTP storage: mount your SFTP server to:", dataAbs)
		log.Println("   Example: sshfs user@host:/path", dataAbs)
	}

	args = append(args, "--restart", "unless-stopped", "registry:2")

	log.Printf("🐳 Starting Docker Registry V2 container...")
	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start registry container: %w\nOutput: %s", err, string(output))
	}

	// Wait for container to become running
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		if r.IsRunning() {
			log.Printf("✅ Docker Registry V2 running at http://localhost:%d", r.port)
			return nil
		}
	}

	// If still not running, check logs
	logCmd := exec.Command("docker", "logs", "--tail", "20", ContainerName)
	logOut, _ := logCmd.CombinedOutput()
	return fmt.Errorf("registry container did not become healthy.\nLogs:\n%s", string(logOut))
}

// Start starts the registry container with the given storage config
func (r *EmbeddedRegistry) Start(config *models.StorageConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.startLocked(config)
}

// Stop stops the registry container
func (r *EmbeddedRegistry) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopContainer()
	log.Println("🛑 Docker Registry V2 stopped")
	return nil
}

// Restart stops and restarts the registry with new config
func (r *EmbeddedRegistry) Restart(config *models.StorageConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	log.Println("🔄 Restarting Docker Registry V2 with new configuration...")
	return r.startLocked(config)
}

// Status returns the current registry status
func (r *EmbeddedRegistry) Status() map[string]interface{} {
	running := r.IsRunning()
	status := map[string]interface{}{
		"running":          running,
		"container_name":   ContainerName,
		"port":             r.port,
		"url":              r.URL(),
		"docker_available": r.IsDockerAvailable(),
	}

	if running {
		out, err := exec.Command("docker", "inspect", "-f",
			"{{.State.Status}}|{{.State.StartedAt}}|{{.Image}}", ContainerName).Output()
		if err == nil {
			parts := strings.SplitN(strings.TrimSpace(string(out)), "|", 3)
			if len(parts) >= 1 {
				status["state"] = parts[0]
			}
			if len(parts) >= 2 {
				status["started_at"] = parts[1]
			}
			if len(parts) >= 3 {
				status["image"] = parts[2][:12] // Truncate image hash
			}
		}
	}

	return status
}

// GetContainerLogs returns the last N lines of container logs
func (r *EmbeddedRegistry) GetContainerLogs(lines int) (string, error) {
	if lines <= 0 {
		lines = 50
	}
	cmd := exec.Command("docker", "logs", "--tail", fmt.Sprintf("%d", lines), ContainerName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %w", err)
	}
	return string(out), nil
}
