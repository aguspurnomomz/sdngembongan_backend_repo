package payment

type CreateTransactionRequest struct {
	PackageName string `json:"package_name" binding:"required"`
	TokenAmount int    `json:"token_amount" binding:"required"`
	Amount      int    `json:"amount" binding:"required"`
}