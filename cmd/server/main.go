package main

import (
	"log/slog"
	"net/http"
	"shellshift/internal/db"
	"shellshift/internal/factory"
	"shellshift/web/features/chat"
	"shellshift/web/features/graph"
)

func main() {
	m := http.NewServeMux()
	conn := factory.GetDB()
	q := db.New(conn)
	m.Handle("/chat/", http.StripPrefix("/chat", chat.InitMux(q)))
	m.Handle("/graph/", http.StripPrefix("/graph", graph.InitMux(q, "/chat")))
	slog.Info("site running on port 3000...")
	if err := http.ListenAndServe(":3000", m); err != nil {
		slog.Error("serving finished with", "err", err.Error())
	}
}
