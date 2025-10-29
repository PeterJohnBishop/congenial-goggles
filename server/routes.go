package server

import (
	"congenial-goggles/server/middlware"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/resend/resend-go/v2"
)

func AddPublicRoutes(client *dynamodb.Client, r *gin.Engine) {
	r.GET("/", Hello())
	r.POST("/register", CreateNewUserReq(client))
	r.POST("/login", AuthUserReq(client))
	r.POST("/refresh-token", middlware.RefreshTokenHandler(client))
}

func AddDProtectedRoutes(ddbClient *dynamodb.Client, resendClient *resend.Client, s3Client *s3.Client, r *gin.Engine) {
	auth := r.Group("/", middlware.AuthMiddleware())
	{
		auth.GET("/users", GetAllUsersReq(ddbClient))
		auth.GET("/users/:id", GetUserByIDReq(ddbClient))
		auth.PUT("/users", UpdateUserReq(ddbClient))
		auth.PUT("/users/password", UpdatePasswordReq(ddbClient))
		auth.DELETE("/users/:id", DeleteUserReq(ddbClient))
		auth.POST("/upload", Upload(s3Client))
		auth.POST("/download/direct", Download(s3Client))
		auth.POST("/download/url", DownloadURL(s3Client))
		auth.POST("/download/qr", DownloadQR(s3Client))
		auth.POST("/send_url", SendURLViaResend(resendClient))
		auth.POST("/send_qr", SendQRViaResend(resendClient))
	}
}
