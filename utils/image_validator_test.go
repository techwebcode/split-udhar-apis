package utils

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func createTestPNG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func createTestJPEG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{B: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

func TestValidateImage_ValidFormats(t *testing.T) {
	// Valid PNG
	pngData := createTestPNG(50, 50)
	metaPNG, err := ValidateImage(pngData)
	if err != nil {
		t.Fatalf("expected valid PNG, got error: %v", err)
	}
	if metaPNG.ContentType != "image/png" || metaPNG.Extension != ".png" {
		t.Errorf("unexpected PNG metadata: %+v", metaPNG)
	}

	// Valid JPEG
	jpegData := createTestJPEG(100, 80)
	metaJPEG, err := ValidateImage(jpegData)
	if err != nil {
		t.Fatalf("expected valid JPEG, got error: %v", err)
	}
	if metaJPEG.ContentType != "image/jpeg" || metaJPEG.Extension != ".jpg" {
		t.Errorf("unexpected JPEG metadata: %+v", metaJPEG)
	}
}

func TestValidateImage_InvalidMagicBytes(t *testing.T) {
	// Plain text or random string
	fakeData := []byte("<html><body>Hello not an image</body></html>")
	_, err := ValidateImage(fakeData)
	if err == nil {
		t.Fatalf("expected error for text file disguised as image, got nil")
	}

	// PDF magic bytes (%PDF)
	pdfData := []byte("%PDF-1.5 some pdf data here")
	_, err = ValidateImage(pdfData)
	if err == nil {
		t.Fatalf("expected error for PDF file, got nil")
	}
}

func TestValidateImage_SizeLimits(t *testing.T) {
	// Zero bytes
	_, err := ValidateImage([]byte{})
	if err == nil {
		t.Fatalf("expected error for empty file, got nil")
	}

	// Oversized (> 5MB)
	oversizedBytes := make([]byte, 5*1024*1024+1)
	_, err = ValidateImage(oversizedBytes)
	if err == nil {
		t.Fatalf("expected error for oversized image, got nil")
	}
}
