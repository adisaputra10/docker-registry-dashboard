# Docker Registry Dashboard Manager

A modern, feature-rich dashboard for managing your Docker Registry V2.
Ideally suited for private registry management, providing a user-friendly interface for browsing images, managing storage, and automating cleanup.

## 🚀 Key Features

*   **Registry Management**: Add/Remove multiple registries (Local, Remote).
*   **Image Browser**: browse repositories, tags, manifest details, and layers.
*   **Storage Configuration**: easy setup for Filesystem, S3, or SFTP backend storage.
*   **Docker Integration**: embedded Docker Registry V2 management (Start/Stop/Restart).
*   **Image Retention & Cleanup**: Automated policies to keep your registry clean.
    *   **Keep Last N Images**: Retain only the most recent builds.
    *   **Keep Time Window**: Retain images within a specific timeframe (e.g. 7 days).
    *   **Whitelist Protection**: Regex-based protection for important tags (e.g. `latest`, `stable`).
    *   **Shared Digest Safety**: Smart deletion prevents removing images used by protected tags.
    *   **Dry Run**: Simulate cleanup actions before permanent deletion.

## 🛠️ Installation & Usage

### Prerequisites
*   Docker installed and running.
*   Go 1.22+ (if building from source).

### Running Pre-built Binary
Assuming you have `registry-dashboard.exe`:
```bash
./registry-dashboard.exe -port 8080
```
Access the dashboard at `http://localhost:8080`.

### Building from Source
```bash
go mod tidy
go build -o registry-dashboard.exe .
./registry-dashboard.exe
```

## ⚙️ Configuration

The application stores its configuration in:
*   **Database**: `data/registry.db` (SQLite) - Stores registry list, retention policies.
*   **Registry Config**: `registry-config/config.yml` - Generated config for the embedded registry.

## 🧹 Retention Policy Guide

The Retention feature allows you to define rules for automatic image cleanup.

### How it Works
1.  **Define Rules**: Set `Keep Last Count` or `Keep Days`.
2.  **Whitelist**: Add Regex patterns for tags you NEVER want to delete.
    *   Example: `^latest$|^main$` protects exact "latest" and "main" tags.
    *   Example: `v1\..*` protects all v1.x tags.
3.  **Filter Repos**: Optionally choose to process only specific repositories.
4.  **Run**: Execute "Run Cleanup Now".
    *   **Dry Run**: See what *would* happen (Recommended first).
    *   **Wet Run**: Uncheck Dry Run to actually delete manifests.

### Safety Mechanisms
*   **Fail-Safe**: By default, if no policy is set, nothing is deleted.
*   **Shared Digest Protection**: If a tag targeted for deletion shares the same image Digest as a protected tag (e.g. `latest`), the underlying image will **NOT** be deleted.

## ⚠️ Important Note on Disk Space
This dashboard deletes **Image Manifests** (references). To reclaim physical disk space, you must run the Docker Registry Garbage Collector.
Usually, this is a separate background process or command:
```bash
# Example for standard registry container
docker exec registry bin/registry garbage-collect /etc/docker/registry/config.yml
```

## 📝 License
MIT License.
