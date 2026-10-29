package payment

import (
	"database/sql"
	//"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

// POST /api/payment/webhook (public, tanpa auth)
func Webhook(c *gin.Context) {
	var notification map[string]any
	if err := c.ShouldBindJSON(&notification); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_PAYLOAD",
				"message": "Format notifikasi tidak valid",
			},
		})
		return
	}

	orderID, _ := notification["order_id"].(string)
	transactionStatus, _ := notification["transaction_status"].(string)
	fraudStatus, _ := notification["fraud_status"].(string)
	statusCode, _ := notification["status_code"].(string)
	grossAmount, _ := notification["gross_amount"].(string)
	receivedSignature, _ := notification["signature_key"].(string)

	if orderID == "" || transactionStatus == "" || statusCode == "" || grossAmount == "" {
		log.Printf("[WEBHOOK] Field wajib tidak lengkap: orderID=%q status=%q statusCode=%q grossAmount=%q",
			orderID, transactionStatus, statusCode, grossAmount)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "MISSING_FIELDS",
				"message": "Field wajib tidak lengkap",
			},
		})
		return
	}

	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	if serverKey == "" {
		log.Printf("[WEBHOOK] FATAL: MIDTRANS_SERVER_KEY tidak dikonfigurasi di environment")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "SERVER_MISCONFIGURED",
				"message": "Konfigurasi server tidak valid",
			},
		})
		return
	}

	if !verifyMidtransSignature(orderID, statusCode, grossAmount, serverKey, receivedSignature) {
		log.Printf("[WEBHOOK] Signature TIDAK VALID untuk order_id=%q dari IP=%s — MUNGKIN SERANGAN!",
			orderID, c.ClientIP())
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "INVALID_SIGNATURE",
				"message": "Signature tidak valid",
			},
		})
		return
	}

	log.Printf("[WEBHOOK] Signature valid untuk order_id=%q, status=%q, fraud=%q",
		orderID, transactionStatus, fraudStatus)

	switch transactionStatus {

	case "capture", "settlement":
		if fraudStatus != "" && fraudStatus != "accept" {
			log.Printf("[WEBHOOK] Order %s fraud_status=%q, skip proses", orderID, fraudStatus)
			c.JSON(http.StatusOK, gin.H{"status": "ok", "note": "fraud not accepted"})
			return
		}

		var userID string
		var tokenAmount int
		var dbAmount int
		var currentStatus string
		err := database.DB.QueryRow(
			`SELECT user_id, token_amount, amount, status FROM token_orders WHERE order_id = $1`,
			orderID,
		).Scan(&userID, &tokenAmount, &dbAmount, &currentStatus)

		if err != nil {
			if err == sql.ErrNoRows {
				log.Printf("[WEBHOOK] Order %s tidak ditemukan di database", orderID)
				c.JSON(http.StatusOK, gin.H{"status": "ok", "note": "order not found"})
				return
			}
			log.Printf("[WEBHOOK] Error query order %s: %v", orderID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		// ---------- IDEMPOTENCY CHECK ----------
		if currentStatus == "success" {
			log.Printf("[WEBHOOK] Order %s sudah diproses sebelumnya, skip (idempotent)", orderID)
			c.JSON(http.StatusOK, gin.H{"status": "ok", "note": "already processed"})
			return
		}

		if currentStatus != "pending" {
			log.Printf("[WEBHOOK] Order %s status tidak valid untuk diproses: %q", orderID, currentStatus)
			c.JSON(http.StatusOK, gin.H{"status": "ok", "note": "invalid status transition"})
			return
		}

		// ---------- VERIFIKASI AMOUNT ----------
		midtransAmountFloat, err := strconv.ParseFloat(grossAmount, 64)
		if err != nil {
			log.Printf("[WEBHOOK] Order %s gross_amount tidak valid: %q", orderID, grossAmount)
			c.JSON(http.StatusOK, gin.H{"status": "ok", "note": "invalid amount format"})
			return
		}
		if int(midtransAmountFloat) != dbAmount {
			log.Printf("[WEBHOOK] ⚠️ Amount MISMATCH untuk order %s: Midtrans=%v, DB=%d",
				orderID, midtransAmountFloat, dbAmount)
			c.JSON(http.StatusOK, gin.H{"status": "ok", "note": "amount mismatch"})
			return
		}

		// ---------- PROSES DALAM TRANSACTION ----------
		tx, err := database.DB.Begin()
		if err != nil {
			log.Printf("[WEBHOOK] Gagal mulai transaksi untuk order %s: %v", orderID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		defer tx.Rollback()

		result, err := tx.Exec(
			`UPDATE token_orders SET status = 'success', updated_at = NOW() 
			WHERE order_id = $1 AND status = 'pending'`,
			orderID,
		)
		if err != nil {
			log.Printf("[WEBHOOK] Gagal update order %s: %v", orderID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			log.Printf("[WEBHOOK] Order %s sudah di-proses oleh request lain (race condition)", orderID)
			c.JSON(http.StatusOK, gin.H{"status": "ok", "note": "concurrent processing detected"})
			return
		}

		_, err = tx.Exec(
			`UPDATE profiles SET token_balance = token_balance + $1, updated_at = NOW() WHERE id = $2`,
			tokenAmount, userID,
		)
		if err != nil {
			log.Printf("[WEBHOOK] Gagal tambah token untuk user %s: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		if err := tx.Commit(); err != nil {
			log.Printf("[WEBHOOK] Gagal commit transaksi order %s: %v", orderID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		log.Printf("[WEBHOOK] Order %s BERHASIL diproses: user=%s +%d token",
			orderID, userID, tokenAmount)

	// ---------- Pembayaran GAGAL/DIBATALKAN ----------
	case "cancel", "deny", "expire":
		result, err := database.DB.Exec(
			`UPDATE token_orders SET status = $1, updated_at = NOW() 
			WHERE order_id = $2 AND status = 'pending'`,
			transactionStatus, orderID,
		)
		if err != nil {
			log.Printf("[WEBHOOK] Gagal update order %s ke status %s: %v", orderID, transactionStatus, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		rowsAffected, _ := result.RowsAffected()
		log.Printf("[WEBHOOK] Order %s status di-update ke %q (rows affected: %d)",
			orderID, transactionStatus, rowsAffected)

	// ---------- Status tidak dikenal ----------
	default:
		log.Printf("[WEBHOOK] Order %s transaction_status tidak dikenal: %q", orderID, transactionStatus)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}