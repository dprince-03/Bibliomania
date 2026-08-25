package cache

import "fmt"

const (
	TTLAuthorList   = 5  // minutes
	TTLAuthorSingle = 10 // minutes
	TTLBookList     = 5  // minutes
	TTLBookSingle   = 10 // minutes
	TTLUserSingle   = 10 // minutes
	TTLSearchResult = 2  // minutes
)

func KeyAuthorSingle(id uint64) string {
	return fmt.Sprintf("authors:single:%d", id)
}

func KeyBookList(page, limit int, genre, author string) string {
	return fmt.Sprintf("books:list:%d:%d:%s:%s", page, limit, genre, author)
}

func KeyBookSingle(id uint64) string {
	return fmt.Sprintf("books:single:%d", id)
}

func KeyUserSingle(id uint64) string {
	return fmt.Sprintf("users:single:%d", id)
}

func KeySearchBooks(query, genre, format string, authorID uint64, year, page, limit int) string {
	return fmt.Sprintf("search:books:%s:%s:%s:%d:%d:%d:%d", query, genre, format, authorID, year, page, limit)
}
