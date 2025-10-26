package middlware

import (
	"congenial-goggles/server/services"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

func HashedPassword(password string) (string, error) {
	hashedPassword, error := bcrypt.GenerateFromPassword([]byte(password), 10)
	return string(hashedPassword), error
}

func CheckPasswordHash(password, hash string) bool {
	if password == "" || hash == "" {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

var (
	AccessTokenSecret  string
	RefreshTokenSecret string
	AccessTokenTTL     = time.Minute * 15
	RefreshTokenTTL    = time.Hour * 24 * 7
)

// Load .env once at startup
func InitAuth() {
	envPath := filepath.Join(".", ".env") // this points to fluffy-octo-tribble/.env

	if err := godotenv.Load(envPath); err != nil {
		log.Fatalf("Error loading .env file at %s: %v", envPath, err)
	}

	AccessTokenSecret = os.Getenv("TOKEN_SECRET")
	RefreshTokenSecret = os.Getenv("REFRESH_TOKEN_SECRET")

	if AccessTokenSecret == "" || RefreshTokenSecret == "" {
		log.Fatal("TOKEN_SECRET or REFRESH_TOKEN_SECRET is missing")
	}
}

type UserClaims struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	TokenType string `json:"token_type"`
	jwt.StandardClaims
}

func NewAccessToken(claims UserClaims) (string, error) {
	claims.TokenType = "access"
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return accessToken.SignedString([]byte(AccessTokenSecret))
}

func NewRefreshToken(claims jwt.StandardClaims) (string, error) {
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return refreshToken.SignedString([]byte(RefreshTokenSecret))
}

func ParseAccessToken(accessToken string) *UserClaims {
	parsedAccessToken, err := jwt.ParseWithClaims(accessToken, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Ensure correct signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(AccessTokenSecret), nil
	})
	if err != nil || !parsedAccessToken.Valid {
		fmt.Println("Token verification failed:", err) // Debugging output
		return nil
	}

	claims, ok := parsedAccessToken.Claims.(*UserClaims)
	if !ok {
		fmt.Println("Failed to cast token claims")
		return nil
	}

	return claims
}

func ParseRefreshToken(refreshToken string) *jwt.StandardClaims {
	parsedRefreshToken, err := jwt.ParseWithClaims(refreshToken, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Ensure correct signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(RefreshTokenSecret), nil
	})
	if err != nil || !parsedRefreshToken.Valid {
		fmt.Println("Refresh token verification failed:", err)
		return nil
	}

	claims, ok := parsedRefreshToken.Claims.(*jwt.StandardClaims)
	if !ok {
		fmt.Println("Failed to cast refresh token claims")
		return nil
	}

	return claims
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token format"})
			c.Abort()
			return
		}

		claims := ParseAccessToken(token)
		if claims == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to verify token"})
			c.Abort()
			return
		}

		if claims.TokenType == "refresh" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Refresh tokens cannot be used here"})
			c.Abort()
			return
		}

		c.Set("claims", claims)
		c.Next()
	}
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

func RefreshTokenHandler(client *dynamodb.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RefreshRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		claims := ParseRefreshToken(req.RefreshToken)
		if claims == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
			return
		}

		userID := claims.Subject

		out, err := client.GetItem(context.TODO(), &dynamodb.GetItemInput{
			TableName: aws.String("users"), // adjust table name
			Key: map[string]types.AttributeValue{
				"id": &types.AttributeValueMemberS{Value: userID},
			},
		})
		if err != nil || out.Item == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}

		var user services.User
		if err := attributevalue.UnmarshalMap(out.Item, &user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unmarshal user"})
			return
		}

		newClaims := UserClaims{
			ID:        user.ID,
			Name:      user.Name,
			Email:     user.Email,
			TokenType: "access",
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: time.Now().Add(AccessTokenTTL).Unix(),
				IssuedAt:  time.Now().Unix(),
				Subject:   user.ID,
			},
		}

		accessToken, err := NewAccessToken(newClaims)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate access token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"accessToken": accessToken,
		})
	}
}
