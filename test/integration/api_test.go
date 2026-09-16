//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/samnart1/albergo/internal/adapter/postgres"
	"github.com/samnart1/albergo/internal/adapter/rest"
	"github.com/samnart1/albergo/internal/app"
	"github.com/samnart1/albergo/internal/domain/shared"
	"github.com/samnart1/albergo/internal/platform/logging"
	"github.com/samnart1/albergo/test/contract"
)

const testStaffKey = "test-staff-key"

func newServer(t *testing.T) (*httptest.Server, contract.Fixture) {
	t.Helper()
	truncate(t)
	fx := seed(t)

	log := logging.New("error", "development")
	services := app.New(app.Deps{
		Tx:           postgres.NewTxManager(pool),
		Catalog:      postgres.NewCatalogRepository(pool),
		Rates:        postgres.NewRateRepository(pool),
		Inventory:    postgres.NewInventoryRepository(pool),
		Guests:       postgres.NewGuestRepository(pool),
		Reservations: postgres.NewReservationRepository(pool),
		Idempotency:  postgres.NewIdempotencyRepository(pool),
		Outbox:       postgres.NewOutboxRepository(pool),
		Clock:        shared.FixedClock{Instant: time.Date(2026, time.February, 1, 12, 0, 0, 0, time.UTC)},
	})

	srv := httptest.NewServer(rest.NewRouter(log, pool, rest.NewHandlers(services, log), testStaffKey))
	t.Cleanup(srv.Close)

	return srv, fx
}

func do(t *testing.T, method, url string, body any) (int, []byte) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if method == http.MethodPost {
		req.Header.Set("Idempotency-Key", uuid.New().String())
	}

	req.Header.Set("Authorization", "Bearer "+testStaffKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, out
}

func decodeInto[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	return out
}

type problemBody struct {
	Status int    `json:"status"`
	Code   string `json:"code"`
}

func TestBookingLifecycle(t *testing.T) {
	srv, fx := newServer(t)

	status, body := do(t, http.MethodPut,
		fmt.Sprintf("%s/v1/properties/%s/room-types/%s/inventory", srv.URL, fx.PropertyID, fx.RoomTypeID),
		map[string]any{"from": "2026-03-14", "to": "2026-03-20", "allotment": 1})
	if status != http.StatusNoContent {
		t.Fatalf("set inventory = %d %s", status, body)
	}

	rates := []map[string]any{}
	for _, d := range []string{"2026-03-14", "2026-03-15", "2026-03-16"} {
		rates = append(rates, map[string]any{"date": d, "price_cents": 12_000, "min_stay": 1})
	}
	status, body = do(t, http.MethodPut,
		fmt.Sprintf("%s/v1/properties/%s/rate-plans/%s/rates", srv.URL, fx.PropertyID, fx.RatePlanID),
		map[string]any{"rates": rates})
	if status != http.StatusNoContent {
		t.Fatalf("set rates = %d %s", status, body)
	}

	status, body = do(t, http.MethodGet,
		fmt.Sprintf("%s/v1/properties/%s/availability?check_in=2026-03-14&check_out=2026-03-16&guests=2", srv.URL, fx.PropertyID), nil)
	if status != http.StatusOK {
		t.Fatalf("availability = %d %s", status, body)
	}

	availability := decodeInto[struct {
		Nights int `json:"nights"`
		Offers []struct {
			RatePlanID string `json:"rate_plan_id"`
			UnitsLeft  int    `json:"units_left"`
			Total      struct {
				AmountCents int64 `json:"amount_cents"`
			} `json:"total"`
		} `json:"offers"`
	}](t, body)

	if availability.Nights != 2 || len(availability.Offers) != 1 {
		t.Fatalf("availability = %+v", availability)
	}
	if availability.Offers[0].Total.AmountCents != 24_000 || availability.Offers[0].UnitsLeft != 1 {
		t.Fatalf("offer = %+v", availability.Offers[0])
	}

	create := map[string]any{
		"property_id":  fx.PropertyID.String(),
		"room_type_id": fx.RoomTypeID.String(),
		"rate_plan_id": fx.RatePlanID.String(),
		"check_in":     "2026-03-14",
		"check_out":    "2026-03-16",
		"guests":       2,
		"guest":        map[string]string{"email": "sam@example.com", "full_name": "Sam T", "phone": ""},
	}

	status, body = do(t, http.MethodPost, srv.URL+"/v1/reservations", create)
	if status != http.StatusCreated {
		t.Fatalf("create = %d %s", status, body)
	}

	created := decodeInto[struct {
		Reference string `json:"reference"`
		Status    string `json:"status"`
		Nights    []any  `json:"nights"`
	}](t, body)

	if created.Status != "confirmed" || len(created.Nights) != 2 {
		t.Fatalf("created = %+v", created)
	}

	status, body = do(t, http.MethodPost, srv.URL+"/v1/reservations", create)
	if status != http.StatusConflict {
		t.Fatalf("second booking = %d %s", status, body)
	}
	if p := decodeInto[problemBody](t, body); p.Code != "sold_out" {
		t.Fatalf("code = %q, want sold_out", p.Code)
	}

	_, body = do(t, http.MethodGet,
		fmt.Sprintf("%s/v1/properties/%s/availability?check_in=2026-03-14&check_out=2026-03-16&guests=2", srv.URL, fx.PropertyID), nil)
	if offers := decodeInto[struct {
		Offers []any `json:"offers"`
	}](t, body); len(offers.Offers) != 0 {
		t.Fatalf("sold out stay still offered: %s", body)
	}

	status, body = do(t, http.MethodPost, srv.URL+"/v1/reservations/"+created.Reference+"/cancel", nil)
	if status != http.StatusOK {
		t.Fatalf("cancel = %d %s", status, body)
	}

	cancelled := decodeInto[struct {
		Refundable bool `json:"refundable"`
		Refund     struct {
			AmountCents int64 `json:"amount_cents"`
		} `json:"refund"`
	}](t, body)

	if !cancelled.Refundable || cancelled.Refund.AmountCents != 24_000 {
		t.Fatalf("cancellation = %+v", cancelled)
	}

	status, body = do(t, http.MethodPost, srv.URL+"/v1/reservations/"+created.Reference+"/cancel", nil)
	if status != http.StatusConflict {
		t.Fatalf("double cancel = %d %s", status, body)
	}

	_, body = do(t, http.MethodGet,
		fmt.Sprintf("%s/v1/properties/%s/availability?check_in=2026-03-14&check_out=2026-03-16&guests=2", srv.URL, fx.PropertyID), nil)
	if offers := decodeInto[struct {
		Offers []any `json:"offers"`
	}](t, body); len(offers.Offers) != 1 {
		t.Fatalf("cancellation did not release the room: %s", body)
	}
}

func TestRequestValidation(t *testing.T) {
	srv, fx := newServer(t)

	tests := []struct {
		name       string
		method     string
		url        string
		body       any
		wantStatus int
		wantCode   string
	}{
		{
			name: "check out before check in", method: http.MethodGet,
			url:        fmt.Sprintf("%s/v1/properties/%s/availability?check_in=2026-03-16&check_out=2026-03-14", srv.URL, fx.PropertyID),
			wantStatus: http.StatusBadRequest, wantCode: "invalid_stay",
		},
		{
			name: "missing dates", method: http.MethodGet,
			url:        fmt.Sprintf("%s/v1/properties/%s/availability", srv.URL, fx.PropertyID),
			wantStatus: http.StatusBadRequest, wantCode: "missing_parameter",
		},
		{
			name: "property is not a uuid", method: http.MethodGet,
			url:        srv.URL + "/v1/properties/not-a-uuid/availability?check_in=2026-03-14&check_out=2026-03-16",
			wantStatus: http.StatusBadRequest, wantCode: "invalid_id",
		},
		{
			name: "unknown reservation", method: http.MethodGet,
			url:        srv.URL + "/v1/reservations/ALB-ZZZZZZ",
			wantStatus: http.StatusNotFound, wantCode: "not_found",
		},
		{
			name: "unknown field in body", method: http.MethodPost,
			url:        srv.URL + "/v1/reservations",
			body:       map[string]any{"property_id": fx.PropertyID.String(), "nights": 3},
			wantStatus: http.StatusBadRequest, wantCode: "invalid_body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := do(t, tt.method, tt.url, tt.body)
			if status != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", status, tt.wantStatus, body)
			}
			if p := decodeInto[problemBody](t, body); p.Code != tt.wantCode {
				t.Fatalf("code = %q, want %q", p.Code, tt.wantCode)
			}
		})
	}
}

func doWithKey(t *testing.T, method, url string, body any, key string) (int, []byte) {
	t.Helper()

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, out
}
