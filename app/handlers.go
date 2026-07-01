package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/laraibg786/codeCurfew/internal/core"
)

// WebhookHandler is the HTTP driving adapter: it decodes an inbound webhook
// request into a provider-agnostic domain event (via the selected provider) and
// drives the core service with it. It holds no provider SDK types and no
// orchestration logic itself, and names no concrete adapter package — it works
// with any core.Provider.
type WebhookHandler struct {
	Provider core.Provider
	Service  *core.Service
}

func NewWebhookHandler(provider core.Provider, service *core.Service) *WebhookHandler {
	return &WebhookHandler{Provider: provider, Service: service}
}

func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) error {
	l := loggerFromRequest(r)

	event, err := h.Provider.DecodeWebhook(r)
	if err != nil {
		// A skipped event (unsupported action) is a successful no-op, matching the
		// original HandleWebhook behavior of returning nil.
		if errors.Is(err, core.ErrSkipEvent) {
			return nil
		}
		return err
	}

	return h.Service.HandlePullRequestUpdated(r.Context(), event, l)
}

func HandleHealth(w http.ResponseWriter, r *http.Request) error {
	var (
		up     string
		uptime = time.Since(startTime)
	)
	w.Header().Set("Content-Type", "application/json")
	if startTime.IsZero() {
		up = "unknown"
	} else {
		up = uptime.String()
	}
	return json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"uptime": up,
	})
}
