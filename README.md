# All In One Key

Lightweight AI API Key & Personal Account Manager.

## Version

Current version: `0.0.0`

The project follows SemVer-style `MAJOR.MINOR.PATCH` versioning.
Git tags use `vX.Y.Z`, for example `v0.1.0`.

## Features

- Zero-knowledge security: Master Key is not stored in the database
- AES-256-GCM encrypted API Keys, account passwords, and TOTP secrets
- Argon2id Master Key verifier
- AES-GCM sealed session token: Master Key is encrypted inside an opaque bearer token instead of readable JWT claims
- Multiple AI Key providers, pool groups, custom OpenAI-compatible relays, and per-key proxy URL
- Batch API Key import
- API Key health probing without fake balance accounting
- Secure account vault with favicon and TOTP display
- Invitation-based multi-user registration with expiry
- Login brute-force cooldown
- Admin-only invitation management
- Audit logs
- Encrypted JSON / CSV export and JSON import
- Offline decrypt helper for exported ciphertext
- Docker-ready deployment with host UID/GID data ownership protection

## Tech Stack

- Backend: Go 1.25.0 + Gin + GORM + SQLite
- Frontend: Vue 3 + TypeScript + Vite + TailwindCSS v4 + Pinia
- Package manager: Bun
- Deployment: Docker multi-stage build

## Development

```bash
make dev
```

Backend only:

```bash
make dev-server
```

Frontend only:

```bash
make dev-web
```

Build:

```bash
make build
```

Reset local dev data after forgetting a test account or Master Key:

```bash
make clean-data
make dev
```

or:

```bash
make reset-data
```

## Docker

```bash
export ALLINONEKEY_JWT_SECRET='[REDACTED_RANDOM_SECRET]'
export ALLINONEKEY_SESSION_SECRET='[REDACTED_RANDOM_SECRET]'
make docker-up
```

Stop Docker:

```bash
make docker-down
```

The compose stack mounts `./data:/app/data`. The entrypoint fixes ownership to `PUID:PGID` before dropping privileges, so local `data/` should not become root-owned.

Production must set strong secrets. `ALLINONEKEY_SESSION_SECRET` seals the opaque session token. `ALLINONEKEY_JWT_SECRET` is kept as an app-level compatibility/guard secret. Do not reuse weak examples in production.

## Backup, Restore, and Offline Decrypt

Encrypted JSON export is available from the Dashboard or API:

```bash
curl -H "Authorization: Bearer [REDACTED]" http://127.0.0.1:8080/api/export/json -o allinonekey-export.json
```

CSV export is also available:

```bash
curl -H "Authorization: Bearer [REDACTED]" http://127.0.0.1:8080/api/export/csv -o allinonekey-export.csv
```

JSON import preserves encrypted ciphertext and reassigns records to the current user:

```bash
curl -X POST http://127.0.0.1:8080/api/import/json \
  -H "Authorization: Bearer [REDACTED]" \
  -H "Content-Type: application/json" \
  --data-binary @allinonekey-export.json
```

Offline decrypt helper:

```bash
go run scripts/decrypt.go allinonekey-export.json '[REDACTED_MASTER_KEY]'
```

Single ciphertext decrypt:

```bash
go run scripts/decrypt.go '<ciphertext_base64>' '[REDACTED_MASTER_KEY]'
```

Keep exported files private. They contain encrypted ciphertext and metadata, but they are still sensitive backups.

## Documentation

See [`REQUIREMENTS.md`](./REQUIREMENTS.md) for the living product requirements, API contracts, versioning rules, and engineering boundaries.
