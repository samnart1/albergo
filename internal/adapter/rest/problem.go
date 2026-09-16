package rest

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/samnart1/albergo/internal/domain/shared"
)

type problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
}

var titles = map[int]string{
	http.StatusBadRequest:          "Bad Request",
	http.StatusNotFound:            "Not Found",
	http.StatusConflict:            "Conflict",
	http.StatusUnprocessableEntity: "Unprocessable Content",
	http.StatusInternalServerError: "Internal Server Error",
	http.StatusUnauthorized:        "Unauthorized",
}

func statusFor(kind shared.Kind) int {
	switch kind {
	case shared.KindInvalid:
		return http.StatusBadRequest
	case shared.KindNotFound:
		return http.StatusNotFound
	case shared.KindConflict:
		return http.StatusConflict
	case shared.KindUnprocessable:
		return http.StatusUnprocessableEntity
	case shared.KindUnauthenticated:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

func writeProblem(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	kind := shared.KindOf(err)
	status := statusFor(kind)

	detail := shared.MessageOf(err)
	code := shared.CodeOf(err)

	if status == http.StatusInternalServerError {
		log.ErrorContext(r.Context(), "request failed", "error", err, "path", r.URL.Path)
		detail = "request could not be completed"
	}

	body := problem{Type: "about:blank", Title: titles[status], Status: status, Detail: detail, Code: code}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
