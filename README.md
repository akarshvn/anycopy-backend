# AnyCopy

Copy once and access it across all your devices.

This repo is the Go backend. The Android and macOS clients live in separate repositories.

## Requirements

* Go 1.22 or newer

## Run

Set the ANYCOPY_TOKEN environment variable, then start the server:

```
go mod tidy
ANYCOPY_TOKEN=yourtoken go run .
```

The server refuses to start if ANYCOPY_TOKEN is not set. Use a long random string, for example the output of:

```
python3 -c "import secrets; print(secrets.token_hex(32))"
```

The server listens on port 8080 and creates a SQLite file named links.db on first start.

## Authentication

Every route except GET /health requires this header:

```
Authorization: Bearer yourtoken
```

Requests with a missing or wrong token get a 401 response.

## API

* GET /health returns a short status message
* POST /items stores an item and returns it
* GET /items returns all items, oldest first
* GET /items?since=5 returns items with an id greater than 5
* DELETE /items/{id} deletes an item and responds with 204, or 404 if id does not exist 

Example POST body:

```json
{
  "kind": "link",
  "content": "https://example.com",
  "source_device": "windows"
}
```

The kind field is optional and defaults to text. Allowed values are text and link.

Example response:

```json
{
  "id": 1,
  "kind": "link",
  "content": "https://example.com",
  "source_device": "windows",
  "created_at": "2026-10-21T09:30:00Z"
}
```

The created_at value is in UTC. Request bodies larger than 1 MiB are rejected with a 413 response.