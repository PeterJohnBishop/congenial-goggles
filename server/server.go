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

	dynamoClient, err := services.ConnectDB()
	if err != nil {
		log.Fatalf("Failed to connect to DynamoDB: %v", err)
	}
	err = services.CreateUsersTable(dynamoClient, "Users")
	if err != nil {
		log.Fatalf("Failed to create DynamoDB table: %v", err)
	}
	err = services.CreateFilesTable(dynamoClient, "Files")
	if err != nil {
		log.Fatalf("Failed to create DynamoDB table: %v", err)
	}
	AddPublicRoutes(dynamoClient, r)
	AddDProtectedRoutes(dynamoClient, r)

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
