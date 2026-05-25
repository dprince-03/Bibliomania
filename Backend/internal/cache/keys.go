package cache

import "fmt"

const (
	TTLBookList = 5
	TTLBookSingle = 10
	TTLUserSingle = 10
	TTLSearchResult = 2
)

func KeyBookList(page, limit int, genre, author string) string {
	return fmt.Sprintf("books:list:%d:%d:%s:%s", page, limit, genre, author)
}

func KeyBookSingle(id string) string {
	return fmt.Sprintf("books:single:%s", id)
}

func KeyUserSingle(id string) string {
	return fmt.Sprintf("users:single:%s", id)
}

func KeySearchBooks(query string, page, limit int) string {
	return fmt.Sprintf("search:books:%s:%d:%d", query, page, limit)
}