package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestPNG(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 128, A: 255})
		}
	}
	path := filepath.Join(t.TempDir(), "test.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRenderInlineImageFitsBounds(t *testing.T) {
	path := writeTestPNG(t, 200, 100)
	out, err := renderInlineImage(path, 40, 10)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(out, "\n")
	if len(lines) > 10 {
		t.Fatalf("got %d rows, want <= 10", len(lines))
	}
	if cells := strings.Count(lines[0], "▀"); cells > 40 {
		t.Fatalf("got %d cols, want <= 40", cells)
	}
	if !strings.Contains(out, "[#") {
		t.Fatal("output has no tview color tags")
	}
}

func TestRenderInlineImageTiny(t *testing.T) {
	path := writeTestPNG(t, 1, 1)
	out, err := renderInlineImage(path, 40, 10)
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("empty output for 1x1 image")
	}
}

func TestRenderInlineImageBadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-an-image.jpg")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := renderInlineImage(path, 40, 10); err == nil {
		t.Fatal("expected decode error for non-image file")
	}
}
