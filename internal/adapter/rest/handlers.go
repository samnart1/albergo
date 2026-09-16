package rest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"uuid"

	"github.com/samnart1/albergo/internal/app"
	"github.com/samnart1/albergo/internal/domain/shared"
)

const maxBodyBytes = 1 << 20

type Handlers struct {
	services *app.Services
	log      *slog.Logger
}

func NewHandlers(services *app.Services, log *slog.Logger) *Handlers {
	return &Handlers{services: services, log: log}
}

func (h *Handlers) availability(w http.ResponseWriter, r *http.Request) {
	propertyID, err := pathUUID(r, "propertyID")
	if err != nil {
		writeProblem(w, r, h.log, err)
		return
	}

	stay, err := queryStay(r)
	if err != nil {
		writeProblem(w, r, h.log, err)
		return
	}

	guests := 2
	if raw := r.URL.Query().Get("guests"); raw != "" {
		if guests, err = atoi(raw, "guests"); err != nil {
			writeProblem(w, r, h.log, err)
			return
		}
	}

	offers, err := h.services.Availability.Search(r.Context(), app.SearchInput{
		PropertyID: propertyID, Stay: stay, Guests: guests,
	})
	if err != nil {
		writeProblem(w, r, h.log, err)
		return
	}

	writeJSON(w, http.StatusOK, availabilityResponse{
		CheckIn: stay.CheckIn(), CheckOut: stay.CheckOut(), Nights: stay.Nights(), Offers: toOffers(offers),
	})
}

const maxIdempotencyKeyLength = 255

func (h *Handlers) createReservation(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	switch {
	case key == "":
		writeProblem(w, r, h.log, shared.Invalid("missing_idempotency_key",
			"the Idempotency-Key header is required on this endpoint"))
		return
	case len(key) > maxIdempotencyKeyLength:
		writeProblem(w, r, h.log, shared.Invalid("invalid_idempotency_key",
			"the Idempotency-Key header must be at most %d characters", maxIdempotencyKeyLength))
		return
	}

	req, raw, err := decode[createReservationRequest](w, r)
	if err != nil {
		writeProblem(w, r, h.log, err)
		return
	}

	stay, err := shared.NewStay(req.CheckIn, req.CheckOut)
	if err != nil {
		writeProblem(w, r, h.log, err)
		return
	}

	out, err := h.services.Reservations.Reserve(r.Context(), app.ReserveInput{
		IdempotencyKey: key,
		RequestHash:    hashBody(raw),
		PropertyID:     req.PropertyID,
		RoomTypeID:     req.RoomTypeID,
		RatePlanID:     req.RatePlanID,
		Stay:           stay,
		Guests:         req.Guests,
		Email:          req.Guest.Email,
		FullName:       req.Guest.FullName,
		Phone:          req.Guest.Phone,
	})
	if err != nil {
		writeProblem(w, r, h.log, err)
		return
	}

	// replay
	if out.Replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	w.Header().Set("Location", "/v1/reservations/"+out.Reservation.Reference)
	writeJSON(w, http.StatusCreated, toReservation(out.Reservation))
}

func (h *Handlers) getReservation(w http.ResponseWriter, r *http.Request) {
	res, err := h.services.Reservations.ByReference(r.Context(), r.PathValue("reference"))
	if err != nil {
		writeProblem(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toReservation(res))
}

func (h *Handlers) cancelReservation(w http.ResponseWriter, r *http.Request) {
	out, err := h.services.Reservations.Cancel(r.Context(), r.PathValue("reference"))
	if err != nil {
		writeProblem(w, r, h.log, err)
		return
	}

	writeJSON(w, http.StatusOK, cancelResponse{
		Reservation: toReservation(out.Reservation),
		Refund:      toMoney(out.Cancellation.Refund),
		Refundable:  out.Cancellation.Refundable,
	})
}

func (h *Handlers) setInventory(w http.ResponseWriter, r *http.Request) {
	roomTypeID, err := pathUUID(r, "roomTypeID")
	if err != nil {
		writeProblem(w, r, h.log, err)
		return
	}

	req, _, err := decode[setInventoryRequest](w, r)
	if err != nil {
		writeProblem(w, r, h.log, err)
		return
	}

	if err := h.services.Staff.SetInventory(r.Context(), roomTypeID, req.From, req.To, req.Allotment); err != nil {
		writeProblem(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) setRates(w http.ResponseWriter, r *http.Request) {
	ratePlanID, err := pathUUID(r, "ratePlanID")
	if err != nil {
		writeProblem(w, r, h.log, err)
		return
	}

	req, _, err := decode[setRatesRequest](w, r)
	if err != nil {
		writeProblem(w, r, h.log, err)
		return
	}

	inputs := make([]app.RateInput, 0, len(req.Rates))
	for _, rate := range req.Rates {
		inputs = append(inputs, app.RateInput{
			Date:              rate.Date,
			PriceCents:        rate.PriceCents,
			MinStay:           rate.MinStay,
			ClosedToArrival:   rate.ClosedToArrival,
			ClosedToDeparture: rate.ClosedToDeparture,
		})
	}

	if err := h.services.Staff.SetRates(r.Context(), ratePlanID, inputs); err != nil {
		writeProblem(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) listReservations(w http.ResponseWriter, r *http.Request) {
	propertyID, err := pathUUID(r, "propertyID")
	if err != nil {
		writeProblem(w, r, h.log, err)
		return
	}

	from, err := queryDate(r, "from")
	if err != nil {
		writeProblem(w, r, h.log, err)
		return
	}
	to, err := queryDate(r, "to")
	if err != nil {
		writeProblem(w, r, h.log, err)
		return
	}

	list, err := h.services.Staff.Reservations(r.Context(), propertyID, from, to)
	if err != nil {
		writeProblem(w, r, h.log, err)
		return
	}

	out := make([]reservationDTO, 0, len(list))
	for _, res := range list {
		out = append(out, toReservation(res))
	}
	writeJSON(w, http.StatusOK, reservationListResponse{Reservations: out})
}

func decode[T any](w http.ResponseWriter, r *http.Request) (T, []byte, error) {
	var out T

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return out, nil, shared.Invalid("body_too_large", "request body exceeds %d bytes", maxBodyBytes)
		}
		return out, nil, shared.Invalid("invalid_body", "request body could not be read")
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&out); err != nil {
		return out, nil, shared.Invalid("invalid_body", "request body is not valid json: %v", err)
	}
	return out, raw, nil
}

func hashBody(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func pathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		return uuid.Nil(), shared.Invalid("invalid_id", "%s is not a valid uuid", name)
	}
	return id, nil
}

func queryDate(r *http.Request, name string) (shared.Date, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return shared.Date{}, shared.Invalid("missing_parameter", "%s is required", name)
	}
	return shared.ParseDate(raw)
}

func queryStay(r *http.Request) (shared.Stay, error) {
	checkIn, err := queryDate(r, "check_in")
	if err != nil {
		return shared.Stay{}, err
	}
	checkOut, err := queryDate(r, "check_out")
	if err != nil {
		return shared.Stay{}, err
	}
	return shared.NewStay(checkIn, checkOut)
}

func atoi(raw, name string) (int, error) {
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err != nil {
		return 0, shared.Invalid("invalid_parameter", "%s must be a whole number", name)
	}
	return n, nil
}
