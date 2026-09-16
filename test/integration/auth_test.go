//go:build integration

package integration

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestStaffEndpointsRequireAKey(t *testing.T) {
	srv, fx := newServer(t)

	inventoryURL := fmt.Sprintf("%s/v1/properties/%s/room-types/%s/inventory", srv.URL, fx.PropertyID, fx.RoomTypeID)
	listURL := fmt.Sprintf("%s/v1/properties/%s/reservations?from=2026-03-01&to=2026-04-01", srv.URL, fx.PropertyID)

	tests := []struct {
		name          string
		method        string
		url           string
		authorization string
		wantStatus    int
		wantCode      string
	}{
		{
			name: "no header", method: http.MethodPut, url: inventoryURL,
			wantStatus: http.StatusUnauthorized, wantCode: "missing_credentials",
		},
		{
			name: "not a bearer token", method: http.MethodPut, url: inventoryURL,
			authorization: "Basic " + testStaffKey,
			wantStatus:    http.StatusUnauthorized, wantCode: "missing_credentials",
		},
		{
			name: "wrong key", method: http.MethodGet, url: listURL,
			authorization: "Bearer not-the-key",
			wantStatus:    http.StatusUnauthorized, wantCode: "invalid_credentials",
		},
		{
			name: "right key, wrong case prefix", method: http.MethodGet, url: listURL,
			authorization: "bearer " + testStaffKey,
			wantStatus:    http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, tt.url, strings.NewReader(`{"from":"2026-03-14","to":"2026-03-20","allotment":1}`))
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("do: %v", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", resp.StatusCode, tt.wantStatus, body)
			}
			if tt.wantCode == "" {
				return
			}
			if p := decodeInto[problemBody](t, body); p.Code != tt.wantCode {
				t.Fatalf("code = %q, want %q", p.Code, tt.wantCode)
			}
			if got := resp.Header.Get("WWW-Authenticate"); got == "" {
				t.Error("401 without a WWW-Authenticate header")
			}
		})
	}
}

func TestGuestEndpointsAreOpen(t *testing.T) {
	srv, fx := newServer(t)

	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/v1/properties/%s/availability?check_in=2026-03-14&check_out=2026-03-16", srv.URL, fx.PropertyID), nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("availability without credentials = %d, want 200", resp.StatusCode)
	}
}
