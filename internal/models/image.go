package models

import (
	"errors"
	"time"
)

var (
	ErrNotFound        = errors.New("image not found")
	ErrNotReady        = errors.New("image is still processing")
	ErrProcessFailed   = errors.New("image processing failed")
	ErrInvalidFormat   = errors.New("unsupported image format")
	ErrFilenameIsEmpty = errors.New("filename is empty")
	ErrInvalidVariant  = errors.New("invalid variant")
	ErrFileIsEmpty     = errors.New("file is empty")
	ErrInvalidId       = errors.New("invalid id")
)

type ImageTask struct {
	Id     int    `json:"id"`
	Action string `json:"action"`
}

type ImageStatus string

const (
	StatusProcessing ImageStatus = "processing"
	StatusDone       ImageStatus = "done"
	StatusFailed     ImageStatus = "failed"
)

type ImageVariant string

const (
	VariantResized     ImageVariant = "resized"
	VariantWatermarked ImageVariant = "watermarked"
	VariantThumbnail   ImageVariant = "thumbnail"
)

func (v ImageVariant) IsValid() bool {
	switch v {
	case VariantResized, VariantWatermarked, VariantThumbnail:
		return true
	default:
		return false
	}
}

type ImageMeta struct {
	Id           string      `json:"id"`
	OriginalName string      `json:"original_name"`
	Format       string      `json:"format"`
	Status       ImageStatus `json:"status"`
	Error        string      `json:"error,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
}
