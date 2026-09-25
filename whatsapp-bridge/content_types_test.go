package main

import (
	"strings"
	"testing"

	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestExtractTextContent_LocationAndContacts(t *testing.T) {
	loc := extractTextContent(&waProto.Message{LocationMessage: &waProto.LocationMessage{
		DegreesLatitude:  proto.Float64(41.14961),
		DegreesLongitude: proto.Float64(-8.61099),
		Name:             proto.String("Estação de São Bento"),
	}})
	if !strings.HasPrefix(loc, "[Location] 41.149610, -8.610990") || !strings.Contains(loc, "maps.google.com/?q=41.149610,-8.610990") || !strings.Contains(loc, "(Estação de São Bento)") {
		t.Errorf("location text = %q", loc)
	}

	card := extractTextContent(&waProto.Message{ContactMessage: &waProto.ContactMessage{
		DisplayName: proto.String("Ana Silva"),
		Vcard:       proto.String("BEGIN:VCARD\nFN:Ana Silva\nTEL;type=CELL;waid=351912345678:+351 912 345 678\nEND:VCARD"),
	}})
	if card != "[Contact] Ana Silva +351 912 345 678" {
		t.Errorf("contact text = %q", card)
	}
}
