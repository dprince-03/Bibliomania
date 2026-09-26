package graph

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// DepthLimit rejects operations nested deeper than Max before they run.
// The complexity limit bounds total work, but this schema has cycles
// (Book → myBorrowStatus → BorrowRecord → book → …), each level a fan-out
// to another service, so depth is bounded separately. Fragments are
// followed; introspection's __schema/__type subtrees are exempt (GraphiQL
// needs them and they touch no service).
type DepthLimit struct {
	Max int
}

var _ interface {
	graphql.HandlerExtension
	graphql.OperationContextMutator
} = DepthLimit{}

func (DepthLimit) ExtensionName() string                   { return "DepthLimit" }
func (DepthLimit) Validate(graphql.ExecutableSchema) error { return nil }

func (d DepthLimit) MutateOperationContext(_ context.Context, rc *graphql.OperationContext) *gqlerror.Error {
	if rc.Operation == nil {
		return nil
	}
	if depth := selectionDepth(rc.Operation.SelectionSet, rc.Doc.Fragments, 0, map[string]bool{}); depth > d.Max {
		err := gqlerror.Errorf("query depth %d exceeds the maximum of %d", depth, d.Max)
		err.Extensions = map[string]any{"code": "BAD_USER_INPUT"}
		return err
	}
	return nil
}

func selectionDepth(set ast.SelectionSet, fragments ast.FragmentDefinitionList, depth int, seen map[string]bool) int {
	deepest := depth
	for _, sel := range set {
		var d int
		switch s := sel.(type) {
		case *ast.Field:
			if s.Name == "__schema" || s.Name == "__type" {
				continue
			}
			d = depth
			if len(s.SelectionSet) > 0 {
				d = selectionDepth(s.SelectionSet, fragments, depth+1, seen)
			}
		case *ast.InlineFragment:
			d = selectionDepth(s.SelectionSet, fragments, depth, seen)
		case *ast.FragmentSpread:
			if seen[s.Name] { // validation already rejects cycles; belt and braces
				continue
			}
			if f := fragments.ForName(s.Name); f != nil {
				seen[s.Name] = true
				d = selectionDepth(f.SelectionSet, fragments, depth, seen)
				delete(seen, s.Name)
			}
		default:
			panic(fmt.Sprintf("unexpected selection %T", s))
		}
		if d > deepest {
			deepest = d
		}
	}
	return deepest
}
