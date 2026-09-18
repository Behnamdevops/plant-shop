package storage

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// validJPEG returns bytes for a genuine, tiny encoded JPEG image.
func validJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 50, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("failed to encode fixture JPEG: %v", err)
	}
	return buf.Bytes()
}

// validPNG returns bytes for a genuine, tiny encoded PNG image.
func validPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 20, G: 180, B: 20, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode fixture PNG: %v", err)
	}
	return buf.Bytes()
}

// validWebP is a minimal, genuine 1x1 lossless (VP8L) WebP fixture. There is
// no WebP encoder in the standard library or golang.org/x/image (decode
// only), so this well-known minimal byte sequence is used directly instead.
func validWebP() []byte {
	return []byte{
		0x52, 0x49, 0x46, 0x46, 0x24, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50,
		0x56, 0x50, 0x38, 0x4C, 0x18, 0x00, 0x00, 0x00, 0x2F, 0x00, 0x00, 0x00,
		0x10, 0x00, 0x00, 0x00, 0x00, 0x88, 0x88, 0x08, 0x29, 0x24, 0x22, 0x22,
		0x94, 0x02,
	}
}

func TestValidateImageAcceptsSupportedFormats(t *testing.T) {
	cases := []struct {
		name    string
		data    []byte
		wantExt string
	}{
		{"jpeg", validJPEG(t), ".jpg"},
		{"png", validPNG(t), ".png"},
		{"webp", validWebP(), ".webp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateImage(tc.data)
			if err != nil {
				t.Fatalf("ValidateImage failed: %v", err)
			}
			if got.Ext != tc.wantExt {
				t.Errorf("expected ext %q, got %q", tc.wantExt, got.Ext)
			}
		})
	}
}

func TestValidateImageRejectsEmptyFile(t *testing.T) {
	_, err := ValidateImage(nil)
	if err != ErrEmptyFile {
		t.Fatalf("expected ErrEmptyFile, got %v", err)
	}
}

func TestValidateImageRejectsOversizedFile(t *testing.T) {
	data := make([]byte, MaxImageBytes+1)
	_, err := ValidateImage(data)
	if err != ErrTooLarge {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}
}

func TestValidateImageRejectsFakeImageWithJPEGExtensionClaim(t *testing.T) {
	// Plain text content, as if a client renamed a .txt file to fake.jpg
	// and lied about the Content-Type. The extension/Content-Type must
	// never be trusted — only actual decodable content matters.
	data := []byte("this is not an image, just plain text pretending to be one")
	_, err := ValidateImage(data)
	if err != ErrUnsupportedFormat {
		t.Fatalf("expected ErrUnsupportedFormat, got %v", err)
	}
}

func TestValidateImageRejectsSVG(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	_, err := ValidateImage(svg)
	if err != ErrUnsupportedFormat {
		t.Fatalf("expected ErrUnsupportedFormat for SVG, got %v", err)
	}
}

func TestValidateImageRejectsHTML(t *testing.T) {
	html := []byte(`<!DOCTYPE html><html><body><script>alert(1)</script></body></html>`)
	_, err := ValidateImage(html)
	if err != ErrUnsupportedFormat {
		t.Fatalf("expected ErrUnsupportedFormat for HTML, got %v", err)
	}
}

func TestValidateImageRejectsExecutable(t *testing.T) {
	// Minimal ELF header magic bytes followed by junk.
	elf := append([]byte{0x7f, 'E', 'L', 'F'}, bytes.Repeat([]byte{0x00}, 64)...)
	_, err := ValidateImage(elf)
	if err != ErrUnsupportedFormat {
		t.Fatalf("expected ErrUnsupportedFormat for executable, got %v", err)
	}
}

func TestValidateImageRejectsGIF(t *testing.T) {
	// GIF89a header is not in our allow-list of registered decoders even
	// though Go's stdlib can decode GIF elsewhere; this package
	// deliberately never imports image/gif.
	gif := []byte("GIF89a")
	_, err := ValidateImage(gif)
	if err != ErrUnsupportedFormat {
		t.Fatalf("expected ErrUnsupportedFormat for GIF, got %v", err)
	}
}

func TestReadLimitedRejectsOversized(t *testing.T) {
	data := bytes.Repeat([]byte{0x01}, 100)
	_, err := ReadLimited(bytes.NewReader(data), 50)
	if err != ErrTooLarge {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}
}

func TestReadLimitedAllowsExactLimit(t *testing.T) {
	data := bytes.Repeat([]byte{0x01}, 50)
	got, err := ReadLimited(bytes.NewReader(data), 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 50 {
		t.Fatalf("expected 50 bytes, got %d", len(got))
	}
}
