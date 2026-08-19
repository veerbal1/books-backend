# Book API

A REST-style Book CRUD API built with Go and PostgreSQL. It provides predictable HTTP contracts, JSON response envelopes, pagination, filtering, sorting, request-body validation, and persistent storage.

This README is the setup guide for a new developer starting from a clean clone.

## Tech

- Go
- `net/http`
- `encoding/json`
- PostgreSQL with `database/sql` and pgx
- Goose migrations
- OpenAPI 3.1 (`openapi.yaml`)

## Prerequisites

Install these before starting:

- Go version declared in [`go.mod`](go.mod)
- PostgreSQL
- [Goose](https://github.com/pressly/goose) migration CLI

Confirm the first two are available:

```bash
go version
psql --version
goose -version
```

## Quick start

### 1. Create the development database

Choose a database name and create it locally:

```bash
createdb books_backend_app
```

### 2. Configure the application

Copy the example file. `.env` is deliberately ignored by Git, so secrets and local database names are never committed.

```bash
cp .env.example .env
```

Edit `.env` and replace the placeholder `DATABASE_URL`. A local PostgreSQL installation using your macOS username and no password might look like:

```dotenv
DATABASE_URL=postgres://your_username@localhost:5432/books_backend_app?sslmode=disable
```

`PORT` is optional; the API uses `8080` when it is not set.

### 3. Apply migrations

The API does **not** change database schema when it starts. Check migration state first, then apply outstanding migrations:

```bash
set -a; source .env; set +a
GOOSE_DRIVER=postgres GOOSE_DBSTRING="$DATABASE_URL" GOOSE_MIGRATION_DIR=migrations goose status
GOOSE_DRIVER=postgres GOOSE_DBSTRING="$DATABASE_URL" GOOSE_MIGRATION_DIR=migrations goose up
```

### 4. Run the API

```bash
go run .
```

The server logs its startup and listens at `http://localhost:8080` by default.

### 5. Verify it

In a second terminal:

```bash
curl -i http://localhost:8080/health
curl -i http://localhost:8080/ready
curl -i http://localhost:8080/books
```

`/health` confirms that the Go process can answer HTTP requests. `/ready` also checks that PostgreSQL is reachable, so use it when deciding whether the API should receive traffic.

## Run locally

Books persist across server restarts. The v1 API exposes the position-1 database author as its singular `author` field.

## Testing

Unit tests use a fake store and do not need PostgreSQL:

```bash
go test ./...
```

Run the complete local quality checks that CI uses:

```bash
test -z "$(gofmt -l .)"
go test ./...
go test -race ./...
go vet ./...
```

### PostgreSQL integration tests

Integration tests use real PostgreSQL, not a mock. They require a separate database so test cleanup can never touch development data.

Create it once:

```bash
createdb books_backend_test
```

Set `TEST_DATABASE_URL` in `.env`, apply migrations to that database, then run the suite:

```bash
set -a; source .env; set +a
GOOSE_DRIVER=postgres GOOSE_DBSTRING="$TEST_DATABASE_URL" GOOSE_MIGRATION_DIR=migrations goose up
TEST_DATABASE_URL="$TEST_DATABASE_URL" go test ./...
```

The integration suite truncates only `book_authors`, `books`, and `authors` in the database named by `TEST_DATABASE_URL`, before and after each test. Never point that variable at your development database.

## Continuous integration

[`.github/workflows/ci.yml`](.github/workflows/ci.yml) runs on every push and pull request. It checks formatting, unit tests, the race detector, and `go vet` on a clean Ubuntu machine.

This first CI workflow intentionally does not start PostgreSQL, so integration tests safely skip there until a PostgreSQL CI service is added as part of the next integration-testing upgrade.

## API endpoints

| Method | Route | Purpose | Success |
|---|---|---|---:|
| `GET` | `/health` | Process liveness check | `200` |
| `GET` | `/ready` | API and PostgreSQL readiness check | `200` or `503` |
| `GET` | `/welcome` | Welcome message | `200` |
| `GET` | `/books` | List, filter, sort, and paginate books | `200` |
| `POST` | `/books` | Create a book | `201` |
| `GET` | `/books/{id}` | Retrieve one book | `200` |
| `PATCH` | `/books/{id}` | Partially update one book | `200` |
| `DELETE` | `/books/{id}` | Delete one book | `204` |

## Response contract

Successful JSON responses use a `data` envelope:

```json
{
  "data": {
    "id": 1,
    "title": "Rich Dad, Poor Dad",
    "author": "Japanese"
  }
}
```

Collection responses include pagination metadata:

```json
{
  "data": [],
  "pagination": {
    "limit": 10,
    "offset": 0,
    "total": 0,
    "has_more": false
  }
}
```

Handler-generated errors use a structured JSON response:

```json
{
  "error": {
    "code": "book_not_found",
    "message": "Request book not found"
  }
}
```

`DELETE` success returns `204 No Content`, so it intentionally has no response body.

## List query parameters

`GET /books` supports optional filters, sorting, and pagination.

| Parameter | Meaning | Default / allowed values |
|---|---|---|
| `author` | Case-insensitive exact author filter | Optional |
| `title` | Case-insensitive partial title filter | Optional |
| `sort` | Sort field | `id` (default), `title`, `author` |
| `order` | Sort direction | `asc` (default), `desc` |
| `limit` | Maximum books returned | `10` default, `1`–`100` |
| `offset` | Matching books to skip | `0` default, `0` or greater |

The list pipeline is:

```text
filter → sort → calculate total → paginate → response
```

Example:

```bash
curl -i 'http://localhost:8080/books?title=go&sort=title&order=asc&limit=10&offset=0'
```

## Create and update

`POST /books` and `PATCH /books/{id}` require `Content-Type: application/json` (including valid parameters such as `charset=utf-8`). Request bodies are limited to 64 KiB.

Create a book:

```bash
curl -i -X POST \
  -H 'Content-Type: application/json' \
  --data '{"title":"Learning Go","author":"Jon Bodner"}' \
  http://localhost:8080/books
```

Partially update a book:

```bash
curl -i -X PATCH \
  -H 'Content-Type: application/json' \
  --data '{"title":"Learning Go, Second Edition"}' \
  http://localhost:8080/books/1
```

## Error behavior

| Situation | Status | Error code |
|---|---:|---|
| Invalid ID, JSON, pagination, or sorting | `400` | `invalid_id` or `invalid_request` |
| Invalid field values | `400` | `invalid_values` |
| Missing book | `404` | `book_not_found` |
| Body exceeds 64 KiB | `413` | `body_too_large` |
| Unsupported/missing JSON content type | `415` | `unsupported_media_type` |

The standard-library router owns unknown-route `404` and unsupported-method `405` fallback responses; those currently use Go's default plain-text response format.

## OpenAPI

[`openapi.yaml`](openapi.yaml) is the machine-readable API contract. It documents all current endpoints, parameters, schemas, request bodies, and responses. It can be used with tools such as Swagger UI, Redoc, Postman, or client-code generators.

## Current limitations

- No authentication or authorization.
- No idempotency-key support for retried create requests.
- No Swagger UI is hosted yet; `openapi.yaml` is ready for one.
