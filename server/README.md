# Tender Recommendation Server (PostgreSQL)

Go backend for the diploma module with PostgreSQL storage.

## Features

- registration and login
- company profiles CRUD
- tenders list and details
- analysis requests and results
- integration message log
- external tender intake endpoint

## API

- `POST /api/auth/register`
- `POST /api/auth/login`
- `GET /api/auth/me`
- `PUT /api/auth/me`
- `GET /api/dashboard/summary`
- `GET /api/tenders`
- `GET /api/tenders/:id`
- `GET /api/companies`
- `GET /api/companies/:id`
- `POST /api/companies`
- `PUT /api/companies/:id`
- `DELETE /api/companies/:id`
- `GET /api/analysis/requests`
- `POST /api/analysis/requests`
- `GET /api/analysis/requests/:id`
- `GET /api/analysis/results/:id`
- `GET /api/integration/messages`
- `POST /api/integration/messages`

## Storage

This version uses PostgreSQL.
Schema initialization happens automatically on startup.
Demo seed is inserted only when tenders and integration messages tables are empty.

## Environment variables

```env
APP_ADDR=:8080
DATABASE_URL=postgres://postgres:postgres@postgres:5432/tender_recommendation?sslmode=disable
APP_JWT_SECRET=dev-secret-change-me
CLIENT_ORIGIN=http://localhost:5173
LLM_SERVICE_URL=
APP_ENABLE_DEMO_SEED=true
INTEGRATION_API_KEY=
```

## Run locally

```bash
go mod download
go run ./cmd/api
```

## Docker compose

Use the updated compose file from the `docker-setup-postgres` artifact. It starts:
- `postgres`
- `server`
- `client`
