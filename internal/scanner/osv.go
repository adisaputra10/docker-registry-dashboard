package scanner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// OSVOutput matches top level OSV JSON
type OSVOutput struct {
	Results []OSVResultItem `json:"results"`
}

type OSVResultItem struct {
	Packages []OSVPackageMatch `json:"packages"`
}

type OSVPackageMatch struct {
	Package         OSVPackageInfo     `json:"package"`
	Vulnerabilities []OSVVulnerability `json:"vulnerabilities"`
}

type OSVPackageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type OSVVulnerability struct {
	ID       string `json:"id"`
	Summary  string `json:"summary"`
	Severity []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	} `json:"severity"`
	DatabaseSpecific map[string]interface{} `json:"database_specific"`
}

// ScanImageOSV generates an SBOM using Trivy and scans it with OSV-Scanner
func ScanImageOSV(registryURL, repo, tag string) (string, string, error) {
	// 1. Determine Image Ref
	targetURL := registryURL
	// Replace localhost with host.docker.internal for Docker-in-Docker networking
	if strings.Contains(targetURL, "localhost") || strings.Contains(targetURL, "127.0.0.1") {
		targetURL = strings.Replace(targetURL, "localhost", "host.docker.internal", 1)
		targetURL = strings.Replace(targetURL, "127.0.0.1", "host.docker.internal", 1)
	}
	targetURL = strings.TrimPrefix(targetURL, "http://")
	targetURL = strings.TrimPrefix(targetURL, "https://")

	imageRef := fmt.Sprintf("%s/%s:%s", targetURL, repo, tag)
	log.Printf("📥 [OSV] Preparing scan for: %s", imageRef)

	// Ensure scan_temp dir exists
	tempDir := "scan_temp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create temp dir: %v", err)
	}

	// 2. Generate SBOM using Trivy
	// We need absolute path for volume mount
	absTempDir, err := filepath.Abs(tempDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to get absolute path for temp dir: %v", err)
	}

	cleanRepo := strings.ReplaceAll(repo, "/", "_")
	sbomFilename := fmt.Sprintf("sbom_%s_%s.json", cleanRepo, tag)
	// Local path relative to CWD for cleanup
	sbomPath := filepath.Join(tempDir, sbomFilename)
	// Container output path (mounted)
	containerSbomPath := fmt.Sprintf("/output/%s", sbomFilename)

	log.Printf("🔨 [OSV] Generating SBOM with Trivy: %s...", sbomFilename)

	// Create Trivy command to generate SBOM
	// docker run --rm -v "absTempDir":/output -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy image --format cyclonedx --output /output/sbom.json <image>
	trivyCmd := exec.Command("docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/output", absTempDir),
		"-v", "/var/run/docker.sock:/var/run/docker.sock", // Mount docker socket so trivy can find the image
		"aquasec/trivy", "image",
		"--format", "cyclonedx",
		"--output", containerSbomPath,
		"--scanners", "vuln", // Trivy still needs to know what to look at, though for SBOM 'image' is key
		"--insecure",
		"--no-progress",
		imageRef,
	)

	var trivyOut, trivyErr bytes.Buffer
	trivyCmd.Stdout = &trivyOut
	trivyCmd.Stderr = &trivyErr

	if err := trivyCmd.Run(); err != nil {
		log.Printf("⚠️ [OSV] Trivy SBOM generation failed. Stderr: %s", trivyErr.String())
		return "", "", fmt.Errorf("trivy sbom generation failed: %v", err)
	}
	log.Printf("✅ [OSV] SBOM generated successfully.")

	defer func() {
		// Clean up SBOM file
		if err := os.Remove(sbomPath); err != nil {
			log.Printf("⚠️ [OSV] Failed to remove temp file %s: %v", sbomPath, err)
		}
	}()

	// 3. Scan the SBOM with OSV-Scanner
	log.Printf("🔍 [OSV] Scanning SBOM with OSV-Scanner...")

	// docker run --rm -v "absTempDir":/output ghcr.io/google/osv-scanner --sbom /output/sbom.json --json
	cmd := exec.Command("docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/output", absTempDir),
		"ghcr.io/google/osv-scanner:v1.9.2",
		"--sbom", containerSbomPath,
		"--json",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	log.Printf("🔍 [OSV] OSV-Scanner exit status: err=%v, stdout len=%d, stderr len=%d", err, stdout.Len(), stderr.Len())

	if stderr.Len() > 0 {
		log.Printf("📝 [OSV] Stderr Output:\n%s", stderr.String())
	}

	if stdout.Len() == 0 {
		stderrMsg := stderr.String()
		log.Printf("⚠️ [OSV] Empty output from OSV-Scanner. Stderr: %s", stderrMsg)
		return "", "", fmt.Errorf("osv-scanner failed (empty output): %v, stderr: %s", err, stderrMsg)
	}

	jsonOutput := stdout.String()
	log.Printf("✅ [OSV] Output received, size: %d bytes", len(jsonOutput))
	summary, err := parseOSVSummary(jsonOutput)
	if err != nil {
		log.Printf("⚠️ [OSV] Summary parse error: %v", err)
	}

	return jsonOutput, summary, nil
}

func parseOSVSummary(jsonStr string) (string, error) {
	var out OSVOutput
	if err := json.Unmarshal([]byte(jsonStr), &out); err != nil {
		return "", err
	}

	sum := SeveritySummary{}

	for _, res := range out.Results {
		for _, pkg := range res.Packages {
			for _, v := range pkg.Vulnerabilities {
				// Try to get severity from database_specific
				sev := ""
				if v.DatabaseSpecific != nil {
					if s, ok := v.DatabaseSpecific["severity"].(string); ok {
						sev = s
					}
				}

				switch strings.ToUpper(sev) {
				case "CRITICAL":
					sum.Critical++
				case "HIGH":
					sum.High++
				case "MEDIUM", "MODERATE":
					sum.Medium++
				case "LOW":
					sum.Low++
				default:
					sum.Unknown++
				}
			}
		}
	}

	b, _ := json.Marshal(sum)
	return string(b), nil
}
