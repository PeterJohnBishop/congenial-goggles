package server

import (
	"congenial-goggles/server/middlware"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/gin-gonic/gin"
)

func AddPublicRoutes(client *dynamodb.Client, r *gin.Engine) {
	r.GET("/", Hello())
	r.POST("/register", CreateNewUserReq(client))
	r.POST("/login", AuthUserReq(client))
	r.POST("/refresh-token", middlware.RefreshTokenHandler(client))
}

func AddDProtectedRoutes(client *dynamodb.Client, r *gin.Engine) {
	auth := r.Group("/", middlware.AuthMiddleware())
	{
		auth.GET("/users", GetAllUsersReq(client))
		auth.GET("/users/:id", GetUserByIDReq(client))
		auth.PUT("/users", UpdateUserReq(client))
		auth.PUT("/users/password", UpdatePasswordReq(client))
		auth.DELETE("/users/:id", DeleteUserReq(client))
		auth.POST("/upload", Upload())
		auth.POST("/download/direct", Download())
		auth.POST("/download/url", DownloadURL())
		auth.POST("/download/qr", DownloadQR())
	}
}
