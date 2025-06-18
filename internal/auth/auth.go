package auth

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	"github.com/clerk/clerk-sdk-go/v2/user"
)

func Init() {
	secretKey := os.Getenv("CLERK_SECRET_KEY")
	if secretKey == "" {
		panic("Clerk secret key is not available")
	}

	clerk.SetKey(secretKey)
	slog.Info("Clerk key setted")
}

func ProtectedRoute(next func(http.ResponseWriter, *http.Request)) http.Handler {

	inner := func(w http.ResponseWriter, r *http.Request) {
		claims, ok := clerk.SessionClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			slog.Error("Failed to access the route. User unauthorized")
			return
		}

		usr, err := user.Get(r.Context(), claims.Subject)
		if err != nil {
			slog.Error("Failed to get the user", "with", err)
		}

		slog.Info("User found", "user", usr)

		next(w, r)
	}

	return clerkhttp.WithHeaderAuthorization()(http.HandlerFunc(inner))
}
