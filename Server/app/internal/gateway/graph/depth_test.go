package graph

import (
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

func depthOf(t *testing.T, q string) int {
	t.Helper()
	doc, err := parser.ParseQuery(&ast.Source{Input: q})
	if err != nil {
		t.Fatal(err)
	}
	return selectionDepth(doc.Operations[0].SelectionSet, doc.Fragments, 0, map[string]bool{})
}

func TestSelectionDepth(t *testing.T) {
	cases := map[string]int{
		`{ me { email } }`: 1,
		`{ book(id: "1") { myBorrowStatus { book { title } } } }`:                                                                        3,
		`{ book(id: "1") { ...B } } fragment B on Book { myBorrowStatus { book { authors { id } } } }`:                                   4,
		`{ __schema { types { fields { type { ofType { name } } } } } }`:                                                                 0,
		`{ myLibrary { items { book { myBorrowStatus { book { myBorrowStatus { book { myBorrowStatus { book { title } } } } } } } } } }`: 9, // over the limit of 8
	}
	for q, want := range cases {
		if got := depthOf(t, q); got != want {
			t.Errorf("%s\n  depth %d, want %d", q, got, want)
		}
	}
}
