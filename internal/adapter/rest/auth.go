package rest

import (
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/samnart1/albergo/internal/domain/shared"
)

// guards endpoint with shared secret.
// TODO: research better design decision here...
func RequireStaffKey(key string, log *slog.Logger) func(http.Handler) http.Handler {
	// hashed before comparing
	want := sha256.Sum256([]byte(key))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			presented, ok := bearerToken(r)
			if !ok {
				unauthorized(w, r, log, "missing_credentials", "a staff api key is required")
				return
			}

			got := sha256.Sum256([]byte(presented))
			if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
				unauthorized(w, r, log, "invalid_credentials", "that staff api key is not valid")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "

	header := r.Header.Get("Authorization")
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	return header[len(prefix):], true
}

func unauthorized(w http.ResponseWriter, r *http.Request, log *slog.Logger, code, detail string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="albergo"`)
	writeProblem(w, r, log, shared.Unauthenticated(code, "%s", detail))
}
