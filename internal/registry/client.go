package registry

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"docker-registry-dashboard/internal/models"
)

// Client communicates with Docker Registry V2 API
type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

// NewClient creates a new Registry V2 API client
func NewClient(url, username, password string, insecure bool) *Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecure,
		},
	}

	return &Client{
		baseURL:  url,
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout:   15 * time.Second,
			Transport: transport,
		},
	}
}

// NewClientFromRegistry creates a client from a Registry model
func NewClientFromRegistry(r *models.Registry) *Client {
	return NewClient(r.URL, r.Username, r.Password, r.Insecure)
}

func (c *Client) doRequest(method, path string, headers map[string]string) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, path)
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.httpClient.Do(req)
}

// Ping checks if the registry is accessible (GET /v2/)
func (c *Client) Ping() error {
	resp, err := c.doRequest("GET", "/v2/", nil)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication failed (401)")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}

// catalogResponse represents the /v2/_catalog response
type catalogResponse struct {
	Repositories []string `json:"repositories"`
}

// ListRepositories returns all repositories in the registry
func (c *Client) ListRepositories() ([]models.Repository, error) {
	var allRepos []models.Repository
	nextURL := "/v2/_catalog?n=100"

	for nextURL != "" {
		// Ensure URL is relative to base if it's full
		if strings.HasPrefix(nextURL, c.baseURL) {
			nextURL = strings.TrimPrefix(nextURL, c.baseURL)
		}

		resp, err := c.doRequest("GET", nextURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories: %w", err)
		}

		// Decode response
		var catalog catalogResponse
		if resp.StatusCode == http.StatusOK {
			if err := json.NewDecoder(resp.Body).Decode(&catalog); err != nil {
				resp.Body.Close()
				return nil, fmt.Errorf("failed to decode catalog: %w", err)
			}
			for _, name := range catalog.Repositories {
				allRepos = append(allRepos, models.Repository{Name: name})
			}
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(body))
		}

		// check Link header
		link := resp.Header.Get("Link")
		resp.Body.Close() // Close body immediately after reading

		nextURL = ""
		if link != "" {
			// Parse Link header: <url>; rel="next"
			parts := strings.Split(link, ";")
			if len(parts) >= 2 && strings.Contains(parts[1], `rel="next"`) {
				url := strings.Trim(parts[0], " <>")
				nextURL = url
			}
		}
	}

	return allRepos, nil
}

// tagsResponse represents the /v2/<name>/tags/list response
type tagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// ListTags returns all tags for a repository
func (c *Client) ListTags(repoName string) ([]models.Tag, error) {
	path := fmt.Sprintf("/v2/%s/tags/list", repoName)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(body))
	}

	var tagsResp tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("failed to decode tags: %w", err)
	}

	tags := make([]models.Tag, len(tagsResp.Tags))
	for i, name := range tagsResp.Tags {
		tags[i] = models.Tag{Name: name}
	}
	return tags, nil
}

// GetManifest returns the manifest for a specific tag
func (c *Client) GetManifest(repoName, tag string) (*models.ImageManifest, error) {
	path := fmt.Sprintf("/v2/%s/manifests/%s", repoName, tag)
	headers := map[string]string{
		"Accept": "application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json",
	}

	resp, err := c.doRequest("GET", path, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest body: %w", err)
	}

	// Parse manifest
	var rawManifest struct {
		SchemaVersion int    `json:"schemaVersion"`
		MediaType     string `json:"mediaType"`
		Config        struct {
			MediaType string `json:"mediaType"`
			Size      int64  `json:"size"`
			Digest    string `json:"digest"`
		} `json:"config"`
		Layers []struct {
			MediaType string `json:"mediaType"`
			Size      int64  `json:"size"`
			Digest    string `json:"digest"`
		} `json:"layers"`
	}

	if err := json.Unmarshal(body, &rawManifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	manifest := &models.ImageManifest{
		SchemaVersion: rawManifest.SchemaVersion,
		MediaType:     rawManifest.MediaType,
		Digest:        resp.Header.Get("Docker-Content-Digest"),
	}

	if rawManifest.Config.Digest != "" {
		manifest.Config = &models.ManifestConfig{
			MediaType: rawManifest.Config.MediaType,
			Size:      rawManifest.Config.Size,
			Digest:    rawManifest.Config.Digest,
		}
	}

	var totalSize int64
	for _, layer := range rawManifest.Layers {
		manifest.Layers = append(manifest.Layers, models.ManifestLayer{
			MediaType: layer.MediaType,
			Size:      layer.Size,
			Digest:    layer.Digest,
		})
		totalSize += layer.Size
	}
	manifest.TotalSize = totalSize

	return manifest, nil
}

// DeleteManifest deletes a manifest by digest
func (c *Client) DeleteManifest(repoName, digest string) error {
	path := fmt.Sprintf("/v2/%s/manifests/%s", repoName, digest)
	resp, err := c.doRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("failed to delete manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetDigestForTag returns the digest for a specific tag
func (c *Client) GetDigestForTag(repoName, tag string) (string, error) {
	path := fmt.Sprintf("/v2/%s/manifests/%s", repoName, tag)
	headers := map[string]string{
		"Accept": "application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json",
	}

	resp, err := c.doRequest("HEAD", path, headers)
	if err != nil {
		return "", fmt.Errorf("failed to get digest: %w", err)
	}
	defer resp.Body.Close()

	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return "", fmt.Errorf("no digest returned for tag %s", tag)
	}
	return digest, nil
}

// GetImageCreated returns the creation time of an image tag
func (c *Client) GetImageCreated(repoName, tag string) (time.Time, error) {
	manifest, err := c.GetManifest(repoName, tag)
	if err != nil {
		return time.Time{}, err
	}

	if manifest.Config == nil || manifest.Config.Digest == "" {
		return time.Time{}, fmt.Errorf("manifest config digest missing")
	}

	// Fetch config blob
	path := fmt.Sprintf("/v2/%s/blobs/%s", repoName, manifest.Config.Digest)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to fetch config blob: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("blob fetch failed with status %d", resp.StatusCode)
	}

	var config struct {
		Created time.Time `json:"created"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return time.Time{}, fmt.Errorf("failed to decode image config: %w", err)
	}

	return config.Created, nil
}
