package scanner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// Summary counts per severity
type SeveritySummary struct {
	Critical int `json:"Critical"`
	High     int `json:"High"`
	Medium   int `json:"Medium"`
	Low      int `json:"Low"`
	Unknown  int `json:"Unknown"`
}

// TrivyVulnerability represents a single vulnerability in Trivy output
type TrivyVulnerability struct {
	VulnerabilityID  string `json:"VulnerabilityID"`
	PkgName          string `json:"PkgName"`
	InstalledVersion string `json:"InstalledVersion"`
	FixedVersion     string `json:"FixedVersion"`
	Severity         string `json:"Severity"`
	Title            string `json:"Title"`
	Description      string `json:"Description"`
}

// TrivyResult matches minimal structure of Trivy JSON output
type TrivyResult struct {
	Target          string               `json:"Target"`
	Vulnerabilities []TrivyVulnerability `json:"Vulnerabilities"`
}

type TrivyReport struct {
	Results []TrivyResult `json:"Results"`
}

// ScanImage runs trivy scan against a target image
func ScanImage(registryURL, repo, tag string) (string, string, error) {
	// Prepare target URL
	// Replace localhost with host.docker.internal for Docker-in-Docker networking on Windows/Mac
	// Assuming registryURL is like "http://localhost:5000"
	targetURL := registryURL
	if strings.Contains(targetURL, "localhost") || strings.Contains(targetURL, "127.0.0.1") {
		targetURL = strings.Replace(targetURL, "localhost", "host.docker.internal", 1)
		targetURL = strings.Replace(targetURL, "127.0.0.1", "host.docker.internal", 1)
	}

	// Remove protocol for docker image ref
	targetURL = strings.TrimPrefix(targetURL, "http://")
	targetURL = strings.TrimPrefix(targetURL, "https://")

	imageRef := fmt.Sprintf("%s/%s:%s", targetURL, repo, tag)

	log.Printf("🔍 Scanning image: %s (via trivy)", imageRef)

	// Command: docker run --rm aquasec/trivy image --format json --insecure --scanners vuln <image>
	cmd := exec.Command("docker", "run", "--rm",
		"aquasec/trivy", "image",
		"--format", "json",
		"--scanners", "vuln",
		"--insecure", // Allow insecure registry
		"--no-progress",
		imageRef,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("trivy execution failed: %v, stderr: %s", err, stderr.String())
	}

	jsonOutput := stdout.String()
	log.Printf("✅ Trivy scan completed. Output length: %d bytes", len(jsonOutput))

	// Parse summary
	summary, err := parseSummary(jsonOutput)
	if err != nil {
		// If parsing fails, maybe output isn't JSON or empty. Return raw anyway?
		log.Printf("⚠️ Failed to parse trivy output: %v", err)
	}

	log.Printf("📊 Summary: %s", summary)
	return jsonOutput, summary, nil
}

func parseSummary(jsonStr string) (string, error) {
	var report TrivyReport
	if err := json.Unmarshal([]byte(jsonStr), &report); err != nil {
		return "", err
	}

	sum := SeveritySummary{}
	for _, res := range report.Results {
		for _, v := range res.Vulnerabilities {
			switch v.Severity {
			case "CRITICAL":
				sum.Critical++
			case "HIGH":
				sum.High++
			case "MEDIUM":
				sum.Medium++
			case "LOW":
				sum.Low++
			default:
				sum.Unknown++
			}
		}
	}

	b, _ := json.Marshal(sum)
	return string(b), nil
}
