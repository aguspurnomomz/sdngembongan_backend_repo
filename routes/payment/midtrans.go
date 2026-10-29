package payment

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	//"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

// verifyMidtransSignature memverifikasi signature key dari notifikasi Midtrans.
func verifyMidtransSignature(orderID, statusCode, grossAmount, serverKey, receivedSignature string) bool {
	if receivedSignature == "" {
		return false
	}
	rawString := orderID + statusCode + grossAmount + serverKey
	hash := sha512.Sum512([]byte(rawString))
	calculatedSignature := hex.EncodeToString(hash[:])
	return hmac.Equal([]byte(calculatedSignature), []byte(receivedSignature))
}

// randString dipakai untuk generate suffix order_id
func randString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

// POST /api/payment/create-snap (protected)
func CreateSnap(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data paket tidak valid: " + err.Error()})
		return
	}

	orderID := fmt.Sprintf("TP-%d-%s", time.Now().Unix(), strings.ToUpper(randString(4)))

	_, err := database.DB.Exec(
		`INSERT INTO token_orders (user_id, order_id, package_name, token_amount, amount, status) VALUES ($1, $2, $3, $4, $5, 'pending')`,
		userID, orderID, req.PackageName, req.TokenAmount, req.Amount,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mencatat pesanan ke database"})
		return
	}

	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	if serverKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Konfigurasi Midtrans belum lengkap di server"})
		return
	}

	isProduction := os.Getenv("MIDTRANS_IS_PRODUCTION") == "true"

	midtransURL := "https://app.sandbox.midtrans.com/snap/v1/transactions"
	if isProduction {
		midtransURL = "https://app.midtrans.com/snap/v1/transactions"
	}

	payload := map[string]any{
		"transaction_details": map[string]any{
			"order_id":     orderID,
			"gross_amount": req.Amount,
		},
		"item_details": []map[string]any{
			{
				"id":       req.PackageName,
				"price":    req.Amount,
				"quantity": 1,
				"name":     fmt.Sprintf("Paket Token %s (%d Token)", req.PackageName, req.TokenAmount),
			},
		},
	}

	jsonBody, _ := json.Marshal(payload)
	httpReq, _ := http.NewRequest("POST", midtransURL, bytes.NewBuffer(jsonBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.SetBasicAuth(serverKey, "")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal terhubung ke gateway pembayaran Midtrans"})
		return
	}
	defer resp.Body.Close()

	var midtransResp map[string]any
	json.NewDecoder(resp.Body).Decode(&midtransResp)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		errMsg := "Gagal membuat transaksi dari Midtrans"
		if errs, ok := midtransResp["error_messages"].([]any); ok && len(errs) > 0 {
			errMsg = fmt.Sprintf("%v", errs[0])
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	snapToken, _ := midtransResp["token"].(string)
	redirectURL, _ := midtransResp["redirect_url"].(string)

	if snapToken == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token Snap tidak ditemukan dari Midtrans"})
		return
	}

	_, _ = database.DB.Exec(
		`UPDATE token_orders SET snap_token = $1, snap_redirect_url = $2 WHERE order_id = $3`,
		snapToken, redirectURL, orderID,
	)

	c.JSON(http.StatusOK, gin.H{
		"order_id":     orderID,
		"snap_token":   snapToken,
		"redirect_url": redirectURL,
	})
}