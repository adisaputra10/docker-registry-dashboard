package tasks

import (
	"fmt"
	"log"
	"regexp"
	"sync"
	"time"

	"docker-registry-dashboard/internal/database"
	"docker-registry-dashboard/internal/models"
	"docker-registry-dashboard/internal/registry"
	"docker-registry-dashboard/internal/scanner"
)

type ScanJob struct {
	RegistryURL string
	RegistryID  int64
	Repo        string
	Tag         string
}

type Scheduler struct {
	db      *database.DB
	jobChan chan ScanJob
	quit    chan struct{}
	wg      sync.WaitGroup
}

func NewScheduler(db *database.DB) *Scheduler {
	return &Scheduler{
		db:      db,
		jobChan: make(chan ScanJob, 100), // Buffer 100 jobs
		quit:    make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	// Start 2 Workers
	for i := 0; i < 2; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}

	// Start Ticker
	go s.runTicker()
}

func (s *Scheduler) Stop() {
	close(s.quit)
	close(s.jobChan)
	s.wg.Wait()
}

func (s *Scheduler) runTicker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkSchedules()
		case <-s.quit:
			return
		}
	}
}

// checkSchedules checks DB for due policies
func (s *Scheduler) checkSchedules() {
	policies, err := s.db.ListEnabledScanPolicies()
	if err != nil {
		log.Println("Scheduler DB Error:", err)
		return
	}

	now := time.Now()
	for _, p := range policies {
		// If NextRunAt is zero (first time) or passed
		if p.NextRunAt.IsZero() || now.After(p.NextRunAt) {
			log.Printf("⏰ Triggering scheduled scan for registry %d", p.RegistryID)

			// Update Next Run immediately to prevent double trigger
			interval := p.IntervalHours
			if interval < 1 {
				interval = 24
			}
			next := now.Add(time.Duration(interval) * time.Hour)
			s.db.UpdatePolicyRunTime(p.ID, now, next)

			go s.triggerPolicy(p)
		}
	}
}

func (s *Scheduler) triggerPolicy(p models.ScanPolicy) {
	reg, err := s.db.GetRegistry(p.RegistryID)
	if err != nil {
		log.Printf("❌ Scheduler: Registry %d not found", p.RegistryID)
		return
	}

	client := registry.NewClient(reg.URL, reg.Username, reg.Password, reg.Insecure)
	repos, err := client.ListRepositories()
	if err != nil {
		log.Printf("❌ Scheduler: Failed to list repos for registry %d: %v", p.RegistryID, err)
		return
	}

	var filterRe *regexp.Regexp
	if p.FilterRepos != "" {
		filterRe, err = regexp.Compile(p.FilterRepos)
		if err != nil {
			log.Printf("⚠️ Scheduler: Invalid filter regex for policy %d: %v", p.ID, err)
			return
		}
	}

	count := 0
	for _, repo := range repos {
		repoName := repo.Name
		if filterRe != nil && !filterRe.MatchString(repoName) {
			continue
		}

		tags, err := client.ListTags(repoName)
		if err != nil {
			continue
		}

		for _, tag := range tags {
			// Queue Job
			select {
			case s.jobChan <- ScanJob{
				RegistryURL: reg.URL,
				RegistryID:  reg.ID,
				Repo:        repoName,
				Tag:         tag.Name,
			}:
				count++
			case <-time.After(2 * time.Second):
				log.Printf("⚠️ Scheduler job queue full, skipping %s:%s", repoName, tag.Name)
			}
		}
	}
	log.Printf("✅ Scheduler queued %d images for registry %d", count, p.RegistryID)
}

func (s *Scheduler) worker(id int) {
	defer s.wg.Done()
	log.Printf("👷 Scan Worker %d started", id)
	for job := range s.jobChan {
		// Create DB record (status: scanning)
		scan := &models.VulnerabilityScan{
			RegistryID: job.RegistryID,
			Repository: job.Repo,
			Tag:        job.Tag,
			Status:     "scanning",
			ScannedAt:  time.Now(),
		}

		if err := s.db.SaveScan(scan); err != nil {
			log.Printf("Worker DB Error: %v", err)
			continue
		}

		// Run Scan
		// Pass credentials if needed (currently not supported by scanner func, assumes no auth/public)
		// But in scheduler we have registry object access in triggerPolicy.
		// job struct only has URL.
		// Future improvement: Pass auth.

		report, summary, err := scanner.ScanImage(job.RegistryURL, job.Repo, job.Tag)
		if err != nil {
			scan.Status = "failed"
			scan.Report = fmt.Sprintf(`{"error": "%s"}`, err.Error())
		} else {
			scan.Status = "completed"
			scan.Report = report
			scan.Summary = summary
		}
		scan.ScannedAt = time.Now()

		if err := s.db.SaveScan(scan); err != nil {
			log.Printf("Worker DB Error saving result: %v", err)
		}
	}
}
