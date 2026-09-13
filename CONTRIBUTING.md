# Contributing to dbshift

Thank you for your interest in contributing to `dbshift`! We welcome contributions from developers of all skill levels.

---

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md). Please read it before contributing.

---

## How to Contribute

### 1. Reporting Bugs
- Check the [existing issues](https://github.com/dusmamud/dbshift/issues) to avoid duplicates.
- Use our [Bug Report Template](.github/ISSUE_TEMPLATE/bug_report.md).
- Include database engine, version, operating system, and steps to reproduce.

### 2. Suggesting Enhancements
- Use our [Feature Request Template](.github/ISSUE_TEMPLATE/feature_request.md).
- Clearly explain the use case and expected behavior.

### 3. Submitting Pull Requests
1. **Fork the Repository:**
   ```bash
   git clone https://github.com/<your-username>/dbshift.git
   cd dbshift
   ```
2. **Create a Feature Branch:**
   Follow our branch naming conventions:
   ```bash
   git checkout -b feature/cockroach-partition-streaming
   # or
   git checkout -b fix/mongo-cursor-timeout
   ```
3. **Make Your Changes:**
   - Keep functions focused and modular.
   - Write unit tests for new functionality.
   - Ensure `go test ./...` passes without errors.
   - Format your code with `go fmt ./...`.
4. **Commit Your Changes:**
   Use [Conventional Commits](https://www.conventionalcommits.org/):
   - `feat: add support for Supabase pooled connections`
   - `fix: resolve foreign key ordering in CockroachDB`
   - `docs: update CLI flag reference in README`
   - `test: add unit tests for URI mask utility`
   - `perf: optimize pgx CopyFrom batch allocations`
5. **Push and Open a PR:**
   - Push to your fork: `git push origin feature/your-feature`
   - Fill out the [Pull Request Template](.github/pull_request_template.md).

---

## Development Setup

### Prerequisites
- **Go:** `go1.22+` (Recommended `go1.26+`)
- **Git**

### Build and Test
```bash
# Run all unit tests
go test -v ./...

# Build binary
go build -o bin/dbshift ./cmd/dbshift

# Windows PowerShell:
.\scripts\build.ps1
```

---

## Coding Standards

1. **Error Handling:** Never ignore errors silently. Wrap errors with `core.NewError(engine, op, msg, err)`.
2. **Zero Secrets:** Never log passwords, tokens, or raw database connection strings. Use `ui.MaskURI()`.
3. **Clean Architecture:** Keep adapters inside `internal/driver/` and orchestration logic inside `internal/core/`.
4. **Minimal Dependencies:** Prefer standard library packages and existing driver dependencies (`pgx`, `mongo-driver`).
