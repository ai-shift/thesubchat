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
	chatURI     = "/chat"
	graphURI    = "/graph"
	authURI     = "/auth"
	loginURI    = "/auth/login"
	registerURI = "/auth/register"
)

func main() {
	m := http.NewServeMux()
	conn := factory.GetDB()
	q := db.New(conn)

	secretKey := os.Getenv("CLERK_SECRET_KEY")
	if secretKey == "" {
		panic("Clerk secret key is not available")
	}

	clerk.SetKey(secretKey)
	slog.Info("Clerk key setted")

	protector := auth.NewProtectionMiddleware(secretKey)

	m.Handle(fmt.Sprintf("%s/", chatURI), http.StripPrefix(chatURI, chat.InitMux(q, protector, chatURI, graphURI)))
	m.Handle(fmt.Sprintf("%s/", graphURI), http.StripPrefix(graphURI, graph.InitMux(q, protector, chatURI)))
	m.Handle(fmt.Sprintf("%s/", authURI), http.StripPrefix(authURI, auth.InitMux(q, protector, loginURI, registerURI, graphURI)))
	slog.Info("site running on port 3000...")
	if err := http.ListenAndServe(":3000", m); err != nil {
		slog.Error("serving finished with", "err", err.Error())
	}
}
