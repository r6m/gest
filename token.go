package gest

import (
	"reflect"
	"strconv"
)

// Token identifies a provider by reflect type, optional name, or both.
//
// Token values are comparable with ==. TokenOf[SomeInterface]() captures the
// interface type itself, TokenOf[*Service]() captures the pointer type, and
// pointer/non-pointer types are intentionally distinct. Named tokens have no
// type unless one is set explicitly.
type Token struct {
	Type reflect.Type
	Name string
}

// TokenOf returns a token for T.
func TokenOf[T any]() Token {
	return Token{Type: reflect.TypeFor[T]()}
}

// Named returns a name-only token.
func Named(name string) Token {
	return Token{Name: name}
}

// String returns a deterministic diagnostic representation of the token.
func (t Token) String() string {
	switch {
	case t.Type == nil && t.Name == "":
		return "<zero token>"
	case t.Type == nil:
		return "name:" + strconv.Quote(t.Name)
	case t.Name == "":
		return "type:" + t.Type.String()
	default:
		return "type:" + t.Type.String() + " name:" + strconv.Quote(t.Name)
	}
}
