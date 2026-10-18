# AnyCopy

Copy once and access it across all your devices. 

This repo is the Go backend. The Android and macOS clients live in separate repositories.

## Requirements

* Go 1.22 or newer

## Run

```
go mod tidy
go run .
```

The server listens on port 8080 and creates a SQLite file named links.db on first start.

## API

* GET /health returns a short status message
* POST /items stores an item
* GET /items returns all items, oldest first
* GET /items?since=5 returns items with an id greater than 5

Example POST body:

```json
{
  "kind": "link",
  "content": "https://example.com",
  "source_device": "windows"
}
```

The kind field is optional and defaults to text. Allowed values are text and link.