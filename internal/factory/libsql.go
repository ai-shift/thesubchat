package factory

import (
	"database/sql"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	"log"
	"os"
)

func GetDB() *sql.DB {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		panic("Set DATABASE_URL env var")
	}
	token := os.Getenv("DATABASE_TOKEN")
	if len(token) > 0 {
		url = url + "?authToken=" + token
	}
	conn, err := sql.Open("libsql", url)
	if err != nil {
		log.Fatalf("failed to open db %s: %s", url, err)
	}
	return conn
}
