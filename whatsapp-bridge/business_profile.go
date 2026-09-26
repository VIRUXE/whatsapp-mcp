package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// BusinessProfileResponse answers "is this phone number a WhatsApp Business
// account, and what kind of business?" for callers that classify contacts.
type BusinessProfileResponse struct {
	Phone        string   `json:"phone"`
	OnWhatsApp   bool     `json:"on_whatsapp"`
	JID          string   `json:"jid,omitempty"`
	IsBusiness   bool     `json:"is_business"`
	VerifiedName string   `json:"verified_name,omitempty"`
	Categories   []string `json:"categories,omitempty"`
	Email        string   `json:"email,omitempty"`
	Address      string   `json:"address,omitempty"`
}

// buildBusinessProfileResponse folds an IsOnWhatsApp result and an optional
// business profile into the response. A non-nil VerifiedName is how WhatsApp
// marks a business account; the profile adds categories, email and address.
func buildBusinessProfileResponse(phone string, on types.IsOnWhatsAppResponse, profile *types.BusinessProfile) BusinessProfileResponse {
	resp := BusinessProfileResponse{Phone: phone, OnWhatsApp: on.IsIn}
	if !on.IsIn {
		return resp
	}
	resp.JID = on.JID.String()
	if on.VerifiedName != nil {
		resp.IsBusiness = true
		resp.VerifiedName = on.VerifiedName.Details.GetVerifiedName()
	}
	if profile != nil {
		resp.IsBusiness = true
		resp.Email = profile.Email
		resp.Address = profile.Address
		for _, c := range profile.Categories {
			if c.Name != "" {
				resp.Categories = append(resp.Categories, c.Name)
			}
		}
	}
	return resp
}

// normalizeLookupPhone keeps the digits of a phone number and prefixes "+",
// which is the form IsOnWhatsApp expects. Returns "" when nothing is left.
func normalizeLookupPhone(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := strings.TrimPrefix(b.String(), "00")
	if len(digits) < 6 {
		return ""
	}
	return "+" + digits
}

// registerBusinessProfileEndpoint adds GET /api/business-profile?phone=...
func registerBusinessProfileEndpoint(mux *http.ServeMux, auth func(http.HandlerFunc) http.HandlerFunc, client *whatsmeow.Client) {
	mux.HandleFunc("/api/business-profile", auth(func(w http.ResponseWriter, r *http.Request) {
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
		if len(results) == 0 {
			_ = json.NewEncoder(w).Encode(BusinessProfileResponse{Phone: phone})
			return
		}
		on := results[0]

		var profile *types.BusinessProfile
		if on.IsIn && on.VerifiedName != nil {
			// Categories are nice to have; a failed profile query still
			// leaves a valid "is a business" answer.
			if p, perr := client.GetBusinessProfile(ctx, on.JID); perr == nil {
				profile = p
			} else {
				fmt.Printf("business-profile %s: GetBusinessProfile failed: %v\n", on.JID, perr)
			}
		}
		_ = json.NewEncoder(w).Encode(buildBusinessProfileResponse(phone, on, profile))
	}))
}
