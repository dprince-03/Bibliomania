package catalog

type BookAuthor struct {
	BookID   uint64 `db:"book_id"`
	AuthorID uint64 `db:"author_id"`
	Role     string `db:"role"`
}
