package server

import (
	"congenial-goggles/server/middlware"
	"congenial-goggles/server/services"
	"log"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/resend/resend-go/v2"
)

type AppServices struct {
	ResendClient *resend.Client
	S3Client     *s3.Client
	DynamoClient *dynamodb.Client
}

func ServeGin() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(middlware.RateLimitMiddleware())

	var (
		appServices AppServices
		wg          sync.WaitGroup
		errChan     = make(chan error, 3)
	)

	wg.Add(3)

	go func() {
		defer wg.Done()
		appServices.ResendClient = services.InitResendClient()
		log.Println("Resend client initialized")
	}()

	go func() {
		defer wg.Done()
		s3Client, err := services.ConnectS3()
		if err != nil {
			errChan <- err
			return
		}
		appServices.S3Client = s3Client
		log.Println("S3 client initialized")
	}()

	go func() {
		defer wg.Done()
		ddbClient, err := services.ConnectDB()
		if err != nil {
			errChan <- err
			return
		}
		appServices.DynamoClient = ddbClient
		log.Println("DynamoDB client initialized")

		if err := services.CreateUsersTable(ddbClient, "Users"); err != nil {
			errChan <- err
			return
		}
		if err := services.CreateFilesTable(ddbClient, "Files"); err != nil {
			errChan <- err
			return
		}
		log.Println("DynamoDB tables created")
	}()

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			log.Fatalf("Initialization failed: %v", err)
		}
	}

	AddPublicRoutes(appServices.DynamoClient, r)
	AddDProtectedRoutes(appServices.DynamoClient, appServices.ResendClient, appServices.S3Client, r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server listening on :%s\n", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
