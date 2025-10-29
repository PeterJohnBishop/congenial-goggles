package services

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func ConnectDB() (*dynamodb.Client, error) {

	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	ddb_region := os.Getenv("AWS_REGION")

	ddbCfg, _ := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(ddb_region),
		config.WithCredentialsProvider(
			aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		),
		//config.WithClientLogMode(aws.LogRequestWithBody|aws.LogResponseWithBody),
	)
	ddbClient := dynamodb.NewFromConfig(ddbCfg)

	return ddbClient, nil
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
