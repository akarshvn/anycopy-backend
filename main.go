package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS items(
    id	INTEGER PRIMARY KEY AUTOINCREMENT,
    kind TEXT NOT NULL DEFAULT 'text',
    content TEXT NOT NULL,
    source_device TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

type Item struct {
	ID           int64  `json:"id"`
	Kind         string `json:"kind"`
	Content      string `json:"content"`
	SourceDevice string `json:"source_device"`
	CreatedAt    string `json:"created_at"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Println("encode response: ", err)
	}
}

func auth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		got := r.Header.Get("Authorization")
		want := "Bearer " + token

		if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func postItem(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var it Item

		var tooBig *http.MaxBytesError
		if err := json.NewDecoder(r.Body).Decode(&it); err != nil {
			if errors.As(err, &tooBig) {
				http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
				return
			}

			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if it.Content == "" || it.SourceDevice == "" {
			http.Error(w, "content and source_device are required.", http.StatusBadRequest)
			return
		}

		if it.Kind == "" {
			it.Kind = "text"
		}
		switch it.Kind {
		case "text", "link":
		default:
			http.Error(w, "unknown kind", http.StatusBadRequest)
			return
		}

		err := db.QueryRow(
			"INSERT INTO items(kind, content, source_device) VALUES (?, ?, ?) RETURNING id, created_at",
			it.Kind, it.Content, it.SourceDevice,
		).Scan(&it.ID, &it.CreatedAt)

		if err != nil {
			log.Println("insert failed: ", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusCreated, it)
	}
}

func getItems(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		since := int64(0)
		if s := r.URL.Query().Get("since"); s != "" {
			n, err := strconv.ParseInt(s, 10, 64)

			if err != nil {
				http.Error(w, "since must be an integer", http.StatusBadRequest)
				return
			}
			since = n
		}

		rows, err := db.Query(
			"SELECT id, kind, content, source_device, created_at FROM items WHERE id > ? ORDER BY id",
			since,
		)
		if err != nil {
			log.Println("query failed: ", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		items := []Item{}

		for rows.Next() {
			var it Item

			if err := rows.Scan(&it.ID, &it.Kind, &it.Content, &it.SourceDevice, &it.CreatedAt); err != nil {
				log.Println("scan failed: ", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			items = append(items, it)
		}

		if err := rows.Err(); err != nil {
			log.Println("rows error: ", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, items)
	}
}

func deleteItem(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

		if err != nil {
			log.Println("ParseInt of id failed: ", err)
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		res, err := db.Exec("DELETE FROM items WHERE id = ?", id)
		if err != nil {
			log.Println("delete failed: ", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		n, err := res.RowsAffected()
		if err != nil {
			log.Println("rows affected: ", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		if n == 0 {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func main() {

	token := os.Getenv("ANYCOPY_TOKEN")
	if token == "" {
		log.Fatal("ANYCOPY_TOKEN is not set")
	}

	db, err := sql.Open("sqlite", "file:links.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")

	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(schema); err != nil {

		log.Fatal(err)
	}
	log.Println("db ready")

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("okie"))
	})

	mux.HandleFunc("POST /items", postItem(db))
	mux.HandleFunc("GET /items", getItems(db))
	mux.HandleFunc("DELETE /items/{id}", deleteItem(db))

	log.Println("listening on 8080")
	log.Fatal(http.ListenAndServe(":8080", auth(token, mux)))
}
