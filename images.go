package main

import (
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	"github.com/normen/whatscli/config"
)

// inlineImagesSupported is probed once at startup: half-block rendering needs a
// terminal with at least 256 colors, otherwise the result is unreadable noise
// and the clickable "open in native viewer" fallback is better.
var inlineImagesSupported = detectInlineImageSupport()

func detectInlineImageSupport() bool {
	colorTerm := os.Getenv("COLORTERM")
	if strings.Contains(colorTerm, "truecolor") || strings.Contains(colorTerm, "24bit") {
		return true
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "iTerm.app", "WezTerm", "ghostty", "vscode", "Hyper":
		return true
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	term := os.Getenv("TERM")
	return strings.Contains(term, "kitty") ||
		strings.Contains(term, "direct") ||
		strings.Contains(term, "256color")
}

func canRenderInlineImages() bool {
	return config.Config.General.InlineImages && inlineImagesSupported
}

// renderInlineImage decodes an image file and returns it as tview-tagged
// half-block text (one ▀ per cell, foreground = top pixel, background = bottom
// pixel), fitting maxCols x maxRows cells while preserving the aspect ratio.
func renderInlineImage(path string, maxCols, maxRows int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return "", err
	}

	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	if srcW < 1 || srcH < 1 {
		return "", errors.New("empty image")
	}
	if maxCols < 4 {
		maxCols = 4
	}
	if maxRows < 2 {
		maxRows = 2
	}

	// target pixel grid: 1 cell = 1 px wide x 2 px tall, never upscale
	maxPxH := maxRows * 2
	scale := float64(maxCols) / float64(srcW)
	if s := float64(maxPxH) / float64(srcH); s < scale {
		scale = s
	}
	if scale > 1 {
		scale = 1
	}
	w := int(float64(srcW)*scale + 0.5)
	if w < 1 {
		w = 1
	}
	h := int(float64(srcH)*scale + 0.5)
	if h%2 != 0 {
		h++
	}
	if h < 2 {
		h = 2
	}

	var sb strings.Builder
	for ty := 0; ty < h; ty += 2 {
		for tx := 0; tx < w; tx++ {
			tr, tg, tb := avgPixel(img, bounds, tx, ty, w, h)
			br, bg, bb := avgPixel(img, bounds, tx, ty+1, w, h)
			fmt.Fprintf(&sb, "[#%02x%02x%02x:#%02x%02x%02x]▀", tr, tg, tb, br, bg, bb)
		}
		sb.WriteString("[-:-:-]\n")
	}
	return strings.TrimSuffix(sb.String(), "\n"), nil
}

// avgPixel box-averages the source rectangle that maps onto target pixel (tx, ty).
func avgPixel(img image.Image, bounds image.Rectangle, tx, ty, w, h int) (uint8, uint8, uint8) {
	srcW, srcH := bounds.Dx(), bounds.Dy()
	x0 := bounds.Min.X + tx*srcW/w
	x1 := bounds.Min.X + (tx+1)*srcW/w
	y0 := bounds.Min.Y + ty*srcH/h
	y1 := bounds.Min.Y + (ty+1)*srcH/h
	if x1 <= x0 {
		x1 = x0 + 1
	}
	if y1 <= y0 {
		y1 = y0 + 1
	}
	if x1 > bounds.Max.X {
		x1 = bounds.Max.X
	}
	if y1 > bounds.Max.Y {
		y1 = bounds.Max.Y
	}
	if x1 <= x0 || y1 <= y0 {
		return 0, 0, 0
	}

	var r, g, b, n uint64
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			pr, pg, pb, _ := img.At(x, y).RGBA() // premultiplied: transparency blends to black
			r += uint64(pr >> 8)
			g += uint64(pg >> 8)
			b += uint64(pb >> 8)
			n++
		}
	}
	return uint8(r / n), uint8(g / n), uint8(b / n)
}
