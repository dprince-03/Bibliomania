package payment

import (
	"io"
	"net/http"
	"time"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/middleware"
	"github.com/dprince-03/Bibliomania/internal/utils"

	"github.com/go-playground/validator/v10"
)

type CheckoutRequest struct {
	BookID uint64 `json:"book_id" validate:"required"`
	// Optional: force a provider. By default the book's currency decides
	// (NGN/GHS/ZAR/KES → Paystack, anything else → Stripe).
	Provider string `json:"provider" validate:"omitempty,oneof=stripe paystack"`
}

type CheckoutResponse struct {
	PurchaseID  uint64 `json:"purchase_id"`
	PublicID    string `json:"public_id"`
	Provider    string `json:"provider"`
	CheckoutURL string `json:"checkout_url"`
}

type PurchaseResponse struct {
	ID          uint64    `json:"id"`
	PublicID    string    `json:"public_id"`
	BookID      uint64    `json:"book_id"`
	BookTitle   string    `json:"book_title"`
	AmountCents int64     `json:"amount_cents"`
	Currency    string    `json:"currency"`
	Provider    string    `json:"provider"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type Handler struct {
	service  *Service
	validate *validator.Validate
}

func NewHandler(service *Service, validate *validator.Validate) *Handler {
	return &Handler{service: service, validate: validate}
}

func (h *Handler) Routes(mux *http.ServeMux, guard middleware.Guards) {
	mux.Handle("POST /api/v1/payments/checkout", guard.Member(h.Checkout))
	mux.Handle("GET /api/v1/payments/my", guard.Member(h.ListMine))
	// Public: authenticated by each provider's signature, not a JWT.
	mux.HandleFunc("POST /api/v1/payments/webhook/{provider}", h.Webhook)
}

// Checkout godoc
//
//	@Summary		Buy a book
//	@Description	Starts a hosted checkout for a book that has a price set; redirect the reader to checkout_url. The provider follows the book's currency — Paystack for NGN/GHS/ZAR/KES (PAYSTACK_CURRENCIES), Stripe otherwise — unless "provider" is given. The purchase becomes "paid" when the provider's webhook confirms it. 503 if the needed provider isn't configured on this server.
//	@Tags			payments
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CheckoutRequest	true	"Book to buy"
//	@Success		201		{object}	utils.APIResponse{data=CheckoutResponse}
//	@Failure		400		{object}	utils.APIError	"invalid body, book not for sale, or no provider takes its currency"
//	@Failure		401		{object}	utils.APIError	"missing/invalid token"
//	@Failure		404		{object}	utils.APIError	"book not found"
//	@Failure		409		{object}	utils.APIError	"already owned"
//	@Failure		503		{object}	utils.APIError	"payments not configured"
//	@Security		BearerAuth
//	@Router			/payments/checkout [post]
func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	var req CheckoutRequest
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		utils.HandleError(w, err)
		return
	}
	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	ctx := r.Context()
	resp, err := h.service.CreateCheckout(ctx, middleware.GetUserID(ctx), middleware.GetUserEmail(ctx), req.BookID, req.Provider)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	utils.Success(w, http.StatusCreated, "checkout started", resp)
}

// ListMine godoc
//
//	@Summary	List my purchases
//	@Tags		payments
//	@Produce	json
//	@Param		page	query		int	false	"Page number"		default(1)
//	@Param		limit	query		int	false	"Items per page"	default(10)
//	@Success	200		{object}	utils.APIResponse{data=utils.PaginatedResponse{items=[]PurchaseResponse}}
//	@Failure	401		{object}	utils.APIError	"missing/invalid token"
//	@Security	BearerAuth
//	@Router		/payments/my [get]
func (h *Handler) ListMine(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.ListMine(r.Context(), middleware.GetUserID(r.Context()), utils.GetPagination(r))
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	utils.Success(w, http.StatusOK, "purchases retrieved", resp)
}

// Webhook godoc
//
//	@Summary		Payment provider webhook
//	@Description	Called by Stripe or Paystack, not clients — register /api/v1/payments/webhook/stripe and /api/v1/payments/webhook/paystack in each dashboard. Stripe: verified with Stripe-Signature against STRIPE_WEBHOOK_SECRET. Paystack: x-paystack-signature (HMAC-SHA512 with the secret key), then re-checked against Paystack's Verify API. Deduped per provider event; a paid amount/currency that doesn't match the purchase never grants the book.
//	@Tags			payments
//	@Accept			json
//	@Produce		json
//	@Param			provider	path		string	true	"stripe or paystack"	Enums(stripe, paystack)
//	@Success		200			{object}	utils.APIResponse
//	@Failure		400			{object}	utils.APIError	"bad signature or payload"
//	@Failure		404			{object}	utils.APIError	"unknown provider"
//	@Failure		503			{object}	utils.APIError	"provider not configured"
//	@Router			/payments/webhook/{provider} [post]
func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		utils.Error(w, apperrors.BadRequest("unreadable body", err))
		return
	}
	if err := h.service.HandleWebhook(r.Context(), r.PathValue("provider"), payload, r.Header); err != nil {
		utils.HandleError(w, err)
		return
	}
	utils.Success(w, http.StatusOK, "received", nil)
}
