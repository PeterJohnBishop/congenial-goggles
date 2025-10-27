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
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
)

type UserFile struct {
	UserID   string `dynamodbav:"userId"` // partition key
	FileID   string `dynamodbav:"fileId"` // sort key
	FileKey  string `dynamodbav:"fileKey"`
	Uploaded int64  `dynamodbav:"uploaded"`
}

func ConnectS3() (*s3.Client, error) {

	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	s3_region := os.Getenv("AWS_REGION_S3")

	s3Cfg, _ := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(s3_region),
		config.WithCredentialsProvider(
			aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		),
		//config.WithClientLogMode(aws.LogRequestWithBody|aws.LogResponseWithBody), <- for debugging
	)
	s3Client := s3.NewFromConfig(s3Cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	_, err := s3Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}
	return s3Client, nil
}

func StreamUploadFile(fileName string, fileContent multipart.File) error {
	bucketName := os.Getenv("AWS_BUCKET")
	client, err := ConnectS3()
	if err != nil {
		return err
	}
	fileKey := "uploads/" + fileName

	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileKey),
		Body:   fileContent,
	})
	if err != nil {
		return err
	}

	return nil
}

func StreamDownloadFile(c *gin.Context, fileName string) error {
	bucketName := os.Getenv("AWS_BUCKET")
	client, err := ConnectS3()
	if err != nil {
		return fmt.Errorf("failed to connect to S3: %w", err)
	}

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

func GeneratePresignedDownloadURL(fileKey string) (string, error) {
	bucketName := os.Getenv("AWS_BUCKET")
	client, err := ConnectS3()
	if err != nil {
		return "", fmt.Errorf("failed to connect to S3: %w", err)
	}

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
