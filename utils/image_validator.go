package utils

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
)

const (
	// MaxImageSizeBytes is the maximum allowed image file size (5 MB).
	MaxImageSizeBytes = 5 * 1024 * 1024
)

// ValidatedImage contains the validated metadata of an uploaded image.
type ValidatedImage struct {
	Extension   string // e.g., ".jpg", ".png", ".webp"
	ContentType string // e.g., "image/jpeg", "image/png", "image/webp"
}

// ValidateImage validates that data represents an authentic, non-corrupted image of supported format and size.
func ValidateImage(data []byte) (*ValidatedImage, error) {
	if len(data) == 0 {
		return nil, errors.New("empty image file")
	}

	if len(data) > MaxImageSizeBytes {
		return nil, fmt.Errorf("image exceeds maximum allowed size of 5 MB (size: %.2f MB)", float64(len(data))/(1024*1024))
	}

	// 1. Magic bytes check
	contentType := http.DetectContentType(data)
	var ext string

	switch {
	case isJPEG(data):
		contentType = "image/jpeg"
		ext = ".jpg"
	case isPNG(data):
		contentType = "image/png"
		ext = ".png"
	case isWebP(data):
		contentType = "image/webp"
		ext = ".webp"
	default:
		return nil, fmt.Errorf("unsupported image format: %s. Only JPEG, PNG, and WebP are supported", contentType)
	}

	// 2. Decode configuration to ensure header integrity and non-zero dimensions
	if ext == ".webp" {
		// Verify WebP chunk structure
		if err := validateWebPHeader(data); err != nil {
			return nil, fmt.Errorf("corrupted WebP image: %w", err)
		}
	} else {
		// JPEG and PNG standard decoder validation
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("corrupted image file: %w", err)
		}
		if config.Width <= 0 || config.Height <= 0 {
			return nil, errors.New("invalid image dimensions")
		}
	}

	return &ValidatedImage{
		Extension:   ext,
		ContentType: contentType,
	}, nil
}

func isJPEG(data []byte) bool {
	return len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF
}

func isPNG(data []byte) bool {
	return len(data) >= 8 &&
		data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 &&
		data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A
}

func isWebP(data []byte) bool {
	return len(data) >= 12 &&
		string(data[0:4]) == "RIFF" &&
		string(data[8:12]) == "WEBP"
}

func validateWebPHeader(data []byte) error {
	if len(data) < 16 {
		return errors.New("file is too short for WebP header")
	}
	chunkType := string(data[12:16])
	if chunkType != "VP8 " && chunkType != "VP8L" && chunkType != "VP8X" {
		return fmt.Errorf("unrecognized WebP chunk format: %s", chunkType)
	}
	return nil
}
