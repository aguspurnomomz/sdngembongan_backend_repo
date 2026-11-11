package upload

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func GeneratePresignedPutURL(ctx context.Context, objectKey, contentType string) (string, error) {
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKeyID := os.Getenv("R2_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	bucketName := os.Getenv("R2_BUCKET_NAME")

	if accountID == "" || accessKeyID == "" || secretAccessKey == "" || bucketName == "" {
		return "", fmt.Errorf("konfigurasi R2 belum lengkap (cek R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_BUCKET_NAME)")
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		),
		config.WithRegion("auto"),
	)
	if err != nil {
		return "", fmt.Errorf("gagal load config R2: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID))
	})

	presignClient := s3.NewPresignClient(client)

	result, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(objectKey),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(10*time.Minute))

	if err != nil {
		return "", fmt.Errorf("gagal generate presigned URL: %w", err)
	}

	return result.URL, nil
}

func PresignedURL(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var userIDStr string
	switch v := userID.(type) {
	case string:
		userIDStr = v
	case fmt.Stringer:
		userIDStr = v.String()
	default:
		userIDStr = fmt.Sprintf("%v", v)
	}

	fileType := c.Query("file_type")
	if fileType == "" {
		fileType = "image"
	}

	var ext, contentType string
	switch fileType {
	case "image":
		ext = ".webp"
		contentType = "image/webp"
	case "pdf":
		ext = ".pdf"
		contentType = "application/pdf"
	case "table":
		ext = ".webp"
		contentType = "image/webp"
	default:
		ext = ".bin"
		contentType = "application/octet-stream"
	}

	objectKey := fmt.Sprintf("exams/%s/%s%s", userIDStr, uuid.New().String(), ext)

	url, err := GeneratePresignedPutURL(c.Request.Context(), objectKey, contentType)
	if err != nil {
		fmt.Printf("[PRESIGN] error untuk user=%s objectKey=%s: %v\n", userIDStr, objectKey, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal generate presigned URL: " + err.Error()})
		return
	}

	publicURL := ""
	if base := os.Getenv("R2_PUBLIC_URL"); base != "" {
		publicURL = strings.TrimRight(base, "/") + "/" + objectKey
	}

	c.JSON(http.StatusOK, gin.H{
		"upload_url":   url,
		"object_key":   objectKey,
		"content_type": contentType,
		"public_url":   publicURL,
		"expires_in":   600, 
	})
}