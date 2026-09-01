package transport

import (
	"ImageProcessor/internal/models"
	"ImageProcessor/pkg/logger"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wb-go/wbf/config"
	"github.com/wb-go/wbf/ginext"
	"go.uber.org/zap"
)

const maxUploadSize = 10 << 20

type ImageServiceInterface interface {
	CreateImageFile(ctx context.Context, filename string, file io.Reader) (models.ImageMeta, error)
	GetModifiedFile(ctx context.Context, id string, variant models.ImageVariant) (io.ReadCloser, models.ImageMeta, error)
	DeleteFile(ctx context.Context, id string) error
}

type ImageServer struct {
	ctx context.Context
	cfg *config.Config
	srv ImageServiceInterface
}

func NewImageServer(ctx context.Context, cfg *config.Config, srv ImageServiceInterface) *ImageServer {
	return &ImageServer{
		ctx: ctx,
		cfg: cfg,
		srv: srv,
	}
}

func (s *ImageServer) Run() error {
	eng := ginext.New("release")
	eng.Use(ginext.Logger())
	eng.Use(s.injectLogger())
	eng.Use(s.errorLogger())

	v1 := eng.Group("/api/v1")
	v1.POST("/upload", s.CreateImageTaskHandler())
	v1.GET("/image/:id", s.GetModifiedImageHandler())
	v1.DELETE("/image/:id", s.DeleteImageHandler())

	return eng.Run(s.cfg.GetString("HOST") + ":" + s.cfg.GetString("PORT"))
}

func (s *ImageServer) errorLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.GetLoggerFromCtx(s.ctx).Error("panic recovered",
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
					zap.Any("panic", rec),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			}
		}()

		c.Next()

		if c.Writer.Status() >= http.StatusInternalServerError && len(c.Errors) > 0 {
			logger.GetLoggerFromCtx(s.ctx).Error("request failed",
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.Int("status", c.Writer.Status()),
				zap.Error(c.Errors.Last().Err),
			)
		}
	}
}

func (s *ImageServer) injectLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), logger.Key, logger.GetLoggerFromCtx(s.ctx))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func (s *ImageServer) CreateImageTaskHandler() ginext.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

		fileHeader, err := c.FormFile("image")
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file is too large"})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			c.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot open file"})
			return
		}
		defer file.Close()

		meta, err := s.srv.CreateImageFile(c.Request.Context(), fileHeader.Filename, file)
		if err != nil {
			switch {
			case errors.Is(err, models.ErrInvalidFormat),
				errors.Is(err, models.ErrFilenameIsEmpty),
				errors.Is(err, models.ErrFileIsEmpty):
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			default:
				c.Error(err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			}
			return
		}

		c.JSON(http.StatusAccepted, meta)
	}
}

func (s *ImageServer) GetModifiedImageHandler() ginext.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		variant := models.ImageVariant(c.DefaultQuery("variant", string(models.VariantResized)))

		file, meta, err := s.srv.GetModifiedFile(c.Request.Context(), id, variant)
		if err != nil {
			switch {
			case errors.Is(err, models.ErrNotFound):
				c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
			case errors.Is(err, models.ErrNotReady):
				c.JSON(http.StatusAccepted, gin.H{"id": id, "status": meta.Status})
			case errors.Is(err, models.ErrProcessFailed):
				c.JSON(http.StatusUnprocessableEntity, gin.H{"id": id, "status": meta.Status, "error": meta.Error})
			case errors.Is(err, models.ErrInvalidId), errors.Is(err, models.ErrInvalidVariant):
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			default:
				c.Error(err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			}
			return
		}
		defer file.Close()

		c.DataFromReader(http.StatusOK, -1, contentTypeFor(meta.Format), file, nil)
	}
}

func (s *ImageServer) DeleteImageHandler() ginext.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		err := s.srv.DeleteFile(c.Request.Context(), id)
		if err != nil {
			switch {
			case errors.Is(err, models.ErrNotFound):
				c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
			case errors.Is(err, models.ErrInvalidId):
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			default:
				c.Error(err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			}
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func contentTypeFor(format string) string {
	switch strings.ToLower(format) {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}
