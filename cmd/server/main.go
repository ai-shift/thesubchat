package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"shellshift/internal/db"
	"shellshift/internal/factory"
	"shellshift/web/features/auth"
	"shellshift/web/features/chat"
	"shellshift/web/features/graph"

	"github.com/clerk/clerk-sdk-go/v2"
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
	dbFactory := db.NewFactory("sql/migrations/schema.sql")
	secretKey := os.Getenv("CLERK_SECRET_KEY")
	if secretKey == "" {
		panic("Clerk secret key is not available")
	}

	clerk.SetKey(secretKey)
	slog.Info("Clerk key setted")

	protector := auth.NewProtectionMiddleware(authURI, secretKey)

	m.Handle("/", http.RedirectHandler(chatURI, http.StatusMovedPermanently))
	m.Handle(fmt.Sprintf("%s/", chatURI), http.StripPrefix(chatURI, chat.InitMux(dbFactory, q, protector, chatURI, graphURI)))
	m.Handle(fmt.Sprintf("%s/", graphURI), http.StripPrefix(graphURI, graph.InitMux(q, protector, chatURI)))
	m.Handle(fmt.Sprintf("%s/", authURI), http.StripPrefix(authURI, auth.InitMux(q, protector, secretKey, authURI, chatURI)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "42069"
	}
	slog.Info("site running", "port", port)
	if err := http.ListenAndServe(":"+port, m); err != nil {
		slog.Error("serving finished with", "err", err.Error())
	}
}
