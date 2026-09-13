<div align="center">

# 🚀 dbshift

**Blazing Fast Cloud Database Migration & Replication Engine**

[![Go Version](https://img.shields.io/github/go-mod/go-version/dusmamud/dbshift?style=for-the-badge&logo=go&color=00ADD8)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=for-the-badge)](LICENSE)
[![CI Status](https://img.shields.io/badge/CI-Passing-brightgreen.svg?style=for-the-badge&logo=githubactions)](https://github.com/dusmamud/dbshift/actions)
[![Release](https://img.shields.io/badge/Release-v1.0.0-orange.svg?style=for-the-badge&logo=github)](https://github.com/dusmamud/dbshift/releases)
[![Status: Maintained](https://img.shields.io/badge/Status-Actively%20Maintained-success.svg?style=for-the-badge)]()

*Piping entire databases between cloud clusters with zero data loss, sub-second latency, and live terminal analytics.*

[Features](#-features) • [Tech Stack](#-tech-stack) • [Installation](#-installation) • [Quickstart](#-quickstart) • [Environment Variables](#-environment-variables) • [Testing](#-testing) • [Documentation](#-documentation) • [Roadmap](#-roadmap) • [Contributing](#-contributing) • [License](#-license)

</div>

---

## ⚡ Overview

`dbshift` is an open-source, production-grade CLI tool built in **Go** to replicate and migrate tables, schemas, indexes, and high-volume datasets across cloud databases.

Whether migrating between **CockroachDB Cloud clusters**, moving from **PostgreSQL to Supabase / Neon**, or transferring **MongoDB Atlas collections**, `dbshift` delivers maximum throughput through **goroutine worker pools** and native binary streaming protocols (`pgx/v5` `CopyFrom` and Mongo `BulkWrite`).

```
       __  __       __    _ _____ __ 
  ____/ / / /_  ___/ /_  (_) __// /_
 / __  / / __ \/ ___/ __ \/ / /_ / __/
/ /_/ /_/ /_/ (__  ) / / / / __// /_  
\__,_(_)_,___/____/_/ /_/_/_/   \__/  

Version: v1.0.0 | CockroachDB • PostgreSQL • MongoDB Atlas
```

---

## ✨ Features

- **🚀 Native Binary Wire Streaming:** Utilizes PostgreSQL `pgx/v5` binary `CopyFrom` protocol for up to 10x faster ingestion compared to standard SQL inserts.
- **🧠 Ultra-Low Memory Footprint (<50MB):** Streams data in configurable chunk windows (default `2,500` rows) via streaming cursors—never buffers entire tables into RAM.
- **☁️ Cloud Database Native:** Specially tuned for **CockroachDB Cloud**, **PostgreSQL**, **Supabase**, **Neon**, and **MongoDB Atlas**.
- **🛡️ 2-Phase Constraint Handling:** Safely migrates tables with complex Foreign Key graphs by deferring constraints to post-load phase, eliminating circular dependency deadlocks.
- **🎨 Interactive Terminal Wizard:** Intuitive CLI experience with auto-detection of engine from connection URIs and simple numbered menu choices `[1]`, `[2]`.
- **📊 Real-time Progress Tracking:** Displays live transfer speed (`rows/s`), total rows streamed, elapsed time, and ETA.
- **✔ Post-Migration Verification Audit:** Automatically queries and matches source vs target row counts to verify 100% data integrity.
- **🔄 Auto Sequence Synchronization:** Automatically resets `SERIAL` and `BIGSERIAL` sequences to `MAX(id)` on target databases post-transfer.
- **🤖 Headless & CI/CD Ready:** Pass flags or environment variables for zero-interaction scripts and automated backup pipelines.

---

## 🛠️ Tech Stack

- **Language:** Go (`go1.22+` / `go1.26+`)
- **PostgreSQL / CockroachDB Driver:** `github.com/jackc/pgx/v5` (Native binary protocol & connection pooling)
- **MongoDB Driver:** `go.mongodb.org/mongo-driver/v2` (Official BSON & bulk operations)
- **CLI Framework:** `github.com/spf13/cobra`
- **Terminal UI & Styling:** `github.com/pterm/pterm`
- **Environment Management:** `github.com/joho/godotenv`
- **Containerization:** Docker multi-stage build (`alpine:3.20`)

---

## 📦 Supported Database Engines

| Database Engine | Dialect / Wire Protocol | Supported Features |
| :--- | :--- | :--- |
| **CockroachDB Cloud** | PostgreSQL Wire | Native `SHOW CREATE TABLE`, Data, Primary Keys, Indexes, Sequences, JSONB, UUIDs |
| **PostgreSQL** | Standard Postgres (v12-v17) | Full Schema + High-Speed Binary Copy |
| **Supabase** | Managed Postgres | SSL Pooler & Direct Connections |
| **Neon** | Serverless Postgres | Auto-suspend friendly connection management |
| **MongoDB Atlas** | Mongo Wire Protocol | Collections, Document BSON Streaming, Secondary Indexes |

---

## 📥 Installation

### 1. Download Prebuilt Binaries (Recommended)
Download the standalone executable for Windows, Linux, or macOS from the [GitHub Releases](https://github.com/dusmamud/dbshift/releases) page.

### 2. Via `go install`
```bash
go install github.com/dusmamud/dbshift/cmd/dbshift@latest
```

### 3. Build from Source
```bash
# 1. Clone the repository
git clone https://github.com/dusmamud/dbshift.git

# 2. Move into the directory
cd dbshift

# 3. Build binary
# On Windows PowerShell:
.\scripts\build.ps1

# On Linux / macOS:
go build -ldflags="-s -w" -o bin/dbshift ./cmd/dbshift
```

### 4. Docker
```bash
docker build -t dbshift .
docker run --rm -it --env-file .env dbshift
```

---

## 🚀 Quickstart

### Option A: Interactive Terminal Wizard (Default)
Simply run `dbshift` without arguments:
```bash
dbshift
```

1. **Auto-Detects Engine:** Reads your `.env` connection strings and automatically identifies CockroachDB / Postgres or Mongo.
2. **Inspects Schema:** Shows all tables and their exact row counts in a clean terminal table.
3. **Choose Scope:** Press `[1]` to migrate all tables or `[2]` to select specific tables.
4. **Live Stream:** Watch real-time multi-threaded streaming with rows/sec transfer rate.
5. **Verify:** Review post-migration audit table comparing source vs target row counts.

---

### Option B: Headless / Direct Command (CI/CD Pipelines)

#### CockroachDB to CockroachDB:
```bash
dbshift \
  --source="postgresql://user:pass@source-cluster.cockroachlabs.cloud:26257/defaultdb?sslmode=verify-full" \
  --target="postgresql://user:pass@target-cluster.cockroachlabs.cloud:26257/defaultdb?sslmode=verify-full" \
  --engine=postgres \
  --concurrency=4 \
  --batch-size=2500 \
  --drop-existing
```

#### MongoDB Atlas to MongoDB Atlas:
```bash
dbshift \
  --source="mongodb+srv://user:pass@example.com/prod?retryWrites=true&w=majority" \
  --target="mongodb+srv://user:pass@example.com/prod?retryWrites=true&w=majority" \
  --engine=mongo \
  --concurrency=4
```

---

## 🔐 Environment Variables

You can configure `dbshift` using a `.env` file in your root folder:

```bash
cp .env.example .env
```

| Variable | Required | Default | Description |
| :--- | :---: | :---: | :--- |
| `SOURCE_URI` | Yes | `""` | Connection URI of source (old) database |
| `TARGET_URI` | Yes | `""` | Connection URI of target (new) database |
| `DB_ENGINE` | No | `postgres` | Database engine (`postgres` or `mongo`) |
| `CONCURRENCY`| No | `4` | Number of concurrent table worker goroutines |
| `BATCH_SIZE` | No | `2500` | Number of rows per streaming chunk |

---

## 🛠️ CLI Commands & Flags

```bash
dbshift [command] [flags]
```

### Commands:
- `dbshift` — Launch interactive wizard (or direct migration if `--source` and `--target` are passed).
- `dbshift inspect --url="<URI>" --engine=postgres` — Inspect schema, column counts, indexes, and row counts.
- `dbshift verify --source="<SRC>" --target="<TGT>"` — Run standalone count verification audit between two databases.
- `dbshift version` — Print version and build info.

### Flags:
| Flag | Shorthand | Default | Description |
| :--- | :--- | :--- | :--- |
| `--source` | `-s` | `""` | Source database connection URI |
| `--target` | `-t` | `""` | Target database connection URI |
| `--engine` | `-e` | `postgres` | Database engine (`postgres` or `mongo`) |
| `--mode` | `-m` | `full` | Migration mode: `full`, `data-only`, `schema-only` |
| `--concurrency` | `-c` | `4` | Number of parallel worker goroutines |
| `--batch-size` | `-b` | `2500` | Number of rows per streaming chunk |
| `--drop-existing` | | `false` | Drop conflicting target tables/collections before copying |
| `--tables` | | `""` | Comma-separated list of specific tables (e.g. `users,orders`) |

---

## 🧪 Testing

Run all unit tests:
```bash
go test -v ./...
```

Run tests with race condition detector:
```bash
go test -v -race ./...
```

Run tests with coverage:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

---

## 📊 Verification Audit Preview

After each migration, an automated audit is performed and rendered in the terminal:

```
┌─────────────────────────┬─────────────┬─────────────┬──────────────┬────────────┐
│ Table / Collection      │ Source Rows │ Target Rows │ Verification │ Audit Time │
├─────────────────────────┼─────────────┼─────────────┼──────────────┼────────────┤
│ music_albums            │ 1,803       │ 1,803       │ ✔ MATCH      │ 18ms       │
│ music_artists           │ 13,516      │ 13,516      │ ✔ MATCH      │ 24ms       │
│ music_chart_cache       │ 50          │ 50          │ ✔ MATCH      │ 12ms       │
│ music_songs             │ 51,958      │ 51,958      │ ✔ MATCH      │ 54ms       │
│ user_playlist           │ 8           │ 8           │ ✔ MATCH      │ 8ms        │
│ user_playlist_tracks    │ 13          │ 13          │ ✔ MATCH      │ 10ms       │
└─────────────────────────┴─────────────┴─────────────┴──────────────┴────────────┘

╔══════════════════════════ MIGRATION SUCCESSFUL ══════════════════════════╗
║ All 6 tables/collections verified with 100% integrity.                   ║
║ Total Rows Transferred: 67,348                                           ║
║ Total Elapsed Time: 14.8s                                                ║
╚══════════════════════════════════════════════════════════════════════════╝
```

---

## 📁 Repository Structure

```text
dbshift/
├── .github/
│   ├── ISSUE_TEMPLATE/
│   │   ├── bug_report.md        # GitHub bug report template
│   │   └── feature_request.md   # GitHub feature request template
│   ├── workflows/
│   │   ├── ci.yml               # Automated CI test & build pipeline
│   │   └── release.yml          # Automated multi-arch release pipeline
│   └── pull_request_template.md # GitHub PR template
├── cmd/
│   └── dbshift/
│       └── main.go              # CLI entry point
├── docs/
│   ├── ARCHITECTURE.md          # Concurrency, memory & streaming design
│   ├── CLI_REFERENCE.md         # Comprehensive flag & command reference
│   └── COCKROACHDB.md           # CockroachDB Cloud migration guide
├── internal/
│   ├── config/                  # Configuration & .env parser
│   ├── core/                    # Migration engine & driver interface
│   ├── driver/
│   │   ├── mongo/               # MongoDB Atlas adapter
│   │   └── postgres/            # PostgreSQL & CockroachDB adapter
│   └── ui/                      # Terminal interactive wizard & progress bars
├── scripts/
│   └── build.ps1                # PowerShell build script for Windows
├── .dockerignore
├── .env.example
├── .gitignore
├── .golangci.yml                # Linter configuration
├── CODE_OF_CONDUCT.md           # Contributor Covenant 2.1
├── CONTRIBUTING.md              # Contribution guide
├── Dockerfile                   # Multi-stage production container build
├── LICENSE                      # MIT License
└── README.md                    # Flagship documentation
```

---

## 🗺️ Roadmap

- [x] CockroachDB Cloud ↔ CockroachDB Cloud replication
- [x] PostgreSQL / Supabase / Neon support
- [x] MongoDB Atlas document streaming
- [x] 2-Phase foreign key constraint resolution
- [x] Auto-detection of database engine from URI
- [x] Post-migration verification count audit
- [ ] MySQL ↔ MySQL engine adapter
- [ ] Cross-engine migration (MongoDB to Postgres JSONB)
- [ ] Change Data Capture (CDC) real-time streaming sync

---

## 🤝 Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) before opening a pull request.

---

## 🛡️ Security

If you discover a security vulnerability, please review our [SECURITY.md](SECURITY.md) policy for responsible disclosure.

---

## 📄 License

This project is licensed under the **MIT License** - see the [LICENSE](LICENSE) file for details.

---

## 👤 Author

**dusmamud**
- GitHub: [@dusmamud](https://github.com/dusmamud)

---

<div align="center">
⭐ Star this repository if you find it helpful!
</div>
