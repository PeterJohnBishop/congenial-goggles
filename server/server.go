package server

import (
	"congenial-goggles/server/middlware"
	"congenial-goggles/server/services"
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

func ServeGin() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(middlware.RateLimitMiddleware())

	resendClient := services.InitResendClient()
	s3Client, err := services.ConnectS3()
	if err != nil {
		log.Fatalf("Failed to connect to S3: %v", err)
	}
	ddbClient, err := services.ConnectDB()
	if err != nil {
		log.Fatalf("Failed to connect to DynamoDB: %v", err)
	}
	err = services.CreateUsersTable(ddbClient, "Users")
	if err != nil {
		log.Fatalf("Failed to create DynamoDB table: %v", err)
	}
	err = services.CreateFilesTable(ddbClient, "Files")
	if err != nil {
		log.Fatalf("Failed to create DynamoDB table: %v", err)
	}
	AddPublicRoutes(ddbClient, r)
	AddDProtectedRoutes(ddbClient, resendClient, s3Client, r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server listening on :%s\n", port)
	err = r.Run(":" + port)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
