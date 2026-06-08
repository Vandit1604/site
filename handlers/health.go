package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ShowHealth is a lightweight liveness/readiness endpoint for container and
// platform health checks. Returns 200 with a small JSON body.
func ShowHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
