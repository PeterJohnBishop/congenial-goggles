package services

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
)

type UserFile struct {
	UserID   string `dynamodbav:"userId"` // partition key
	FileID   string `dynamodbav:"fileId"` // sort key
	FileKey  string `dynamodbav:"fileKey"`
	Uploaded int64  `dynamodbav:"uploaded"`
}

func StreamUploadFile(client *s3.Client, fileName string, fileContent multipart.File) error {
	bucketName := os.Getenv("AWS_BUCKET")

	fileKey := "uploads/" + fileName

	_, err := client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileKey),
		Body:   fileContent,
	})
	if err != nil {
		return err
	}

	return nil
}

func StreamDownloadFile(c *gin.Context, client *s3.Client, fileName string) error {
	bucketName := os.Getenv("AWS_BUCKET")

	fileKey := "uploads/" + fileName

	resp, err := client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileKey),
	})
	if err != nil {
		return fmt.Errorf("failed to get S3 object: %w", err)
	}
	defer resp.Body.Close()

	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Header("Content-Type", *resp.ContentType)
	c.Header("Content-Length", fmt.Sprintf("%d", resp.ContentLength))

	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		log.Println("Error streaming S3 object:", err)
		return fmt.Errorf("failed to stream file")
	}

	return nil
}

func GeneratePresignedDownloadURL(client *s3.Client, fileKey string) (string, error) {
	bucketName := os.Getenv("AWS_BUCKET")

	presignClient := s3.NewPresignClient(client)
	expiration := 5 * time.Minute

	req, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileKey),
	}, s3.WithPresignExpires(expiration))
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return req.URL, nil
}
