package server

import (
	"bytes"
	"congenial-goggles/server/middlware"
	"congenial-goggles/server/services"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image/png"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/resend/resend-go/v2"
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

func ShortUUID() string {
	u := uuid.New()
	return strings.TrimRight(
		strings.NewReplacer("-", "", "_", "", "/", "").Replace(
			base64.URLEncoding.EncodeToString(u[:])),
		"=",
	)
}

func CreateNewUserReq(client *dynamodb.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var user services.User
		if err := c.ShouldBindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		id := ShortUUID()

		email := strings.ToLower(user.Email)
		userId := fmt.Sprintf("u_%s", id)

		hashedPassword, err := middlware.HashedPassword(user.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error hashing password"})
			return
		}

		newUser := map[string]types.AttributeValue{
			"id":       &types.AttributeValueMemberS{Value: userId},
			"name":     &types.AttributeValueMemberS{Value: user.Name},
			"email":    &types.AttributeValueMemberS{Value: email},
			"password": &types.AttributeValueMemberS{Value: hashedPassword},
		}

		if err := services.CreateUser(client, "Users", newUser); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		claims := middlware.UserClaims{
			ID:        user.ID,
			Name:      user.Name,
			Email:     email,
			TokenType: "access",
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: time.Now().Add(middlware.AccessTokenTTL).Unix(),
				IssuedAt:  time.Now().Unix(),
				Subject:   user.ID,
			},
		}

		accessToken, err := middlware.NewAccessToken(claims)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create access token"})
			return
		}

		refreshClaims := jwt.StandardClaims{
			ExpiresAt: time.Now().Add(middlware.RefreshTokenTTL).Unix(),
			IssuedAt:  time.Now().Unix(),
			Subject:   user.ID,
		}
		refreshToken, err := middlware.NewRefreshToken(refreshClaims)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create refresh token"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":      "User created successfully",
			"user.id":      userId,
			"accessToken":  accessToken,
			"refreshToken": refreshToken,
		})
	}
}

func AuthUserReq(client *dynamodb.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		if req.Email == "" || req.Password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email and password are required"})
			return
		}

		user, err := services.GetUserByEmail(client, "Users", req.Email)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No user found with that email"})
			return
		}

		if user.Password == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "User record missing password"})
			return
		}

		if !middlware.CheckPasswordHash(req.Password, user.Password) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Incorrect password"})
			return
		}

		claims := middlware.UserClaims{
			ID:        user.ID,
			Name:      user.Name,
			Email:     user.Email,
			TokenType: "access",
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: time.Now().Add(middlware.AccessTokenTTL).Unix(),
				IssuedAt:  time.Now().Unix(),
				Subject:   user.ID,
			},
		}

		accessToken, err := middlware.NewAccessToken(claims)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create access token"})
			return
		}

		refreshClaims := jwt.StandardClaims{
			ExpiresAt: time.Now().Add(middlware.RefreshTokenTTL).Unix(),
			IssuedAt:  time.Now().Unix(),
			Subject:   user.ID,
		}

		refreshToken, err := middlware.NewRefreshToken(refreshClaims)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create refresh token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":      "Login successful",
			"accessToken":  accessToken,
			"refreshToken": refreshToken,
			"user":         user,
		})
	}
}
func GetAllUsersReq(client *dynamodb.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid token"})
			return
		}
		if middlware.ParseAccessToken(token) == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to verify token"})
			return
		}

		resp, err := services.GetAllUsers(client, "Users")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get users"})
			return
		}

		var users []services.User
		for _, item := range resp {
			var user services.User
			if err := attributevalue.UnmarshalMap(item, &user); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode users"})
				return
			}
			users = append(users, user)
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Users Found!",
			"users":   users,
		})
	}
}

func GetUserByIDReq(client *dynamodb.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if token == "" || middlware.ParseAccessToken(token) == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to verify token"})
			return
		}

		resp, err := services.GetUserById(client, "Users", id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
			return
		}

		var user services.User
		if err := attributevalue.UnmarshalMap(resp, &user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode user"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "User Found!",
			"user":    user,
		})
	}
}

func UpdateUserReq(client *dynamodb.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if token == "" || middlware.ParseAccessToken(token) == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to verify token"})
			return
		}

		var user services.User
		if err := c.ShouldBindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		if err := services.UpdateUser(client, "Users", user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User Updated!"})
	}
}

func UpdatePasswordReq(client *dynamodb.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		claims := middlware.ParseAccessToken(token)
		if token == "" || claims == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to verify token"})
			return
		}

		var req struct {
			CurrentPassword string `json:"currentPassword"`
			NewPassword     string `json:"newPassword"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		if req.CurrentPassword == "" || req.NewPassword == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing current or new password"})
			return
		}

		out, err := client.GetItem(context.TODO(), &dynamodb.GetItemInput{
			TableName: aws.String("Users"),
			Key: map[string]types.AttributeValue{
				"id": &types.AttributeValueMemberS{Value: claims.ID},
			},
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
			return
		}

		if out.Item == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		var user services.User
		if err := attributevalue.UnmarshalMap(out.Item, &user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user record"})
			return
		}

		if !middlware.CheckPasswordHash(req.CurrentPassword, user.Password) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
			return
		}

		hashedPassword, err := middlware.HashedPassword(req.NewPassword)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash new password"})
			return
		}

		user.Password = hashedPassword
		if err := services.UpdatePassword(client, "Users", user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
	}
}

func DeleteUserReq(client *dynamodb.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if token == "" || middlware.ParseAccessToken(token) == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to verify token"})
			return
		}

		if err := services.DeleteUser(client, "Users", id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User Deleted!"})
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

func Upload(client *s3.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "Method not allowed"})
			return
		}

		claims, exists := c.Get("claims")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing JWT claims"})
			return
		}

		jwtClaims := claims.(*middlware.UserClaims)

		userId := jwtClaims.ID
		if userId == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "userId not found in JWT"})
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
		id := hex.EncodeToString(mac.Sum(nil))

		err = services.StreamUploadFile(client, header.Filename, file)
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

		err = services.CreateFile(dynamoClient, "Files", id, filepath.Base(header.Filename), userId)
		if err != nil {
			log.Printf("Failed to save metadata: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file metadata"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "File uploaded successfully",
			"fileId":  id,
			"userId":  userId,
		})
	}
}

func Download(client *s3.Client) gin.HandlerFunc {
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
				"id": &types.AttributeValueMemberS{Value: hashedSecret},
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

		if hashedSecretVerification != hashedSecret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid secret"})
			return
		}

		err = services.StreamDownloadFile(c, client, hashedSecret)
		if err != nil {
			log.Printf("Failed to stream file: %v", hashedSecret)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stream file"})
			return
		}
	}
}

func DownloadURL(client *s3.Client) gin.HandlerFunc {
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
				"id": &types.AttributeValueMemberS{Value: hashedSecret},
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

		if expectedHash != hashedSecret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid secret"})
			return
		}
		fileKey := "uploads/" + storedFilename
		url, err := services.GeneratePresignedDownloadURL(client, fileKey)
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

func DownloadQR(client *s3.Client) gin.HandlerFunc {
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
				"id": &types.AttributeValueMemberS{Value: hashedSecret},
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

		if hashedSecretVerification != hashedSecret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid secret"})
			return
		}
		fileKey := "uploads/" + storedFilename
		presignedURL, err := services.GeneratePresignedDownloadURL(client, fileKey)
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

func SendURLViaResend(client *resend.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
	}
}

func SendQRViaResend(client *resend.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
	}
}
