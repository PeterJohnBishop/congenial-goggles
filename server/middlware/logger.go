package middlware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// Process request
		c.Next()

		// After request
		status := c.Writer.Status()
		duration := time.Since(start)

		log.Printf("%s %s | %d | %v\n", method, path, status, duration)
	}
}
