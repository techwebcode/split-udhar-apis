package controllers

import (
	"net/http"
	"split-udhar-apis/services/storage"
	"strings"

	"github.com/gin-gonic/gin"
)

type ImageController struct {
	Storage storage.ImageStorageService
}

func NewImageController(s storage.ImageStorageService) *ImageController {
	if s == nil {
		s = storage.NewStorageServiceFromEnv()
	}
	return &ImageController{Storage: s}
}

func (i *ImageController) ServeImage(c *gin.Context) {
	key := strings.TrimPrefix(c.Param("key"), "/")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key is required"})
		return
	}

	data, contentType, err := i.Storage.Download(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
		return
	}

	if contentType == "" {
		contentType = "image/jpeg"
	}

	// Cache in client browser for 7 days
	c.Header("Cache-Control", "public, max-age=604800, immutable")
	c.Data(http.StatusOK, contentType, data)
}
