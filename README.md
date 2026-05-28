# Water Intensity Monitoring — Speciality Steels UK

A Go-based web application for tracking water usage across Rotherham, Stocksbridge,
Brinsworth and Wednesbury sites, with KPIs, dashboards, manual data entry,
EEmon/Trend integrations, and admin tooling.

This release upgrades the system to a **production-ready** stack: user accounts
with roles, session-based auth, audit logging, password reset, median-based
auto-fill, and env-driven configuration.

---

## 1. Quick start

```bash
# 1. Copy the env template and edit (set WMS_JWT_SECRET!)
cp app.env.example app.env
$EDITOR app.env

# 2. Build & run
go build -o wms .
./wms

# 3. Open
open http://localhost:8080/login
```

On the first start, the server bootstraps the admin user from
`WMS_ADMIN_EMAIL`/`WMS_ADMIN_PASSWORD`. **Sign in and change the password
immediately** — the admin is flagged `must_change_password`.

---

## 2. Configuration (`app.env`)

| Variable | Default | Purpose |
| --- | --- | --- |
| `WMS_HOST` / `WMS_PORT` | `0.0.0.0:8080` | Listen address |
| `WMS_DB_DRIVER` | `json` | `json` (built-in) or future `postgres`/`mysql` |
| `WMS_DB_PATH` | `./data/water.json` | JSON store path |
| `WMS_DB_DSN` | — | SQL DSN (when driver != json) |
| `WMS_FRONTEND_PATH` | `./frontend/index.html` | Index file |
| `WMS_JWT_SECRET` | _required in prod_ | HMAC secret for session tokens |
| `WMS_SESSION_TTL` | `12h` | Session lifetime |
| `WMS_BCRYPT_COST` | `12` | PBKDF2 cost factor (10–14) |
| `WMS_API_KEY` | — | Legacy machine API key (bypasses user auth) |
| `WMS_ADMIN_EMAIL` / `WMS_ADMIN_PASSWORD` | `admin@example.com` / `ChangeMe!123` | Bootstrap admin |

Real environment variables always win over `app.env`.

---

## 3. Authentication & RBAC

The system uses HMAC-SHA256 signed session tokens issued by
`POST /api/auth/login`. Tokens are revocable: every issued token is paired
with a `Session` row keyed by a SHA-256 hash of the token, and logout /
password change revokes them.

Roles:

| Role | Capabilities |
| --- | --- |
| `admin` | Full access. User CRUD, activity log, data tools |
| `manager` | Read + write data entry |
| `user` | Read + write data entry |
| `viewer` | Read-only |

Failed login attempts are tracked per user; 5 failures lock the account for
15 minutes. The login endpoint is also IP-rate-limited (10/minute).

### Auth endpoints

```
POST /api/auth/login          { email, password } → { token, user, expires_at }
POST /api/auth/logout         (Bearer)
GET  /api/auth/me             (Bearer)
POST /api/auth/change-password { current_password, new_password }
```

### Admin endpoints

```
GET    /api/admin/users
POST   /api/admin/users                       { email, name, role, password }
GET    /api/admin/users/{id}
PUT    /api/admin/users/{id}                  partial update
DELETE /api/admin/users/{id}
POST   /api/admin/users/{id}/reset-password   { new_password? } → { temporary_password? }

GET    /api/admin/activity?user_id=&limit=
POST   /api/admin/median-fill                 { meter_id?, site_id?, target_date?, lookback_days?, freshness_days? }
POST   /api/admin/autofill                    legacy mean-based fill (single meter)
POST   /api/admin/autofill-all                legacy mean-based fill (bulk)
POST   /api/admin/clear-data                  irreversible
GET    /api/admin/preferences
PUT    /api/admin/preferences
GET    /api/admin/connection-status
```

### Median fill

The admin UI provides a “Median fill” tool that backfills missing readings
using the **median** of recent usage values for the same meter. Median is
robust against outliers and recommended over the previous mean-based
auto-fill. See `internal/database/median.go`.

---

## 4. Activity log

Every authentication event, user-management mutation, password change, and
admin data operation is appended to `data/users.json::activity`. The log is
capped to the last 10 000 entries (file is rotated in-process). Admins view
the log under `/admin#activity`.

---

## 5. Security posture

- PBKDF2-SHA256 password hashing (per-user salt, configurable cost).
- Constant-time password and token comparisons.
- HMAC-signed session tokens; server-side revocation list.
- Account lockout after repeated failures.
- IP-based rate-limiting on `/api/auth/login`.
- Strict security headers (`X-Content-Type-Options`, `X-Frame-Options: DENY`,
  HSTS, Referrer-Policy, CSP).
- Atomic writes of the user store (`*.tmp` + `rename`).
- Body size limit (1 MiB) and request timeouts on the HTTP server.

> **Production checklist**
> - Terminate TLS at a reverse proxy (nginx / Caddy / cloud LB).
> - Set `WMS_JWT_SECRET` to ≥ 32 random bytes and keep it stable.
> - Run as a non-root user; mount `./data` on a dedicated, backed-up volume.
> - Schedule daily backups of `data/water.json` and `data/users.json`.
> - Probe `/api/health` for liveness.

---

## 6. Recommendations for further enhancement

1. **SQL persistence.** The data layer is intentionally pluggable
   (`WMS_DB_DRIVER`). For multi-instance deployments, port readings, tonnes,
   users and sessions to PostgreSQL with `database/sql` + connection pool.
   Use `golang-migrate` for schema versioning.
2. **Single Sign-On.** Add OIDC (Microsoft Entra ID). Map the `groups` claim
   to the role table.
3. **Email delivery.** Wire password-reset and account-creation emails
   through SMTP or a transactional provider (SendGrid, SES).
4. **Background ingestion.** Move EEmon / Trend syncs into a worker
   (`cron`-style ticker) with exponential backoff and per-meter heartbeats.
5. **Live updates.** Push dashboard refreshes via WebSockets / SSE instead of
   polling.
6. **Observability.** Structured logging (`slog`), request IDs, and
   Prometheus metrics middleware.
7. **CI/CD.** `go test ./...`, `staticcheck`, `govulncheck`, container image
   build, automated deploy.
8. **Frontend.** Migrate the single-file dashboard to a component framework
   (Vue / React / Svelte): keyboard shortcuts, optimistic updates,
   accessibility audit, mobile-first layout for shop-floor tablets.
9. **Data quality.** Anomaly detection on readings (IQR / Hampel filter) to
   flag suspicious entries before they hit reports.
10. **Forecasting.** Per-meter and per-site usage forecasts (seasonal
    decomposition, ARIMA) to drive efficiency targets.

---

## 7. Layout

```
.
├── main.go
├── app.env.example
├── data/                  # JSON datastores (water.json, users.json)
├── frontend/
│   ├── index.html         # Main dashboard (token-aware)
│   ├── login.html         # Sign-in page
│   └── admin.html         # User mgmt, activity log, data tools
└── internal/
    ├── api/               # HTTP handlers, middleware, routing
    ├── auth/              # Password hashing, token signing
    ├── config/            # JSON + env loaders
    ├── database/          # DB, UserStore, median fill
    ├── models/            # Domain + user/session/audit models
    └── reports/           # Export helpers
```
