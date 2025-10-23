package server

import (
	"bytes"
	"congenial-goggles/server/services"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"image/png"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gin-gonic/gin"
	qrcode "github.com/skip2/go-qrcode"
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

func getFileExtension(filename string) string {
	// Get extension (includes the dot, e.g. ".jpg")
	ext := filepath.Ext(filename)

	// Optional: clean it up (remove the dot and lowercase it)
	return strings.ToLower(strings.TrimPrefix(ext, "."))
}

func RemoveFileExtension(filename string) string {
	return filename[:len(filename)-len(filepath.Ext(filename))]
}

func Upload() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "Method not allowed"})
			return
		}

		secret := c.PostForm("shared_secret")
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
		ext := getFileExtension(header.Filename)
		if ext != "" {
			fileId = fileId + "." + ext
		}

		err = services.StreamUploadFile(fileId, file)
		if err != nil {
			log.Printf("Upload failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload file"})
			return
		}

		dynamoClient, err := services.ConnectDB()
		if err != nil {
			log.Printf("DynamoDB connection failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to database"})
			return
		}

		err = services.CreateFile(dynamoClient, "Files", fileId, filepath.Base(header.Filename))
		if err != nil {
			log.Printf("Failed to save metadata: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file metadata"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "File uploaded successfully",
			"fileId":  fileId,
		})
	}
}

func Download() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "Method not allowed"})
			return
		}
		hashedSecret := c.PostForm("hashed_secret")
		if hashedSecret == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing secret"})
			return
		}
		sharedSecret := c.PostForm("shared_secret")
		if sharedSecret == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing secret"})
			return
		}
		dynamoClient, err := services.ConnectDB()
		if err != nil {
			log.Printf("DynamoDB connection failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to database"})
			return
		}
		result, err := dynamoClient.GetItem(context.TODO(), &dynamodb.GetItemInput{
			TableName: aws.String("Files"),
			Key: map[string]types.AttributeValue{
				"fileId": &types.AttributeValueMemberS{Value: hashedSecret},
			},
		})
		if err != nil {
			log.Printf("Failed to retrieve file metadata: %v", err)
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

		mac := hmac.New(sha256.New, []byte(sharedSecret))
		mac.Write([]byte(storedFilename))
		hashedSecretVerification := hex.EncodeToString(mac.Sum(nil))
		ext := getFileExtension(storedFilename)
		if ext != "" {
			hashedSecretVerification = hashedSecretVerification + "." + ext
		}
		if hashedSecretVerification != hashedSecret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid secret"})
			return
		}

		err = services.StreamDownloadFile(c, hashedSecret)
		if err != nil {
			log.Printf("Failed to stream file: %v", hashedSecret)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stream file"})
			return
		}
	}
}

func DownloadURL() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "Method not allowed"})
			return
		}
		hashedSecret := c.PostForm("hashed_secret")
		if hashedSecret == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing hashed_secret"})
			return
		}
		sharedSecret := c.PostForm("shared_secret")
		if sharedSecret == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing shared_secret"})
			return
		}
		dynamoClient, err := services.ConnectDB()
		if err != nil {
			log.Printf("DynamoDB connection failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to database"})
			return
		}
		result, err := dynamoClient.GetItem(context.TODO(), &dynamodb.GetItemInput{
			TableName: aws.String("Files"),
			Key: map[string]types.AttributeValue{
				"fileId": &types.AttributeValueMemberS{Value: hashedSecret},
			},
		})
		if err != nil {
			log.Printf("Failed to retrieve file metadata: %v", err)
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
		mac := hmac.New(sha256.New, []byte(sharedSecret))
		mac.Write([]byte(storedFilename))
		expectedHash := hex.EncodeToString(mac.Sum(nil))
		ext := getFileExtension(storedFilename)
		if ext != "" {
			expectedHash = expectedHash + "." + ext
		}
		if expectedHash != hashedSecret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid secret"})
			return
		}
		fileKey := "uploads/" + hashedSecret
		url, err := services.GeneratePresignedDownloadURL(fileKey)
		if err != nil {
			log.Printf("Failed to generate presigned URL for %v: %v", storedFilename, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate presigned URL"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message":        "Presigned download URL generated",
			"file_name":      storedFilename,
			"presigned_url":  url,
			"url_expires_in": 300, // 5 minutes
		})
	}
}

func DownloadQR() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "Method not allowed"})
			return
		}
		hashedSecret := c.PostForm("hashed_secret")
		sharedSecret := c.PostForm("shared_secret")
		if hashedSecret == "" || sharedSecret == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing secret"})
			return
		}
		dynamoClient, err := services.ConnectDB()
		if err != nil {
			log.Printf("DynamoDB connection failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to database"})
			return
		}
		result, err := dynamoClient.GetItem(context.TODO(), &dynamodb.GetItemInput{
			TableName: aws.String("Files"),
			Key: map[string]types.AttributeValue{
				"fileId": &types.AttributeValueMemberS{Value: hashedSecret},
			},
		})
		if err != nil || result.Item == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
			return
		}
		storedFilenameAttr, ok := result.Item["fileName"].(*types.AttributeValueMemberS)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid file metadata format"})
			return
		}
		storedFilename := storedFilenameAttr.Value

		mac := hmac.New(sha256.New, []byte(sharedSecret))
		mac.Write([]byte(storedFilename))
		hashedSecretVerification := hex.EncodeToString(mac.Sum(nil))
		ext := getFileExtension(storedFilename)
		if ext != "" {
			hashedSecretVerification = hashedSecretVerification + "." + ext
		}
		if hashedSecretVerification != hashedSecret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid secret"})
			return
		}
		fileKey := "uploads/" + hashedSecret
		presignedURL, err := services.GeneratePresignedDownloadURL(fileKey)
		if err != nil {
			log.Printf("Failed to generate presigned URL: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate presigned URL"})
			return
		}
		qrImg, err := qrcode.New(presignedURL, qrcode.Medium)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate QR code"})
			return
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, qrImg.Image(256)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode QR image"})
			return
		}
		c.Header("Content-Type", "image/png")
		c.Header("Content-Disposition", "inline; filename=\"download_qr.png\"")
		c.Writer.Write(buf.Bytes())
	}
}
