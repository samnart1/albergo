package rest

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

func NewRouter(log *slog.Logger, db Pinger, h *Handlers, staffKey string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "database unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	staff := RequireStaffKey(staffKey, log)

	mux.HandleFunc("GET /v1/properties/{propertyID}/availability", h.availability)
	mux.HandleFunc("POST /v1/reservations", h.createReservation)
	mux.HandleFunc("GET /v1/reservations/{reference}", h.getReservation)
	mux.HandleFunc("POST /v1/reservations/{reference}/cancel", h.cancelReservation)

	mux.Handle("PUT /v1/properties/{propertyID}/room-types/{roomTypeID}/inventory", staff(http.HandlerFunc(h.setInventory)))
	mux.Handle("PUT /v1/properties/{propertyID}/rate-plans/{ratePlanID}/rates", staff(http.HandlerFunc(h.setRates)))
	mux.Handle("GET /v1/properties/{propertyID}/reservations", staff(http.HandlerFunc(h.listReservations)))

	return Recoverer(log)(RequestLogger(log)(mux))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
