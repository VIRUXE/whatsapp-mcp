package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 800, 800))
	for x := 0; x < 800; x++ {
		for y := 0; y < 800; y++ {
			img.Set(x, y, color.RGBA{200, 150, 0, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestBuildLinkPreview(t *testing.T) {
	pngData := testPNG(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/casos/1", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>x</title>
<meta property="og:title" content="#1 · Tour &amp; Guimarães">
<meta property="og:description" content="Tipo: Tour · 10 pax">
<meta property="og:image" content="/icon.png"></head></html>`))
	})
	mux.HandleFunc("/icon.png", func(w http.ResponseWriter, r *http.Request) { w.Write(pngData) })
	mux.HandleFunc("/gone", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	msg := buildLinkPreview("Ver caso: " + srv.URL + "/casos/1.")
	if msg == nil {
		t.Fatal("expected a preview")
	}
	if got := msg.GetTitle(); got != "#1 · Tour & Guimarães" {
		t.Errorf("title = %q", got)
	}
	if got := msg.GetDescription(); got != "Tipo: Tour · 10 pax" {
		t.Errorf("description = %q", got)
	}
	if got := msg.GetMatchedText(); got != srv.URL+"/casos/1" {
		t.Errorf("matched text = %q", got)
	}
	thumb, _, err := image.Decode(bytes.NewReader(msg.GetJPEGThumbnail()))
	if err != nil {
		t.Fatalf("thumbnail not decodable: %v", err)
	}
	if thumb.Bounds().Dx() != 300 || thumb.Bounds().Dy() != 300 {
		t.Errorf("thumbnail size = %v", thumb.Bounds())
	}
}

func TestBuildLinkPreviewNothingToShow(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	if buildLinkPreview("sem ligação") != nil {
		t.Error("no URL should give nil")
	}
	if buildLinkPreview(srv.URL+"/gone") != nil {
		t.Error("404 should give nil")
	}
	if buildLinkPreview("http://127.0.0.1:1/x") != nil {
		t.Error("unreachable should give nil")
	}
}
