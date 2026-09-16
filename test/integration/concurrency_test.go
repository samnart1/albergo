//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"uuid"

	"github.com/samnart1/albergo/test/contract"
)

func publish(t *testing.T, srv string, fx contract.Fixture, allotment int) {
	t.Helper()

	status, body := do(t, http.MethodPut,
		fmt.Sprintf("%s/v1/properties/%s/room-types/%s/inventory", srv, fx.PropertyID, fx.RoomTypeID),
		map[string]any{"from": "2026-03-14", "to": "2026-03-17", "allotment": allotment})
	if status != http.StatusNoContent {
		t.Fatalf("set inventory = %d %s", status, body)
	}

	rates := []map[string]any{}
	for _, d := range []string{"2026-03-14", "2026-03-15", "2026-03-16"} {
		rates = append(rates, map[string]any{"date": d, "price_cents": 12_000, "min_stay": 1})
	}

	status, body = do(t, http.MethodPut,
		fmt.Sprintf("%s/v1/properties/%s/rate-plans/%s/rates", srv, fx.PropertyID, fx.RatePlanID),
		map[string]any{"rates": rates})
	if status != http.StatusNoContent {
		t.Fatalf("set rates = %d %s", status, body)
	}
}

func bookingBody(fx contract.Fixture) map[string]any {
	return map[string]any{
		"property_id":  fx.PropertyID.String(),
		"room_type_id": fx.RoomTypeID.String(),
		"rate_plan_id": fx.RatePlanID.String(),
		"check_in":     "2026-03-14",
		"check_out":    "2026-03-16",
		"guests":       2,
		"guest":        map[string]string{"email": "sam@example.com", "full_name": "Sam T", "phone": ""},
	}
}

func TestNoOversellUnderConcurrency(t *testing.T) {
	srv, fx := newServer(t)
	publish(t, srv.URL, fx, 1)

	const attempts = 50

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		created int
		soldOut int
		others  []string
		start   = make(chan struct{})
	)

	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start

			status, body := do(t, http.MethodPost, srv.URL+"/v1/reservations", bookingBody(fx))

			mu.Lock()
			defer mu.Unlock()

			switch status {
			case http.StatusCreated:
				created++
			case http.StatusConflict:
				if decodeInto[problemBody](t, body).Code == "sold_out" {
					soldOut++
					return
				}
				others = append(others, string(body))
			default:
				others = append(others, fmt.Sprintf("%d %s", status, body))
			}
		}()
	}

	close(start)
	wg.Wait()

	if created != 1 {
		t.Fatalf("created = %d, want exactly 1", created)
	}
	if soldOut != attempts-1 {
		t.Fatalf("sold out = %d, want %d (other outcomes: %v)", soldOut, attempts-1, others)
	}

	var booked int
	const q = `SELECT booked FROM inventory WHERE room_type_id = $1 AND stay_date = '2026-03-14'`
	if err := pool.QueryRow(context.Background(), q, fx.RoomTypeID).Scan(&booked); err != nil {
		t.Fatalf("read inventory: %v", err)
	}
	if booked != 1 {
		t.Fatalf("booked = %d, want 1", booked)
	}
}

func TestIdempotentUnderConcurrency(t *testing.T) {
	srv, fx := newServer(t)
	publish(t, srv.URL, fx, 5)

	const attempts = 20
	key := uuid.New().String()

	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		references = map[string]int{}
		failures   []string
		start      = make(chan struct{})
	)

	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start

			status, body := doWithKey(t, http.MethodPost, srv.URL+"/v1/reservations", bookingBody(fx), key)

			mu.Lock()
			defer mu.Unlock()

			if status != http.StatusCreated {
				failures = append(failures, fmt.Sprintf("%d %s", status, body))
				return
			}
			references[decodeInto[struct {
				Reference string `json:"reference"`
			}](t, body).Reference]++
		}()
	}

	close(start)
	wg.Wait()

	if len(failures) > 0 {
		t.Fatalf("%d requests failed: %v", len(failures), failures)
	}
	if len(references) != 1 {
		t.Fatalf("references = %v, want exactly one", references)
	}

	var count, booked int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM reservations WHERE property_id = $1`, fx.PropertyID).Scan(&count); err != nil {
		t.Fatalf("count reservations: %v", err)
	}
	if count != 1 {
		t.Fatalf("reservations = %d, want 1", count)
	}

	if err := pool.QueryRow(context.Background(),
		`SELECT booked FROM inventory WHERE room_type_id = $1 AND stay_date = '2026-03-14'`,
		fx.RoomTypeID).Scan(&booked); err != nil {
		t.Fatalf("read inventory: %v", err)
	}
	if booked != 1 {
		t.Fatalf("booked = %d, want 1", booked)
	}
}
