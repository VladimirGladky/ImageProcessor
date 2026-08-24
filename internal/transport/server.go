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

type ImageServiceInterface interface {
	CreateImageFile(filename string, size int64, file io.Reader) (models.ImageMeta, error)
	GetModifiedFile(id, variant string) (io.ReadCloser, models.ImageMeta, error)
	DeleteFile(id string) error
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

func (s *ImageServer) CreateImageTaskHandler() ginext.HandlerFunc {
	return func(c *gin.Context) {
		fileHeader, err := c.FormFile("image")
		if err != nil {
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

		meta, err := s.srv.CreateImageFile(fileHeader.Filename, fileHeader.Size, file)
		if err != nil {
			c.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusAccepted, meta)
	}
}

func (s *ImageServer) GetModifiedImageHandler() ginext.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		variant := c.DefaultQuery("variant", "resized")

		file, meta, err := s.srv.GetModifiedFile(id, variant)
		if err != nil {
			switch {
			case errors.Is(err, models.ErrNotFound):
				c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
			case errors.Is(err, models.ErrNotReady):
				c.JSON(http.StatusAccepted, gin.H{"id": id, "status": meta.Status})
			default:
				c.Error(err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		err := s.srv.DeleteFile(id)
		if err != nil {
			switch {
			case errors.Is(err, models.ErrNotFound):
				c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
			default:
				c.Error(err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		c.JSON(http.StatusNoContent, gin.H{"id": id})
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
