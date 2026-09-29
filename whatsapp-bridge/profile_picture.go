package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.mau.fi/whatsmeow"
)

// registerProfilePictureEndpoint adds GET /api/profile-picture?phone=...
// It answers with the URL of the contact's WhatsApp profile picture, or
// found:false when they have none or their privacy setting hides it from us.
func registerProfilePictureEndpoint(mux *http.ServeMux, auth func(http.HandlerFunc) http.HandlerFunc, client *whatsmeow.Client) {
	mux.HandleFunc("/api/profile-picture", auth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		phone := normalizeLookupPhone(r.URL.Query().Get("phone"))
		if phone == "" {
			http.Error(w, "phone is required", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		writeErr := func(status int, msg string) {
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": msg})
		}
		if !client.IsConnected() {
			writeErr(http.StatusServiceUnavailable, "not connected to WhatsApp")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		results, err := client.IsOnWhatsApp(ctx, []string{phone})
		if err != nil {
			writeErr(http.StatusBadGateway, fmt.Sprintf("IsOnWhatsApp failed: %v", err))
			return
		}
		if len(results) == 0 || !results[0].IsIn {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"phone": phone, "on_whatsapp": false, "found": false})
			return
		}

		info, err := client.GetProfilePictureInfo(ctx, results[0].JID, &whatsmeow.GetProfilePictureParams{})
		if err == whatsmeow.ErrProfilePictureNotSet || err == whatsmeow.ErrProfilePictureUnauthorized || (err == nil && info == nil) {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"phone": phone, "on_whatsapp": true, "found": false})
			return
		}
		if err != nil {
			writeErr(http.StatusBadGateway, fmt.Sprintf("GetProfilePictureInfo failed: %v", err))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"phone": phone, "on_whatsapp": true, "found": true, "url": info.URL, "id": info.ID})
	}))
}

