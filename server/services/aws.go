package services

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
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
