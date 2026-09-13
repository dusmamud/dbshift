# dbshift CLI Reference

Complete guide to all commands, flags, and usage patterns for `dbshift`.

---

## Command Hierarchy

```text
dbshift [flags]
dbshift [command]

Available Commands:
  inspect     Inspect schema, tables, and row counts of a database
  verify      Compare row counts between source and target databases
  version     Print dbshift version and runtime details
  completion  Generate shell autocompletion script
```

---

## 1. Interactive Mode (Default)

Running `dbshift` with no arguments starts the guided interactive terminal wizard:

```bash
dbshift
```

### Flow:
1. **Engine Detection:** Auto-detects engine from `.env` or prompts for `[1] PostgreSQL / CockroachDB` or `[2] MongoDB Atlas`.
2. **Connection Testing:** Pings both databases and verifies authentication.
3. **Schema Discovery:** Displays discovered tables with their exact row counts in a clean table.
4. **Scope Selection:** Choose `[1] Migrate ALL (A to Z)` or `[2] Select specific tables`.
5. **Streaming Transfer:** High-speed parallel goroutine streaming with animated progress bars.
6. **Integrity Audit:** Displays post-migration verification table comparing source vs target row counts.

---

## 2. Direct / Headless Mode (CI/CD Pipelines)

Run direct migrations without terminal prompts:

### Full Migration:
```bash
dbshift \
  --source="postgresql://user:pass@source-host:26257/defaultdb?sslmode=verify-full" \
  --target="postgresql://user:pass@target-host:26257/defaultdb?sslmode=verify-full" \
  --engine=postgres \
  --mode=full \
  --concurrency=8 \
  --batch-size=5000 \
  --drop-existing
```

### Data Only:
```bash
dbshift \
  --source="postgresql://user:pass@source-host:5432/mydb" \
  --target="postgresql://user:pass@target-host:5432/mydb" \
  --mode=data-only
```

### Specific Tables:
```bash
dbshift \
  --source="postgresql://user:pass@source-host:5432/mydb" \
  --target="postgresql://user:pass@target-host:5432/mydb" \
  --tables="users,orders,transactions"
```

---

## 3. Subcommands

### `dbshift inspect`
Inspects database connection and lists all tables, column counts, indexes, and row counts without migrating:

```bash
dbshift inspect --url="postgresql://user:pass@host:5432/mydb" --engine=postgres
```

### `dbshift verify`
Runs standalone post-migration integrity verification comparing source and target row counts:

```bash
dbshift verify \
  --source="postgresql://user:pass@source:5432/db" \
  --target="postgresql://user:pass@target:5432/db" \
  --engine=postgres
```

### `dbshift version`
Prints current build version and Go runtime:

```bash
dbshift version
```

---

## Global Flags

| Flag | Shorthand | Type | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `--source` | `-s` | `string` | `""` | Source database connection URI |
| `--target` | `-t` | `string` | `""` | Target database connection URI |
| `--engine` | `-e` | `string` | `postgres` | Database engine (`postgres` or `mongo`) |
| `--mode` | `-m` | `string` | `full` | Migration mode (`full`, `data-only`, `schema-only`) |
| `--concurrency` | `-c` | `int` | `4` | Number of parallel worker goroutines |
| `--batch-size` | `-b` | `int` | `2500` | Number of rows per streaming chunk |
| `--drop-existing` | | `bool` | `false` | Drop conflicting target tables/collections before copying |
| `--tables` | | `string` | `""` | Comma-separated list of specific tables |
