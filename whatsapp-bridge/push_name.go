package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
)

// registerPushNameEndpoint adds POST /api/push-name {"name": "..."}.
// It changes this account's own WhatsApp display name through an app-state
// patch, the same setting the phone writes under Settings > profile name.
func registerPushNameEndpoint(mux *http.ServeMux, auth func(http.HandlerFunc) http.HandlerFunc, client *whatsmeow.Client) {
	mux.HandleFunc("/api/push-name", auth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		writeErr := func(status int, msg string) {
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": msg})
		}
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(http.StatusBadRequest, "invalid JSON body")
			return
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			writeErr(http.StatusBadRequest, "name is required")
			return
		}
		if !client.IsConnected() {
			writeErr(http.StatusServiceUnavailable, "not connected to WhatsApp")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		previous := client.Store.PushName
		if err := client.SendAppState(ctx, appstate.BuildSettingPushName(name)); err != nil {
			writeErr(http.StatusBadGateway, fmt.Sprintf("SendAppState failed: %v", err))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "previous": previous, "name": name})
	}))
}
