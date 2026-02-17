package registry

import (
	"docker-registry-dashboard/internal/models"
	"fmt"
	"log"
	"regexp"
	"sort"
	"sync"
	"time"
)

// RunRetention executes the retention policy for a registry
func RunRetention(reg *models.Registry, policy *models.RetentionPolicy) ([]models.RetentionLog, error) {
	client := NewClientFromRegistry(reg)
	repos, err := client.ListRepositories()
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	var logs []models.RetentionLog
	var mu sync.Mutex

	// Compile regexes
	var filterRepoRe, excludeRepoRe *regexp.Regexp
	if policy.FilterRepos != "" {
		filterRepoRe, err = regexp.Compile(policy.FilterRepos)
		if err != nil {
			log.Printf("⚠️ Invalid FilterRepos regex: %v", err)
		}
	}
	if policy.ExcludeRepos != "" {
		excludeRepoRe, err = regexp.Compile(policy.ExcludeRepos)
		if err != nil {
			log.Printf("⚠️ Invalid ExcludeRepos regex: %v", err)
		}
	}

	// Process each repository
	for _, repo := range repos {
		// Repo Filtering
		if filterRepoRe != nil && !filterRepoRe.MatchString(repo.Name) {
			continue // Skip not matching
		}
		if excludeRepoRe != nil && excludeRepoRe.MatchString(repo.Name) {
			continue // Skip excluded
		}

		repoLogs, err := processRepository(client, repo.Name, policy)
		if err != nil {
			log.Printf("⚠️ Error processing repo %s: %v", repo.Name, err)
			continue
		}
		mu.Lock()
		logs = append(logs, repoLogs...)
		mu.Unlock()
	}

	return logs, nil
}

type imageInfo struct {
	Tag       string
	Digest    string
	Created   time.Time
	Protected bool
}

func processRepository(client *Client, repoName string, policy *models.RetentionPolicy) ([]models.RetentionLog, error) {
	tags, err := client.ListTags(repoName)
	if err != nil {
		return nil, err
	}

	if len(tags) == 0 {
		return nil, nil
	}

	// Compile tag whitelist regex
	var excludeTagRe *regexp.Regexp
	if policy.ExcludeTags != "" {
		excludeTagRe, err = regexp.Compile(policy.ExcludeTags)
		if err != nil {
			log.Printf("⚠️ Invalid ExcludeTags regex: %v", err)
		}
	}

	// Fetch details concurrently
	var images []imageInfo
	var mu sync.Mutex
	var wg sync.WaitGroup
	// Concurrency limit to avoid overwhelming the registry
	sem := make(chan struct{}, 5)

	for _, tag := range tags {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Check whitelist first? No, we need Created time for sorting anyway.
			// But can mark as protected.
			isProtected := false
			if excludeTagRe != nil && excludeTagRe.MatchString(t) {
				isProtected = true
			}

			// If protected, do we still fetch created time?
			// Yes, for correct sorting (KeepLastCount logic).

			created, err := client.GetImageCreated(repoName, t)
			if err != nil {
				// Fallback: try to guess or just skip?
				// Logging error and skipping is safer than deleting wrongly.
				// log.Printf("⚠️ Failed to get info for %s:%s: %v", repoName, t, err)
				return
			}

			digest, err := client.GetDigestForTag(repoName, t)
			if err != nil {
				return
			}

			mu.Lock()
			images = append(images, imageInfo{Tag: t, Digest: digest, Created: created, Protected: isProtected})
			mu.Unlock()
		}(tag.Name)
	}
	wg.Wait()

	// Sort by Created DESC (newest first)
	sort.Slice(images, func(i, j int) bool {
		return images[i].Created.After(images[j].Created)
	})

	var logs []models.RetentionLog
	now := time.Now()

	// Track kept digests to prevent deleting shared manifests
	keptDigests := make(map[string]bool)

	type tagDecision struct {
		img    imageInfo
		keep   bool
		reason string
	}
	decisions := make([]tagDecision, 0, len(images))

	// Pass 1: Evaluate Rules
	for i, img := range images {
		shouldKeep := false
		reason := "default keep"

		// Rule 1: Keep Last Count
		if policy.KeepLastCount > 0 {
			if i < policy.KeepLastCount {
				shouldKeep = true
				reason = fmt.Sprintf("within last %d images", policy.KeepLastCount)
			}
		}

		// Rule 2: Keep Days
		if policy.KeepDays > 0 {
			age := now.Sub(img.Created)
			days := int(age.Hours() / 24)
			if days < policy.KeepDays {
				if shouldKeep {
					reason += fmt.Sprintf(" AND newer than %d days", policy.KeepDays)
				} else {
					shouldKeep = true
					reason = fmt.Sprintf("newer than %d days", policy.KeepDays)
				}
			}
		}

		// Rule 3: Whitelist (Override)
		if img.Protected {
			shouldKeep = true
			if reason == "default keep" { // Don't overwrite if already kept by other rules
				reason = "matches whitelist tag"
			} else {
				reason += " AND matches whitelist"
			}
		}

		// Safety: if no policy set, keep everything
		if policy.KeepLastCount <= 0 && policy.KeepDays <= 0 {
			shouldKeep = true
			reason = "no policy set"
		}

		if shouldKeep {
			keptDigests[img.Digest] = true
		}
		decisions = append(decisions, tagDecision{img, shouldKeep, reason})
	}

	// Pass 2: Execute actions
	for _, d := range decisions {
		action := "kept"
		reason := d.reason

		if !d.keep {
			reason = "exceeds retention limits"

			// Critical Safety: Check if digest is used by another KEPT tag
			if keptDigests[d.img.Digest] {
				action = "kept"
				reason = "digest shared with retained tag"
			} else {
				if policy.DryRun {
					action = "would_delete"
				} else {
					if err := client.DeleteManifest(repoName, d.img.Digest); err != nil {
						action = "error_delete"
						reason = fmt.Sprintf("failed to delete: %v", err)
					} else {
						action = "deleted"
					}
				}
			}
		}

		logs = append(logs, models.RetentionLog{
			Repository: repoName,
			Tag:        d.img.Tag,
			Digest:     d.img.Digest,
			Created:    d.img.Created,
			Action:     action,
			Reason:     reason,
		})
	}

	return logs, nil
}
