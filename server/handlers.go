package server

import (
	"congenial-goggles/server/services"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gin-gonic/gin"
)

func Hello() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Success</title>
	<style>
		body {
			font-family: Arial, sans-serif;
			background-color: #f9f9f9;
			display: flex;
			flex-direction: column;
			align-items: center;
			justify-content: center;
			height: 100vh;
			margin: 0;
		}
		h1 {
			margin-bottom: 20px;
		}
		.container {
			background: white;
			padding: 30px;
			border-radius: 10px;
			box-shadow: 0 2px 10px rgba(0,0,0,0.1);
			text-align: center;
			width: 300px;
		}
	</style>
</head>
<body>
	<div class="container">
		<h1>Transfer Complete</h1>
	</div>
</body>
</html>
		`)
	}
}

func Upload() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "Method not allowed"})
			return
		}
		secret := c.PostForm("secret")
		if secret == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing secret"})
			return
		}
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to retrieve file"})
			return
		}
		defer file.Close()
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(header.Filename))
		fileId := hex.EncodeToString(mac.Sum(nil))
		presignedURL, err := services.UploadFile(header.Filename, file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		dynamdbClient, err := services.ConnectDB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to database"})
			return
		}
		err = services.CreateFilesTable(dynamdbClient)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create files table"})
			return
		}
		err = services.CreateFile(dynamdbClient, "Files", fileId, filepath.Base(header.Filename))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file metadata"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message":        "File uploaded successfully",
			"presignedURL":   presignedURL,
			"url_expires_in": 900,
			"secret_hash":    fileId,
		})
	}
}

func Download() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "Method not allowed"})
			return
		}
		secret := c.PostForm("secret")
		if secret == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing secret"})
			return
		}
		filename := c.PostForm("filename")
		if filename == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing filename"})
			return
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(filename))
		fileId := hex.EncodeToString(mac.Sum(nil))
		dynamoClient, err := services.ConnectDB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to database"})
			return
		}
		result, err := dynamoClient.GetItem(context.TODO(), &dynamodb.GetItemInput{
			TableName: aws.String("Files"),
			Key: map[string]types.AttributeValue{
				"fileId": &types.AttributeValueMemberS{Value: fileId},
			},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file metadata"})
			return
		}
		if result.Item == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
			return
		}

		storedFilenameAttr, ok := result.Item["fileName"].(*types.AttributeValueMemberS)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid file metadata format"})
			return
		}
		storedFilename := storedFilenameAttr.Value

		url, err := services.DownloadFile(storedFilename)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate download URL"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":        "File download URL generated successfully",
			"fileName":       storedFilename,
			"presignedURL":   url,
			"url_expires_in": 300, // 5 minutes
		})
	}
}
