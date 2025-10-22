package services

import (
	"context"
	"mime/multipart"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

func UploadFile(filename string, fileContent multipart.File) (string, error) {
	bucketName := os.Getenv("AWS_BUCKET")
	client, err := ConnectS3()
	if err != nil {
		return "", err
	}
	presigner := s3.NewPresignClient(client)

	fileKey := "uploads/" + filename

	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileKey),
		Body:   fileContent,
	})
	if err != nil {
		return "", err
	}

	presignedReq, err := presigner.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileKey),
	}, s3.WithPresignExpires(15*time.Minute))
	if err != nil {
		return "", err
	}

	return presignedReq.URL, nil
}

func DownloadFile(filename string) (string, error) {
	bucketName := os.Getenv("AWS_BUCKET")
	client, err := ConnectS3()
	if err != nil {
		return "", err
	}
	presignClient := s3.NewPresignClient(client)

	fileKey := "uploads/" + filename
	expiration := time.Duration(5) * time.Minute

	presignedURL, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileKey),
	}, s3.WithPresignExpires(expiration))
	if err != nil {
		return "", err
	}

	return presignedURL.URL, nil
}
