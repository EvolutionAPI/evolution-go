package send_service

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// encodePNG builds a simple solid-color PNG of the given size for testing.
func encodePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 120, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test PNG: %v", err)
	}
	return buf.Bytes()
}

func TestMakeJPEGThumbnail_ResizesLandscape(t *testing.T) {
	src := encodePNG(t, 800, 400)

	thumb := makeJPEGThumbnail(src, 72)
	if thumb == nil {
		t.Fatal("expected a thumbnail, got nil")
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(thumb))
	if err != nil {
		t.Fatalf("thumbnail is not a decodable image: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("expected jpeg thumbnail, got %q", format)
	}
	if cfg.Width != 72 {
		t.Fatalf("expected width 72, got %d", cfg.Width)
	}
	// Aspect ratio (2:1) must be preserved: 72 wide -> 36 tall.
	if cfg.Height != 36 {
		t.Fatalf("expected height 36 to preserve aspect ratio, got %d", cfg.Height)
	}
}

func TestMakeJPEGThumbnail_DoesNotUpscaleSmallImages(t *testing.T) {
	src := encodePNG(t, 40, 40)

	thumb := makeJPEGThumbnail(src, 72)
	if thumb == nil {
		t.Fatal("expected a thumbnail, got nil")
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(thumb))
	if err != nil {
		t.Fatalf("thumbnail is not a decodable image: %v", err)
	}
	if cfg.Width != 40 || cfg.Height != 40 {
		t.Fatalf("expected the thumbnail to keep the original 40x40 size, got %dx%d", cfg.Width, cfg.Height)
	}
}

func TestMakeJPEGThumbnail_InvalidInputReturnsNil(t *testing.T) {
	if thumb := makeJPEGThumbnail([]byte("not an image"), 72); thumb != nil {
		t.Fatalf("expected nil for non-image input, got %d bytes", len(thumb))
	}
	if thumb := makeJPEGThumbnail(nil, 72); thumb != nil {
		t.Fatalf("expected nil for nil input, got %d bytes", len(thumb))
	}
}

func TestMakeJPEGThumbnail_DefaultsInvalidMaxWidth(t *testing.T) {
	src := encodePNG(t, 800, 800)

	thumb := makeJPEGThumbnail(src, 0)
	if thumb == nil {
		t.Fatal("expected a thumbnail with defaulted maxWidth, got nil")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(thumb))
	if err != nil {
		t.Fatalf("thumbnail is not a decodable image: %v", err)
	}
	if cfg.Width != 72 {
		t.Fatalf("expected defaulted width 72, got %d", cfg.Width)
	}
}

// ensure the jpeg encoder import stays referenced even if the helper changes.
var _ = jpeg.DefaultQuality
