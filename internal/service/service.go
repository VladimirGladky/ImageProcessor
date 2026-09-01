package service

import (
	"ImageProcessor/internal/models"
	"context"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ImageRepositoryInterface interface {
	SaveImage(ctx context.Context, meta models.ImageMeta, file io.Reader) error
	GetMeta(ctx context.Context, id string) (models.ImageMeta, error)
	OpenVariant(ctx context.Context, id, format string, variant models.ImageVariant) (io.ReadCloser, error)
	Delete(ctx context.Context, id string) error
}

type ImageService struct {
	repo ImageRepositoryInterface
}

func NewImageService(repo ImageRepositoryInterface) *ImageService {
	return &ImageService{
		repo: repo,
	}
}

func (s *ImageService) CreateImageFile(ctx context.Context, filename string, file io.Reader) (models.ImageMeta, error) {
	if filename == "" {
		return models.ImageMeta{}, models.ErrFilenameIsEmpty
	}
	if file == nil {
		return models.ImageMeta{}, models.ErrFileIsEmpty
	}
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	if format == "jpeg" {
		format = "jpg"
	}
	if format != "jpg" && format != "png" && format != "gif" {
		return models.ImageMeta{}, models.ErrInvalidFormat
	}
	meta := models.ImageMeta{
		Id:           uuid.NewString(),
		OriginalName: filename,
		Format:       format,
		Status:       models.StatusProcessing,
		CreatedAt:    time.Now().UTC(),
	}
	err := s.repo.SaveImage(ctx, meta, file)
	if err != nil {
		return models.ImageMeta{}, err
	}
	return meta, nil
}

func (s *ImageService) GetModifiedFile(ctx context.Context, id string, variant models.ImageVariant) (io.ReadCloser, models.ImageMeta, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, models.ImageMeta{}, models.ErrInvalidId
	}
	if !variant.IsValid() {
		return nil, models.ImageMeta{}, models.ErrInvalidVariant
	}
	meta, err := s.repo.GetMeta(ctx, id)
	if err != nil {
		return nil, models.ImageMeta{}, err
	}
	switch meta.Status {
	case models.StatusProcessing:
		return nil, meta, models.ErrNotReady
	case models.StatusFailed:
		return nil, meta, models.ErrProcessFailed
	}
	f, err := s.repo.OpenVariant(ctx, id, meta.Format, variant)
	if err != nil {
		return nil, meta, err
	}
	return f, meta, nil
}

func (s *ImageService) DeleteFile(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return models.ErrInvalidId
	}
	return s.repo.Delete(ctx, id)
}
