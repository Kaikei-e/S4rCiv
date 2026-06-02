---
name: docker-compose
description: Docker Compose command reference for the s4rCiv local stack (root compose.yaml, project name s4rciv). Services: db (Postgres), migrate (Atlas), api (Go), differ (Rust), web (SvelteKit). Use when running, restarting, or inspecting s4rCiv containers locally, or applying migrations.
---

# Docker Compose — s4rCiv

The stack is defined in `compose.yaml` at the repo root with `name: s4rciv`, so plain
`docker compose <cmd>` already targets the right project and file (no `-f` / `-p` needed).
Secrets come from `secrets/` (e.g. `db_password`); env from `.env` (see `.env.example`).

## Services

| Service | Tech | Role | Port (host) |
|---|---|---|---|
| `db` | Postgres 18 | event log + read models. Not host-published; reach as `db:5432` | — |
| `migrate` | Atlas | one-shot: applies `db/migrations` then exits (`restart: "no"`) | — |
| `api` | Go (`services/api`) | HTTP API; waits for `migrate` to complete | `127.0.0.1:${API_HOST_PORT:-8080}` |
| `differ` | Rust (`services/differ`) | background worker; waits for `db` healthy | — |
| `web` | SvelteKit/Node (`web`, pnpm) | dashboard; waits for `api` | `127.0.0.1:${WEB_HOST_PORT:-3000}` |

## Basic commands

```bash
docker compose up -d                 # start the whole stack (runs migrate, then api/differ/web)
docker compose up -d --build         # rebuild images then start
docker compose up -d <service>       # start one service
docker compose logs -f <service>     # tail logs (db | migrate | api | differ | web)
docker compose ps                    # status of all services
docker compose restart <service>     # restart one service
docker compose down                  # stop & remove containers (keeps named volumes)
docker compose down -v               # also drop volumes (destroys local event-store data)
```

## Migrations (Atlas)

The `migrate` service applies `db/migrations` on `up` (after `db` is healthy); `api` only
starts once `migrate` completes successfully. Author new migrations on the host:

```bash
atlas migrate diff <name> --env local    # generate a versioned migration from schema diff
atlas migrate hash --env local           # refresh atlas.sum after editing migration files
atlas migrate lint --latest 1 --env local
docker compose up -d migrate             # apply (or re-run on next full `up`)
```

`atlas.hcl` env `local` uses `dir = file://db/migrations` and an ephemeral `dev` DB
(`docker://postgres/18/dev`, requires Docker).

## Health checks

```bash
docker compose ps                                  # look for healthy / unhealthy
docker compose exec db pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"
curl -fsS http://127.0.0.1:${API_HOST_PORT:-8080}/   # api (also: /app/api -healthcheck inside the container)
curl -fsS http://127.0.0.1:${WEB_HOST_PORT:-3000}/   # web
```

## Notes

- Containers run hardened: `read_only`, `cap_drop: [ALL]`, `no-new-privileges`. A new
  service that needs to write must declare an explicit `tmpfs` / volume rather than
  relaxing `read_only` (see the `secrets-via-docker-secrets` memory).
- `db` is intentionally not host-published — use `docker compose exec db psql ...` for ad-hoc access.
- `down -v` destroys the hash-chained event log in the `db_data` volume; use only when you intend to reset local data.
