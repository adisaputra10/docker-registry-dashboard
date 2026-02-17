# 🚀 Docker Registry Dashboard - Detailed Interface Guide

This document provides a comprehensive walkthrough of the management interface, as captured in the accompanying screenshots. The dashboard is designed to provide full visibility and control over your container ecosystem.

---

## 📂 1. Image Repository Management (`image1.png`)
The **Images Browser** serves as the central hub for exploring your registry contents.

- **Registry Selection**: A dynamic dropdown allows you to switch between multiple connected registries (e.g., Local, Production, Staging).
- **Global Search**: The header search bar provides instant filtering across repositories and tags.
- **Repository List**: Displays a clean, card-based layout for each repository (e.g., `nginx`, `nodejs`).
  - **Tag Counts**: Quickly see how many versions/tags are currently stored.
  - **Navigation**: Click any repository to drill down into specific tags and manifest details.

## 🧹 2. Intelligent Retention Policies (`image2.png`)
Automate the cleanup of old or unused images to save storage costs and maintain registry hygiene.

- **Cleanup Logic**:
  - **Keep Last N**: Ensures you always have the most recent versions (e.g., "Always keep the last 5 builds").
  - **Age-Based Scoped**: Automatically marks images for deletion if they are older than a specific number of days.
- **Advanced Filtering**: (Defined in policy settings)
  - **Regex Whitelisting**: Use patterns like `^v[0-9]+\..*` or `^latest$` to protect critical release tags from being deleted.
  - **Repository Scoping**: Apply cleanup logic only to specific repository groups.
- **Execution Status**: Real-time tracking of the "Last Run" timestamp to ensure your policies are active.

## 🛡️ 3. Vulnerability Scanning Results (`image3.png`)
Deep security analysis for individual image tags using industry-standard scanners.

- **Dual-Scanner Integration**: View results from both **Trivy** (system-level) and **OSV** (application-level/SBOM) in a unified tabbed view.
- **Discovery Cards**: Each vulnerability is grouped by the affected package (e.g., `brace-expansion`, `cross-spawn`).
- **Detailed Metadata**:
  - **Severity Badges**: Color-coded (Red for High, Yellow for Medium, Green for Low) for quick prioritization.
  - **Source Tracking**: Identification of the specific SBOM file or layer where the vulnerability was found.
  - **Direct Links**: Clickable IDs (e.g., `GHSA-...`) that link directly to the official security advisory databases.

## 📊 4. Global Security Insights (`image4.png`)
A high-level dashboard for security officers to assess the health of the entire registry.

- **Aggregated Summary**: Big, bold counters showing the total number of findings categorized by severity across the entire registry.
- **Multidimensional Filters**:
  - Filter by **Repository** or **Tag** to find "hotspots".
  - Filter by **Scanner** to compare Trivy vs. OSV findings.
  - Search by **CVE ID** to find if a specific vulnerability exists anywhere in your infrastructure.
- **Consolidated Table**: A comprehensive list showing Package, Version, and established **Fix Versions** to help developers patch quickly.

## ⚙️ 5. Storage & Registry Control (`image5.png`)
Manage the underlying infrastructure and the lifecycle of the embedded registry.

- **Lifecycle Management**: Dedicated buttons to **Start**, **Stop**, or **Restart** the Docker Registry V2 instance without leaving the dashboard.
- **Real-time Logs**: One-click access to the internal registry logs for debugging connection or push/pull issues.
- **Flexible Storage Backends**:
  - **Local**: Manage the bind-mount path for containerized storage.
  - **Cloud Ready**: Configure S3-compatible storage (AWS, Minio) with SSL support.
  - **Enterprise Sync**: Support for SFTP-based storage backends.
- **Operational Safety**: A "Test" button allows you to verify storage credentials before applying changes, preventing registry downtime.

---
*Created and documented by Antigravity AI — Empowering secure and efficient container management.*
