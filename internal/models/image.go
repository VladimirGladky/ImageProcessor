package models

import (
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("image not found")
	ErrNotReady      = errors.New("image is still processing")
	ErrProcessFailed = errors.New("image processing failed")
	ErrInvalidFormat = errors.New("unsupported image format")
	ErrFileTooLarge  = errors.New("file is too large")
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

type ImageMeta struct {
	Id           string      `json:"id"`
	OriginalName string      `json:"original_name"`
	Format       string      `json:"format"`
	Status       ImageStatus `json:"status"`
	Error        string      `json:"error,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
}
