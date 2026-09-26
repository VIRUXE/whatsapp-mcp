package main

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waVnameCert"
	"go.mau.fi/whatsmeow/types"
)

func TestNormalizeLookupPhone(t *testing.T) {
	cases := map[string]string{
		"+351 918 593 067": "+351918593067",
		"00351918593067":   "+351918593067",
		"351918593067":     "+351918593067",
		"12":               "",
		"":                 "",
	}
	for in, want := range cases {
		if got := normalizeLookupPhone(in); got != want {
			t.Errorf("normalizeLookupPhone(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildBusinessProfileResponse(t *testing.T) {
	jid := types.NewJID("351229448041", types.DefaultUserServer)

	notOn := buildBusinessProfileResponse("+351229448041", types.IsOnWhatsAppResponse{IsIn: false}, nil)
	if notOn.OnWhatsApp || notOn.IsBusiness || notOn.JID != "" {
		t.Errorf("not on WhatsApp: got %+v", notOn)
	}

	person := buildBusinessProfileResponse("+351229448041", types.IsOnWhatsAppResponse{IsIn: true, JID: jid}, nil)
	if !person.OnWhatsApp || person.IsBusiness {
		t.Errorf("plain account: got %+v", person)
	}

	name := "A Nogueira da Costa"
	biz := buildBusinessProfileResponse("+351229448041", types.IsOnWhatsAppResponse{
		IsIn:         true,
		JID:          jid,
		VerifiedName: &types.VerifiedName{Details: &waVnameCert.VerifiedNameCertificate_Details{VerifiedName: &name}},
	}, &types.BusinessProfile{Email: "geral@x.pt", Categories: []types.Category{{ID: "1", Name: "Transportation Service"}, {ID: "2"}}})
	if !biz.IsBusiness || biz.VerifiedName != name || biz.Email != "geral@x.pt" || len(biz.Categories) != 1 || biz.Categories[0] != "Transportation Service" {
		t.Errorf("business: got %+v", biz)
	}
}
