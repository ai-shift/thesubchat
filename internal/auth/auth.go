package auth

import (
	"log/slog"
	"os"

	"github.com/clerk/clerk-sdk-go/v2"
)

func Init() {
	secretKey := os.Getenv("CLERK_SECRET_KEY")
	if secretKey == "" {
		panic("Clerk secret key is not available")
	}

	clerk.SetKey(secretKey)
	slog.Info("Clerk key setted")
}
