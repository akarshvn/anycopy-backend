package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
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
		var it Item
		if err := json.NewDecoder(r.Body).Decode(&it); err != nil {
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

		res, err := db.Exec(
			"INSERT INTO items(kind, content, source_device) VALUES (?, ?, ?)",
			it.Kind, it.Content, it.SourceDevice,
		)

		if err != nil {
			log.Println("insert failed: ", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		id, err := res.LastInsertId()
		if err != nil {
			log.Println("last insert id: ", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		it.ID = id

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(it)
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
			"SELECT id, kind, content, source_device FROM items WHERE id > ? ORDER BY id",
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

			if err := rows.Scan(&it.ID, &it.Kind, &it.Content, &it.SourceDevice); err != nil {
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

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	}
}

func main() {
	db, err := sql.Open("sqlite", "links.db")

	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(schema); err != nil {

		log.Fatal(err)
	}
	log.Println("db ready")

	token := os.Getenv("ANYCOPY_TOKEN")
	if token == "" {
		log.Fatal("ANYCOPY_TOKEN is not set")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("okie"))
	})

	mux.HandleFunc("POST /items", postItem(db))
	mux.HandleFunc("GET /items", getItems(db))

	log.Println("listening on 8080")
	log.Fatal(http.ListenAndServe(":8080", auth(token, mux)))
}
