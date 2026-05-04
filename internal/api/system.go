package api

import (
	"allinonekey/internal/config"

	"github.com/gin-gonic/gin"
)

type SystemHandler struct{}

func (h *SystemHandler) Info(c *gin.Context) {
	c.JSON(200, gin.H{"version": config.AppVersion()})
}
