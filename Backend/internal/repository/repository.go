package repository

import (
	"bibliotheca/internal/models"
	"context"
)

type UserRepository interface {
	GetByID(ctx context.Context, id uint64) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	CreateUser(ctx context.Context, user *models.User) (uint64, error)
	UpdateUser(ctx context.Context, user *models.User) error
	UpdateStatus(ctx context.Context, id uint64, isActive bool) error
}

type UserProfileRepository interface {
	GetByUserID(ctx context.Context, userID uint64) (*models.UserProfile, error)
	CreateUserProfile(ctx context.Context, profile *models.UserProfile) error
	UpdateUserProfile(ctx context.Context, profile *models.UserProfile) error
	UpdateLastOnline(ctx context.Context, userID uint64) error
	UpdateLastReadBook(ctx context.Context, userID uint64, bookID uint64) error
	IncrementBooksRead(ctx context.Context, userID uint64) error
	IncrementPagesRead(ctx context.Context, userID uint64, pages uint32) error
}

type AuthorRepository interface {
	GetAuthorByID(ctx context.Context, id uint64) (*models.Author, error)
	GetAllAuthors(ctx context.Context, limit, offset int) ([]*models.Author, int, error)
	CreateAuthor(ctx context.Context, author *models.Author) (uint64, error)
	UpdateAuthor(ctx context.Context, author *models.Author) error
	DeleteAuthor(ctx context.Context, id uint64) error
}

type BookRepository interface {
	GetBookByID(ctx context.Context, id uint64) (*models.Book, error)
	GetAllBooks(ctx context.Context, limit, offset int) ([]*models.Book, int, error)
	CreateBook(ctx context.Context, book *models.Book) (uint64, error)
	UpdateBook(ctx context.Context, book *models.Book) error
	DeleteBook(ctx context.Context, id uint64) error
	UpdateFilePath(ctx context.Context, id uint64, path string, size int64, format string) error
	DecrementAvailableCopies(ctx context.Context, id uint64) error
	IncrementAvailableCopies(ctx context.Context, id uint64) error
	Search(ctx context.Context, query, genre string, year, limit, offset int) ([]*models.Book, int, error)
}

type BookAuthorRepository interface {
	AssignAuthor(ctx context.Context, ba *models.BookAuthor) error
	RemoveAuthor(ctx context.Context, bookID, authorID uint64) error
	GetAuthorsByBookID(ctx context.Context, bookID uint64) ([]*models.Author, error)
	GetBooksByAuthorID(ctx context.Context, authorID uint64, limit, offset int) ([]*models.Book, int, error)
}

type BorrowRepository interface {
	GetByID(ctx context.Context, id uint64) (*models.BorrowRecord, error)
	GetAllByUserID(ctx context.Context, userID uint64, limit, offset int) ([]*models.BorrowRecord, int, error)
	GetAll(ctx context.Context, limit, offset int) ([]*models.BorrowRecord, int, error)
	Create(ctx context.Context, record *models.BorrowRecord) (uint64, error)
	MarkReturned(ctx context.Context, id uint64) error
	UpdateOverdue(ctx context.Context) error
	HasActiveBorrow(ctx context.Context, userID, bookID uint64) (bool, error)
}

type ReadingSessionRepository interface {
	GetByUserAndBook(ctx context.Context, userID, bookID uint64) (*models.ReadingSession, error)
	Upsert(ctx context.Context, session *models.ReadingSession) error
}

type UserLibraryRepository interface {
	GetByUserID(ctx context.Context, userID uint64, status string, limit, offset int) ([]*models.UserLibrary, int, error)
	GetEntry(ctx context.Context, userID, bookID uint64) (*models.UserLibrary, error)
	Upsert(ctx context.Context, entry *models.UserLibrary) error
}

type BookmarkRepository interface {
	GetByUserAndBook(ctx context.Context, userID, bookID uint64) ([]*models.Bookmark, error)
	GetByID(ctx context.Context, id uint64) (*models.Bookmark, error)
	Create(ctx context.Context, bookmark *models.Bookmark) (uint64, error)
	Delete(ctx context.Context, id uint64) error
}

type TokenRepository interface {
	Create(ctx context.Context, token *models.RefreshToken) error
	GetByToken(ctx context.Context, token string) (*models.RefreshToken, error)
	Revoke(ctx context.Context, token string) error
	RevokeAllByUserID(ctx context.Context, userID uint64) error
	DeleteExpired(ctx context.Context) error
}
