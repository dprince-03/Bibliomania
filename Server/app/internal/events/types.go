package events

import "time"

// Event types — "<producing service>.<past-tense fact>". This file is the
// shared contract between producers and consumers; changing a payload's
// shape incompatibly means bumping its entry in CurrentVersions (see the
// Event doc comment).
const (
	// auth → RabbitMQ
	TypeUserRegistered = "auth.user_registered"

	// user → RabbitMQ
	TypeUserStatusChanged = "user.status_changed"

	// catalog → RabbitMQ
	TypeBookAdded    = "catalog.book_added"
	TypeBookUpdated  = "catalog.book_updated"
	TypeBookDeleted  = "catalog.book_deleted"
	TypeCopyReserved = "catalog.copy_reserved"
	TypeCopyReleased = "catalog.copy_released"

	// borrow → Kafka (topic TopicBorrow)
	TypeBookBorrowed  = "borrow.book_borrowed"
	TypeBookReturned  = "borrow.book_returned"
	TypeBorrowOverdue = "borrow.borrow_overdue"

	// reading → NATS JetStream (stream StreamReading)
	TypeProgressUpdated = "reading.progress_updated"
	TypeBookCompleted   = "reading.book_completed"

	// payment → Kafka (topic TopicPayment)
	TypeBookPurchased = "payment.book_purchased"
	TypePaymentFailed = "payment.payment_failed"
)

// CurrentVersions is the schema version each producer emits today.
var CurrentVersions = map[string]int{
	TypeUserRegistered:    1,
	TypeUserStatusChanged: 1,
	TypeBookAdded:         1,
	TypeBookUpdated:       1,
	TypeBookDeleted:       1,
	TypeCopyReserved:      1,
	TypeCopyReleased:      1,
	TypeBookBorrowed:      1,
	TypeBookReturned:      1,
	TypeBorrowOverdue:     1,
	TypeProgressUpdated:   1,
	TypeBookCompleted:     1,
	TypeBookPurchased:     1,
	TypePaymentFailed:     1,
}

// Broker destinations.
const (
	RabbitExchange = "bibliomania.events"
	TopicBorrow    = "borrow.events"
	TopicPayment   = "payment.events"
	StreamReading  = "READING"
)

// ── Payloads (v1) ─────────────────────────────────────────

type UserRegistered struct {
	UserID    uint64 `json:"user_id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
	IsActive  bool   `json:"is_active"`
}

type UserStatusChanged struct {
	UserID   uint64 `json:"user_id"`
	IsActive bool   `json:"is_active"`
}

type BookChanged struct {
	BookID uint64 `json:"book_id"`
	Title  string `json:"title"`
}

type CopyReservation struct {
	BookID         uint64 `json:"book_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

type BorrowEvent struct {
	BorrowID  uint64    `json:"borrow_id"`
	UserID    uint64    `json:"user_id"`
	UserEmail string    `json:"user_email"`
	BookID    uint64    `json:"book_id"`
	BookTitle string    `json:"book_title"`
	DueAt     time.Time `json:"due_at"`
}

type ProgressUpdated struct {
	UserID      uint64 `json:"user_id"`
	BookID      uint64 `json:"book_id"`
	CurrentPage uint32 `json:"current_page"`
	TotalPages  uint32 `json:"total_pages"`
}

type BookCompleted struct {
	UserID     uint64 `json:"user_id"`
	BookID     uint64 `json:"book_id"`
	TotalPages uint32 `json:"total_pages"`
}

type PaymentEvent struct {
	PurchaseID  uint64 `json:"purchase_id"`
	UserID      uint64 `json:"user_id"`
	UserEmail   string `json:"user_email"`
	BookID      uint64 `json:"book_id"`
	BookTitle   string `json:"book_title"`
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
	// Added with Paystack support — additive, so still schema v1.
	Provider string `json:"provider,omitempty"`
}
