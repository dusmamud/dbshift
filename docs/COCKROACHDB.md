# CockroachDB Cloud Migration Guide

This guide walks you through migrating from an old CockroachDB Cloud cluster to a new CockroachDB Cloud cluster using `dbshift`.

---

## 1. Retrieve Connection Strings

In CockroachDB Cloud Console:
1. Navigate to your **Cluster** → Click **Connect**.
2. Select **General connection string**.
3. Copy the URL. It will look like:
   ```
   postgresql://<username>:<password>@<cluster-host>.cockroachlabs.cloud:26257/<database>?sslmode=verify-full
   ```
4. Repeat for both your **Source (Old)** cluster and **Target (New)** cluster.

---

## 2. Firewall / IP Allowlist

Ensure the machine where you run `dbshift` has its IP address added to the **Networking / IP Allowlist** on both CockroachDB clusters:
- In CockroachDB Cloud → **Networking** → **Add Allowed IP** (or `0.0.0.0/0` temporarily for setup).

---

## 3. Running the Migration

### Option A: Interactive Terminal Mode (Recommended)
Simply run `dbshift` without arguments:
```powershell
.\bin\dbshift.exe
```
Follow the interactive prompts:
1. Select **PostgreSQL / CockroachDB / Supabase / Neon (SQL)**.
2. Paste the Source CockroachDB URI.
3. Paste the Target CockroachDB URI.
4. Select whether to copy all tables or specific ones.
5. Choose **Full Migration (Replicate Schema + Stream Data)**.
6. Watch live progress and review the verification table.

### Option B: Headless / Direct Command
```powershell
.\bin\dbshift.exe `
  --source="postgresql://root:oldpass@oldcluster.cockroachlabs.cloud:26257/defaultdb?sslmode=verify-full" `
  --target="postgresql://root:newpass@newcluster.cockroachlabs.cloud:26257/defaultdb?sslmode=verify-full" `
  --engine=postgres `
  --concurrency=4 `
  --batch-size=2500
```

---

## 4. Verification

After migration completes, `dbshift` will automatically verify row counts between the clusters:
```
┌──────────────────┬─────────────┬─────────────┬──────────────┬────────────┐
│ Table            │ Source Rows │ Target Rows │ Verification │ Audit Time │
├──────────────────┼─────────────┼─────────────┼──────────────┼────────────┤
│ users            │ 50,000      │ 50,000      │ ✔ MATCH      │ 24ms       │
│ orders           │ 184,210     │ 184,210     │ ✔ MATCH      │ 38ms       │
│ transactions     │ 420,900     │ 420,900     │ ✔ MATCH      │ 62ms       │
└──────────────────┴─────────────┴─────────────┴──────────────┴────────────┘
```

You can also run standalone verification anytime without copying:
```powershell
.\bin\dbshift.exe verify --source="<OLD_URI>" --target="<NEW_URI>"
```
