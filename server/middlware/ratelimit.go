package middlware

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

var visitors = make(map[string]*rate.Limiter)
var mtx sync.Mutex

func getVisitor(ip string) *rate.Limiter {
	mtx.Lock()
	defer mtx.Unlock()

	limiter, exists := visitors[ip]
	if !exists {
		limiter = rate.NewLimiter(5.0/60.0, 5) // 5 tokens per 60 seconds, burst 5
		visitors[ip] = limiter
	}

	return limiter
}

func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := getVisitor(ip)

		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Try again later.",
			})
			return
		}

		c.Next()
	}
}
