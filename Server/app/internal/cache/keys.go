package cache

import "fmt"

// All cache keys live here (catalog-service is the only cache user). List
// and search caches are invalidated by prefix (Cache.DeletePrefix), so every
// page/filter combination goes — the old approach deleted three hard-coded
// page keys and left every other page stale.
const (
	TTLAuthorList   = 5  // minutes
	TTLAuthorSingle = 10 // minutes
	TTLBookList     = 5  // minutes
	TTLBookSingle   = 10 // minutes
	TTLSearchResult = 2  // minutes
)

const (
	PrefixAuthorList  = "authors:list:"
	PrefixAuthorBooks = "authors:books:"
	PrefixBookList    = "books:list:"
	PrefixSearch      = "search:books:"
)

func KeyAuthorSingle(id uint64) string {
	return fmt.Sprintf("authors:single:%d", id)
}

func KeyAuthorList(page, limit int) string {
	return fmt.Sprintf("%s%d:%d", PrefixAuthorList, page, limit)
}

func KeyAuthorBooks(authorID uint64, page, limit int) string {
	return fmt.Sprintf("%s%d:%d:%d", PrefixAuthorBooks, authorID, page, limit)
}

func KeyBookList(page, limit int) string {
	return fmt.Sprintf("%s%d:%d", PrefixBookList, page, limit)
}

func KeyBookSingle(id uint64) string {
	return fmt.Sprintf("books:single:%d", id)
}

func KeySearchBooks(query, genre, format string, authorID uint64, year, page, limit int) string {
	return fmt.Sprintf("%s%s:%s:%s:%d:%d:%d:%d", PrefixSearch, query, genre, format, authorID, year, page, limit)
}
