package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestCropAnswerRegionUsesNormalizedBoundsAndPadding(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 100, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 100; x++ {
			source.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), A: 255})
		}
	}

	content, bounds, err := cropAnswerRegion(source, region{X: 0.2, Y: 0.25, Width: 0.3, Height: 0.2}, 5)
	if err != nil {
		t.Fatalf("cropAnswerRegion returned error: %v", err)
	}
	wantBounds := image.Rect(15, 45, 55, 95)
	if bounds != wantBounds {
		t.Fatalf("bounds = %v, want %v", bounds, wantBounds)
	}
	decoded, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("decode crop: %v", err)
	}
	if decoded.Bounds() != image.Rect(0, 0, 40, 50) {
		t.Fatalf("decoded bounds = %v", decoded.Bounds())
	}
	if got := decoded.At(0, 0); got != source.At(15, 45) {
		t.Fatalf("top-left pixel = %v, want %v", got, source.At(15, 45))
	}
}

func TestCropAnswerRegionClampsPaddingToImage(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 100, 200))
	_, bounds, err := cropAnswerRegion(source, region{X: 0, Y: 0, Width: 0.1, Height: 0.1}, 20)
	if err != nil {
		t.Fatalf("cropAnswerRegion returned error: %v", err)
	}
	if want := image.Rect(0, 0, 30, 40); bounds != want {
		t.Fatalf("bounds = %v, want %v", bounds, want)
	}
}

func TestCropAnswerRegionRejectsEmptyRegion(t *testing.T) {
	_, _, err := cropAnswerRegion(image.NewRGBA(image.Rect(0, 0, 10, 10)), region{}, 0)
	if err == nil {
		t.Fatal("cropAnswerRegion accepted an empty region")
	}
}
