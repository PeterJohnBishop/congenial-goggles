package server

import (
	"congenial-goggles/server/middlware"

	"github.com/gin-gonic/gin"
)

func AddPublicRoutes(r *gin.Engine) {
	r.Use(middlware.RateLimitMiddleware())
	r.GET("/", Hello())
	r.POST("/upload", Upload())
	r.POST("/download/direct", Download())
	r.POST("/download/url", DownloadURL())
	r.POST("/download/qr", DownloadQR())
}
