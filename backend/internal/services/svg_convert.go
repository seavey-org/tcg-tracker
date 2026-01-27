package services

import (
	"bytes"
	"image"
	"image/png"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// svgToPNG converts SVG data to PNG format.
// Returns the PNG bytes at the specified size (square).
// If size is 0, defaults to 128x128 pixels.
func svgToPNG(svgData []byte, size int) ([]byte, error) {
	if size <= 0 {
		size = 128
	}

	// Parse the SVG
	icon, err := oksvg.ReadIconStream(bytes.NewReader(svgData))
	if err != nil {
		return nil, err
	}

	// Get the SVG's native size
	w, h := icon.ViewBox.W, icon.ViewBox.H
	if w <= 0 || h <= 0 {
		w, h = float64(size), float64(size)
	}

	// Calculate scale to fit in target size while preserving aspect ratio
	scale := float64(size) / max(w, h)
	outW := int(w * scale)
	outH := int(h * scale)

	// Center the icon in the output
	offsetX := (size - outW) / 2
	offsetY := (size - outH) / 2

	// Set the target size
	icon.SetTarget(float64(offsetX), float64(offsetY), float64(outW), float64(outH))

	// Create the output image (RGBA for transparency support)
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Create rasterizer and render
	scanner := rasterx.NewScannerGV(size, size, img, img.Bounds())
	raster := rasterx.NewDasher(size, size, scanner)
	icon.Draw(raster, 1.0)

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
