package main

import (
	"log/slog"
	"net/http"
	"shellshift/internal/chat"
)

func main() {
	m := http.NewServeMux()
	m.Handle("/chat/", http.StripPrefix("/chat", chat.InitMux()))
	slog.Info("site running on port 3000...")
	if err := http.ListenAndServe(":3000", m); err != nil {
		slog.Error("serving finished with", "err", err.Error())
	}
}
