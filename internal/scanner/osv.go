package scanner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
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
}

// ScanImageOSV pulls and scans an image using osv-scanner docker container
func ScanImageOSV(registryURL, repo, tag string) (string, string, error) {
	// 1. Determine Image Ref
	targetURL := registryURL
	if strings.Contains(targetURL, "localhost") || strings.Contains(targetURL, "127.0.0.1") {
		targetURL = strings.Replace(targetURL, "localhost", "host.docker.internal", 1)
		targetURL = strings.Replace(targetURL, "127.0.0.1", "host.docker.internal", 1)
	}
	targetURL = strings.TrimPrefix(targetURL, "http://")
	targetURL = strings.TrimPrefix(targetURL, "https://")

	imageRef := fmt.Sprintf("%s/%s:%s", targetURL, repo, tag)

	log.Printf("📥 [OSV] Preparing scan for: %s", imageRef)

	// 2. Scan with OSV-Scanner directly on Docker image
	log.Printf("🔍 [OSV] Scanning image with OSV-Scanner...")

	cmd := exec.Command("docker", "run", "--rm",
		"-v", "/var/run/docker.sock:/var/run/docker.sock", // Mount docker socket
		"ghcr.io/google/osv-scanner:v1.5.0",
		"--docker", imageRef, // Use --docker for container image scanning
		"--json",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	// OSV-Scanner might return non-zero on vulns found, check output content
	log.Printf("🔍 [OSV] OSV-Scanner exit status: err=%v, stdout len=%d, stderr len=%d", err, stdout.Len(), stderr.Len())

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
			for range pkg.Vulnerabilities {
				// Count logic could be improved if OSV severity structure is known
				sum.Unknown++
			}
		}
	}

	b, _ := json.Marshal(sum)
	return string(b), nil
}
