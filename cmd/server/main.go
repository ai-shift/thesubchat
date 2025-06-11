package main

import (
	"log/slog"
	"net/http"
	"shellshift/internal/chat"
	"shellshift/internal/db"
	"shellshift/internal/factory"
)

func main() {
	m := http.NewServeMux()
	conn := factory.GetDB()
	q := db.New(conn)
	m.Handle("/chat/", http.StripPrefix("/chat", chat.InitMux(q)))
	slog.Info("site running on port 3000...")
	if err := http.ListenAndServe(":3000", m); err != nil {
		slog.Error("serving finished with", "err", err.Error())
	}
}
