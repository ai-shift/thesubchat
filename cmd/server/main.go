package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"shellshift/internal/db"
	"shellshift/internal/factory"
	"shellshift/web/features/auth"
	"shellshift/web/features/chat"
	"shellshift/web/features/graph"
)

const (
	chatURI  = "/chat"
	graphURI = "/graph"
	authURI  = "/auth"
)

func main() {
	m := http.NewServeMux()
	conn := factory.GetDB()
	q := db.New(conn)
	m.Handle(fmt.Sprintf("%s/", chatURI), http.StripPrefix(chatURI, chat.InitMux(q, chatURI, graphURI)))
	m.Handle(fmt.Sprintf("%s/", graphURI), http.StripPrefix(graphURI, graph.InitMux(q, chatURI)))
	m.Handle(fmt.Sprintf("%s/", authURI), http.StripPrefix(authURI, auth.InitMux(q)))
	slog.Info("site running on port 3000...")
	if err := http.ListenAndServe(":3000", m); err != nil {
		slog.Error("serving finished with", "err", err.Error())
	}
}
