# Book API

A REST-style Book CRUD API built with Go's standard library. It keeps books in memory and focuses on predictable HTTP contracts: JSON response envelopes, structured handler errors, pagination, filtering, sorting, and request-body validation.

## Tech

- Go
- `net/http`
- `encoding/json`
- OpenAPI 3.1 (`openapi.yaml`)

## Run locally

```bash
go run .
```

The API listens at `http://localhost:8080`.

## API endpoints

| Method | Route | Purpose | Success |
|---|---|---|---:|
| `GET` | `/health` | Health check | `200` |
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

- Books are stored only in memory and disappear when the server restarts.
- No authentication, authorization, database, or persistence yet.
- No idempotency-key support for retried create requests.
- No Swagger UI is hosted yet; `openapi.yaml` is ready for one.
