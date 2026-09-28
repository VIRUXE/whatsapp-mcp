package main

import (
	"bytes"
	"html"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"google.golang.org/protobuf/proto"
)

// WhatsApp renders link preview cards from data the sender's device attaches to
// the message; recipients never fetch the URL. A bridge sending plain text
// therefore produces bare links, so the bridge builds the card itself.

const (
	linkPreviewUA         = "WhatsApp/2.2637.100 W"
	linkPreviewTimeout    = 5 * time.Second
	linkPreviewPageMax    = 512 << 10
	linkPreviewImageMax   = 3 << 20
	linkPreviewThumbEdge  = 300
	linkPreviewThumbLevel = 75
)

var (
	linkPreviewURLRe   = regexp.MustCompile(`https?://[^\s<>"']+`)
	linkPreviewMetaRe  = regexp.MustCompile(`(?is)<meta\s+[^>]*>`)
	linkPreviewAttrRe  = regexp.MustCompile(`(?is)([a-zA-Z:_-]+)\s*=\s*("([^"]*)"|'([^']*)')`)
	linkPreviewTitleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
)

// buildLinkPreview returns the message to send for text when its first URL
// yields Open Graph data, or nil when there is no URL or nothing usable, in
// which case the caller sends plain text as before.
func buildLinkPreview(text string) *waProto.ExtendedTextMessage {
	link := linkPreviewURLRe.FindString(text)
	if link == "" {
		return nil
	}
	link = strings.TrimRight(link, ".,;:!?)")

	client := &http.Client{Timeout: linkPreviewTimeout}
	page, base, ok := fetchLimited(client, link, linkPreviewPageMax)
	if !ok {
		return nil
	}

	og := parseOpenGraph(string(page))
	if og["og:title"] == "" {
		og["og:title"] = og["title"]
	}
	if og["og:title"] == "" {
		return nil
	}

	msg := &waProto.ExtendedTextMessage{
		Text:        proto.String(text),
		MatchedText: proto.String(link),
		Title:       proto.String(og["og:title"]),
		PreviewType: waProto.ExtendedTextMessage_NONE.Enum(),
	}
	if d := og["og:description"]; d != "" {
		msg.Description = proto.String(d)
	}
	if img := og["og:image"]; img != "" {
		if ref, err := base.Parse(img); err == nil {
			if data, _, ok := fetchLimited(client, ref.String(), linkPreviewImageMax); ok {
				if thumb := makeThumbnail(data); thumb != nil {
					msg.JPEGThumbnail = thumb
				}
			}
		}
	}
	return msg
}

func fetchLimited(client *http.Client, target string, max int64) ([]byte, *url.URL, bool) {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, nil, false
	}
	req.Header.Set("User-Agent", linkPreviewUA)
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, false
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, max))
	if err != nil {
		return nil, nil, false
	}
	return data, resp.Request.URL, true
}

// parseOpenGraph collects og:* and twitter:* meta content plus the <title>.
func parseOpenGraph(doc string) map[string]string {
	out := map[string]string{}
	for _, tag := range linkPreviewMetaRe.FindAllString(doc, -1) {
		attrs := map[string]string{}
		for _, m := range linkPreviewAttrRe.FindAllStringSubmatch(tag, -1) {
			v := m[3]
			if v == "" {
				v = m[4]
			}
			attrs[strings.ToLower(m[1])] = html.UnescapeString(v)
		}
		key := strings.ToLower(attrs["property"])
		if key == "" {
			key = strings.ToLower(attrs["name"])
		}
		if key != "" && attrs["content"] != "" {
			if _, seen := out[key]; !seen {
				out[key] = strings.TrimSpace(attrs["content"])
			}
		}
	}
	if m := linkPreviewTitleRe.FindStringSubmatch(doc); m != nil {
		out["title"] = strings.TrimSpace(html.UnescapeString(m[1]))
	}
	return out
}

// makeThumbnail decodes an image, scales it to fit linkPreviewThumbEdge and
// re-encodes it as JPEG. It returns nil for anything it cannot decode.
func makeThumbnail(data []byte) []byte {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return nil
	}
	if w > linkPreviewThumbEdge || h > linkPreviewThumbEdge {
		if w >= h {
			h, w = h*linkPreviewThumbEdge/w, linkPreviewThumbEdge
		} else {
			w, h = w*linkPreviewThumbEdge/h, linkPreviewThumbEdge
		}
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: linkPreviewThumbLevel}); err != nil {
		return nil
	}
	return buf.Bytes()
}
