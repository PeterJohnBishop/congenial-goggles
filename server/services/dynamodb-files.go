package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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
		config.WithClientLogMode(aws.LogRequestWithBody|aws.LogResponseWithBody),
	)
	ddbClient := dynamodb.NewFromConfig(ddbCfg)

	return ddbClient, nil
}

func CreateFilesTable(client *dynamodb.Client) error {
	tableName := "Files"

	_, err := client.DescribeTable(context.TODO(), &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err == nil {
		fmt.Printf("%s table already exists\n", tableName)
		return nil
	}

	var notFound *types.ResourceNotFoundException
	if !errors.As(err, &notFound) {
		return fmt.Errorf("error checking table existence: %w", err)
	}

	fmt.Println("Files table not found — creating now...")

	_, err = client.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("fileId"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("fileId"),
				KeyType:       types.KeyTypeHash,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		return fmt.Errorf("failed to create Files table: %w", err)
	}

	waiter := dynamodb.NewTableExistsWaiter(client)
	err = waiter.Wait(context.TODO(), &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	}, 2*time.Minute)
	if err != nil {
		return fmt.Errorf("failed waiting for Files table to become active: %w", err)
	}

	fmt.Println("Files table created and active.")
	return nil
}

func CreateFile(client *dynamodb.Client, tableName, fileId, fileName string) error {
	_, err := client.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]types.AttributeValue{
			"fileId":   &types.AttributeValueMemberS{Value: fileId},
			"fileName": &types.AttributeValueMemberS{Value: fileName},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to insert file: %w", err)
	}

	fmt.Println("File created:", fileId)
	return nil
}

func GetFile(client *dynamodb.Client, tableName, fileId string) (map[string]types.AttributeValue, error) {
	out, err := client.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"fileId": &types.AttributeValueMemberS{Value: fileId},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	if out.Item == nil {
		return nil, fmt.Errorf("file not found: %s", fileId)
	}

	return out.Item, nil
}

func UpdateFileName(client *dynamodb.Client, tableName, fileId, newFileName string) error {
	_, err := client.UpdateItem(context.TODO(), &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"fileId": &types.AttributeValueMemberS{Value: fileId},
		},
		UpdateExpression:          aws.String("SET fileName = :n"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":n": &types.AttributeValueMemberS{Value: newFileName}},
		ReturnValues:              types.ReturnValueUpdatedNew,
	})
	if err != nil {
		return fmt.Errorf("failed to update file name: %w", err)
	}

	fmt.Println("File updated:", fileId)
	return nil
}

func DeleteFile(client *dynamodb.Client, tableName, fileId string) error {
	_, err := client.DeleteItem(context.TODO(), &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"fileId": &types.AttributeValueMemberS{Value: fileId},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	fmt.Println("🗑️ File deleted:", fileId)
	return nil
}

func ListFiles(client *dynamodb.Client, tableName string) ([]map[string]types.AttributeValue, error) {
	out, err := client.Scan(context.TODO(), &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return out.Items, nil
}
