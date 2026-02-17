package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"docker-registry-dashboard/internal/database"
	"docker-registry-dashboard/internal/handlers"
	"docker-registry-dashboard/internal/registry"
	"docker-registry-dashboard/internal/tasks"
)

//go:embed web/*
var webFS embed.FS

func main() {
	port := flag.Int("port", 8080, "Dashboard web UI port")
	registryPort := flag.Int("registry-port", 5000, "Docker Registry V2 port")
	dbPath := flag.String("db", "", "Database file path")
	noRegistry := flag.Bool("no-registry", false, "Do not start embedded Docker Registry")
	flag.Parse()

	// Determine base directory
	baseDir, err := os.Getwd()
	if err != nil {
		baseDir = "."
	}

	if *dbPath == "" {
		*dbPath = filepath.Join(baseDir, "data", "registry.db")
	}

	log.Println("╔══════════════════════════════════════════════╗")
	log.Println("║   Docker Registry V2 Dashboard Manager      ║")
	log.Println("╚══════════════════════════════════════════════╝")

	// Initialize database
	db, err := database.New(*dbPath)
	if err != nil {
		log.Fatalf("❌ Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Printf("✅ Database initialized at %s", *dbPath)

	// Initialize embedded registry manager
	embeddedReg := registry.NewEmbeddedRegistry(baseDir, *registryPort)

	// Start embedded Docker Registry V2
	if !*noRegistry {
		startEmbeddedRegistry(db, embeddedReg)
	} else {
		log.Println("⏭️  Embedded registry disabled (--no-registry)")
	}

	// Initialize Handlers
	h := handlers.New(db, embeddedReg)

	// Initialize Scheduler
	sched := tasks.NewScheduler(db)
	sched.Start()
	defer sched.Stop()

	// Routes
	mux := http.NewServeMux()

	// Dashboard
	mux.HandleFunc("GET /api/dashboard/stats", h.GetDashboardStats)

	// Registry CRUD
	mux.HandleFunc("GET /api/registries", h.ListRegistries)
	mux.HandleFunc("POST /api/registries", h.CreateRegistry)
	mux.HandleFunc("PUT /api/registries/{id}", h.UpdateRegistry)    // Go 1.22 routing
	mux.HandleFunc("DELETE /api/registries/{id}", h.DeleteRegistry) // Go 1.22 routing
	mux.HandleFunc("POST /api/registries/{id}/test", h.TestRegistryConnection)

	// Repository & Tag
	mux.HandleFunc("GET /api/registries/{id}/repositories", h.ListRepositories)
	mux.HandleFunc("GET /api/registries/{id}/tags", h.ListTags)
	mux.HandleFunc("GET /api/registries/{id}/manifest", h.GetManifest)
	mux.HandleFunc("DELETE /api/registries/{id}/tag", h.DeleteTag)

	// Retention Policy
	mux.HandleFunc("GET /api/registries/{id}/retention", h.GetRetentionPolicy)
	mux.HandleFunc("POST /api/registries/{id}/retention", h.SaveRetentionPolicy)
	mux.HandleFunc("POST /api/registries/{id}/retention/run", h.RunRetention)

	// Vulnerability Scanning
	mux.HandleFunc("POST /api/scan/trigger", h.TriggerScan)
	mux.HandleFunc("GET /api/scan/result", h.GetScanResult)
	mux.HandleFunc("GET /api/scan/list", h.ListScans)
	mux.HandleFunc("GET /api/vulnerabilities/list", h.ListVulnerabilities)
	mux.HandleFunc("GET /api/registries/{id}/scan-policy", h.GetScanPolicy)
	mux.HandleFunc("POST /api/registries/{id}/scan-policy", h.SaveScanPolicy)

	// Storage config
	mux.HandleFunc("GET /api/storage", h.GetStorageConfig)
	mux.HandleFunc("POST /api/storage", h.SaveStorageConfig)
	mux.HandleFunc("POST /api/storage/test", h.TestStorageConnection)

	// Embedded registry management
	mux.HandleFunc("GET /api/registry/status", h.GetEmbeddedRegistryStatus)
	mux.HandleFunc("POST /api/registry/restart", h.RestartEmbeddedRegistry)
	mux.HandleFunc("POST /api/registry/stop", h.StopEmbeddedRegistry)
	mux.HandleFunc("POST /api/registry/start", h.StartEmbeddedRegistry)
	mux.HandleFunc("GET /api/registry/logs", h.GetEmbeddedRegistryLogs)

	// Serve embedded static files
	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("❌ Failed to setup web filesystem: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(webContent)))

	// Graceful shutdown
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: mux,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("\n🛑 Shutting down...")
		if !*noRegistry {
			log.Println("🐳 Stopping embedded registry...")
			embeddedReg.Stop()
		}
		srv.Shutdown(context.Background())
	}()

	log.Printf("🚀 Dashboard UI: http://localhost:%d", *port)
	if !*noRegistry {
		log.Printf("🐳 Registry V2:  http://localhost:%d", *registryPort)
	}
	log.Println("─────────────────────────────────────────────")

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("❌ Server error: %v", err)
	}
	log.Println("👋 Goodbye!")
}

// startEmbeddedRegistry starts the Docker Registry V2 container and auto-registers it
func startEmbeddedRegistry(db *database.DB, reg *registry.EmbeddedRegistry) {
	if !reg.IsDockerAvailable() {
		log.Println("⚠️  Docker not available. Embedded registry will not start.")
		log.Println("   Install Docker Desktop or start Docker daemon to use this feature.")
		return
	}

	// Load storage config from database
	storageConfig, err := db.GetStorageConfig()
	if err != nil {
		log.Printf("⚠️  Could not load storage config, using defaults: %v", err)
		storageConfig = nil
	}

	// Start the registry
	if err := reg.Start(storageConfig); err != nil {
		log.Printf("⚠️  Failed to start embedded registry: %v", err)
		log.Println("   You can still add external registries manually.")
		return
	}

	// Auto-register the local registry in the database
	autoRegisterLocalRegistry(db, reg)
}

// autoRegisterLocalRegistry ensures the local embedded registry is registered
func autoRegisterLocalRegistry(db *database.DB, reg *registry.EmbeddedRegistry) {
	registries, err := db.ListRegistries()
	if err != nil {
		log.Printf("⚠️  Could not check existing registries: %v", err)
		return
	}

	registryURL := reg.URL()

	// Check if already registered
	for _, r := range registries {
		if r.URL == registryURL {
			log.Printf("📌 Local registry already registered (ID: %d)", r.ID)
			return
		}
	}

	// Register the local registry
	localReg := &database.RegistryEntry{
		Name: "Local Registry",
		URL:  registryURL,
	}
	if err := db.CreateRegistryEntry(localReg); err != nil {
		log.Printf("⚠️  Could not auto-register local registry: %v", err)
		return
	}
	log.Printf("📌 Local registry auto-registered at %s", registryURL)
}
