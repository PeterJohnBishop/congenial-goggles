package server

import "github.com/gin-gonic/gin"

func AddPublicRoutes(r *gin.Engine) {
	r.GET("/", Hello())
	r.POST("/upload", Upload())
	r.POST("/download", Download())
}
