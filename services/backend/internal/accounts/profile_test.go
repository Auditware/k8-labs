package accounts

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"testing"
)

// The website field is rendered as a clickable link, so the scheme check is a
// stored-XSS control, not formatting. javascript: and data: must never survive.
func TestValidateWebsite(t *testing.T) {
	cases := []struct {
		name    string
		website string
		wantErr bool
	}{
		{"empty is allowed - field is optional", "", false},
		{"https", "https://example.com", false},
		{"http", "http://example.com", false},
		{"https with path and query", "https://example.com/a/b?c=d", false},

		{"javascript scheme", "javascript:alert(1)", true},
		{"uppercase javascript scheme", "JavaScript:alert(1)", true},
		{"data scheme", "data:text/html,<script>alert(1)</script>", true},
		{"file scheme", "file:///etc/passwd", true},
		{"vbscript scheme", "vbscript:msgbox(1)", true},
		{"scheme-relative has no scheme", "//example.com", true},
		{"bare host has no scheme", "example.com", true},
		{"http with no host", "http://", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWebsite(tc.website)
			if tc.wantErr && err == nil {
				t.Errorf("validateWebsite(%q) = nil, want an error", tc.website)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("validateWebsite(%q) = %v, want nil", tc.website, err)
			}
		})
	}
}

func TestValidateWebsiteLengthLimit(t *testing.T) {
	long := "https://example.com/"
	for len(long) <= MaxWebsiteBytes {
		long += "a"
	}
	if err := validateWebsite(long); err == nil {
		t.Error("over-length website accepted, want rejected")
	}
}

// Avatars are served back from our own origin, so the type must come from the
// bytes rather than the uploader's claim. An SVG is a document that can carry
// script - accepting one would be stored XSS against everyone who views the
// profile.
func TestSniffAvatarType(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"png", []byte("\x89PNG\r\n\x1a\nrest-of-file"), "image/png"},
		{"jpeg", append([]byte{0xFF, 0xD8, 0xFF}, []byte("rest")...), "image/jpeg"},
		{"gif87a", []byte("GIF87a-rest-of-file"), "image/gif"},
		{"gif89a", []byte("GIF89a-rest-of-file"), "image/gif"},
		{"webp", []byte("RIFF____WEBPVP8 rest"), "image/webp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sniffAvatarType(tc.data)
			if err != nil {
				t.Fatalf("sniffAvatarType(%s) errored: %v", tc.name, err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			if !allowedAvatarTypes[got] {
				t.Errorf("%q sniffed to a type that isn't on the allowlist", tc.name)
			}
		})
	}
}

func TestSniffAvatarRejectsScriptableAndUnknown(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)},
		{"svg with leading whitespace", []byte("  \n<svg onload=\"alert(1)\"></svg>")},
		{"html", []byte("<!doctype html><script>alert(1)</script>")},
		{"empty", []byte{}},
		{"plain text", []byte("just some text")},
		{"truncated png magic", []byte("\x89PNG")},
		// A file claiming to be a PNG by name but carrying SVG content must
		// still be rejected - only the bytes are consulted.
		{"svg masquerading as png", []byte("<svg/>\x89PNG")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := sniffAvatarType(tc.data); err == nil {
				t.Errorf("accepted %s as %q, want rejected", tc.name, got)
			}
		})
	}
}

func TestAllowlistExcludesSVG(t *testing.T) {
	for _, banned := range []string{"image/svg+xml", "text/html", "application/xml", "image/svg"} {
		if allowedAvatarTypes[banned] {
			t.Errorf("%q is on the avatar allowlist - it can carry script and must not be", banned)
		}
	}
}

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// A 1x1 transparent PNG passes the magic-byte sniff, sets hasAvatar, and then
// renders as an empty circle on the leaderboard and profile instead of the
// initial. The dimension floor keeps those out.
func TestCheckAvatarDimensions(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"1x1 pixel", pngOf(t, 1, 1), ErrAvatarTooSmall},
		{"narrow strip", pngOf(t, MinAvatarPx-1, 200), ErrAvatarTooSmall},
		{"short strip", pngOf(t, 200, MinAvatarPx-1), ErrAvatarTooSmall},
		{"exactly the floor", pngOf(t, MinAvatarPx, MinAvatarPx), nil},
		{"normal avatar", pngOf(t, 256, 256), nil},
		{"magic bytes with a garbage body", []byte("\x89PNG\r\n\x1a\nrest-of-file"), ErrAvatarType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := checkAvatarDimensions(tc.data); !errors.Is(got, tc.want) {
				t.Errorf("checkAvatarDimensions = %v, want %v", got, tc.want)
			}
		})
	}
}
