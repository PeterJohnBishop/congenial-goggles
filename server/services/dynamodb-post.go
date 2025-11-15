package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type BlogPost struct {
	ID          string    `dynamodbav:"id"`
	Title       string    `dynamodbav:"title"`
	Paragraphs  []string  `dynamodbav:"paragraphs"`
	Images      []string  `dynamodbav:"images"`
	Author      string    `dynamodbav:"author"`
	DateCreated time.Time `dynamodbav:"date_created"`
	DateUpdated time.Time `dynamodbav:"date_updated"`
}

func CreateBlogPostTable(client *dynamodb.Client, tableName string) error {

	_, err := client.DescribeTable(context.TODO(), &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err == nil {
		return nil
	}

	var notFound *types.ResourceNotFoundException
	if !errors.As(err, &notFound) {
		return fmt.Errorf("error checking table existence: %w", err)
	}

	fmt.Println("BlogPost table not found — creating now...")

	_, err = client.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),

		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("id"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("author"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},

		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("id"),
				KeyType:       types.KeyTypeHash, // PK
			},
		},

		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: aws.String("author-index"),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String("author"),
						KeyType:       types.KeyTypeHash,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
			},
		},

		BillingMode: types.BillingModePayPerRequest,
	})

	return err
}

func CreateBlogPost(client *dynamodb.Client, tableName string, item map[string]types.AttributeValue) error {

	_, err := client.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to create blog post: %w", err)
	}

	return nil
}

func GetAllBlogPosts(client *dynamodb.Client, tableName string) ([]map[string]types.AttributeValue, error) {
	var items []map[string]types.AttributeValue
	var lastEvaluatedKey map[string]types.AttributeValue

	for {
		out, err := client.Scan(context.TODO(), &dynamodb.ScanInput{
			TableName:         aws.String(tableName),
			ExclusiveStartKey: lastEvaluatedKey,
		})
		if err != nil {
			return nil, err
		}

		items = append(items, out.Items...)

		if out.LastEvaluatedKey == nil {
			break
		}
		lastEvaluatedKey = out.LastEvaluatedKey
	}

	return items, nil
}

func GetBlogPostByAuthor(client *dynamodb.Client, tableName, author string) ([]BlogPost, error) {

	result, err := client.Query(context.TODO(), &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("author-index"),
		KeyConditionExpression: aws.String("author = :author"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":author": &types.AttributeValueMemberS{Value: author},
		},
	})
	if err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		return []BlogPost{}, nil
	}

	posts := make([]BlogPost, len(result.Items))
	for i, item := range result.Items {
		var post BlogPost
		err = attributevalue.UnmarshalMap(item, &post)
		if err != nil {
			return nil, err
		}
		posts[i] = post
	}

	return posts, nil
}

func UpdateBlogPost(client *dynamodb.Client, tableName string, post BlogPost) error {
	updateBuilder := expression.UpdateBuilder{}
	updatedFields := 0

	getOut, err := client.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: post.ID},
		},
	})
	if err != nil {
		return fmt.Errorf("error checking blog post existence: %w", err)
	}
	if getOut.Item == nil {
		return fmt.Errorf("blog post with ID %s not found", post.ID)
	}

	if post.Title != "" {
		updateBuilder = updateBuilder.Set(expression.Name("title"), expression.Value(post.Title))
		updatedFields++
	}

	if post.Paragraphs != nil {
		updateBuilder = updateBuilder.Set(expression.Name("paragraphs"), expression.Value(post.Paragraphs))
		updatedFields++
	}

	if post.Images != nil {
		updateBuilder = updateBuilder.Set(expression.Name("images"), expression.Value(post.Images))
		updatedFields++
	}

	if post.Author != "" {
		updateBuilder = updateBuilder.Set(expression.Name("author"), expression.Value(post.Author))
		updatedFields++
	}

	if !post.DateUpdated.IsZero() {
		updateBuilder = updateBuilder.Set(expression.Name("date_updated"), expression.Value(post.DateUpdated))
		updatedFields++
	}

	if updatedFields == 0 {
		return fmt.Errorf("must update at least one field")
	}

	expr, err := expression.NewBuilder().WithUpdate(updateBuilder).Build()
	if err != nil {
		return fmt.Errorf("error in expression builder: %w", err)
	}

	_, err = client.UpdateItem(context.TODO(), &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: post.ID},
		},
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
		ConditionExpression:       aws.String("attribute_exists(id)"),
		ReturnValues:              types.ReturnValueUpdatedNew,
	})
	if err != nil {
		return fmt.Errorf("error updating blog post: %w", err)
	}

	return nil
}

func DeleteBlogPost(client *dynamodb.Client, tableName, id string) error {
	_, err := client.DeleteItem(context.TODO(), &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"ID": &types.AttributeValueMemberS{Value: id},
		},
	})
	return err
}
