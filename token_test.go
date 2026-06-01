package gest

import (
	"reflect"
	"testing"
)

type tokenTestService struct{}

type tokenTestInterface interface {
	TokenTest()
}

func TestTokenOfPointerCapturesPointerType(t *testing.T) {
	token := TokenOf[*tokenTestService]()
	want := reflect.TypeOf((*tokenTestService)(nil))

	if token.Type != want {
		t.Fatalf("Type = %v, want %v", token.Type, want)
	}
	if token.Name != "" {
		t.Fatalf("Name = %q, want empty", token.Name)
	}
}

func TestTokenOfInterfaceCapturesInterfaceType(t *testing.T) {
	token := TokenOf[tokenTestInterface]()
	want := reflect.TypeOf((*tokenTestInterface)(nil)).Elem()

	if token.Type != want {
		t.Fatalf("Type = %v, want %v", token.Type, want)
	}
	if token.Type.Kind() != reflect.Interface {
		t.Fatalf("Kind = %v, want %v", token.Type.Kind(), reflect.Interface)
	}
}

func TestNamedHasNameAndNilType(t *testing.T) {
	token := Named("cache.redis")

	if token.Name != "cache.redis" {
		t.Fatalf("Name = %q, want %q", token.Name, "cache.redis")
	}
	if token.Type != nil {
		t.Fatalf("Type = %v, want nil", token.Type)
	}
}

func TestTokensCompareWithEqualOperator(t *testing.T) {
	first := TokenOf[*tokenTestService]()
	second := TokenOf[*tokenTestService]()

	if first != second {
		t.Fatalf("equal tokens did not compare with ==")
	}
	if first == TokenOf[tokenTestService]() {
		t.Fatalf("pointer and non-pointer tokens compared equal")
	}
}

func TestTokenStringOutputIsStable(t *testing.T) {
	tests := []struct {
		name  string
		token Token
		want  string
	}{
		{
			name:  "zero",
			token: Token{},
			want:  "<zero token>",
		},
		{
			name:  "named",
			token: Named("cache.redis"),
			want:  `name:"cache.redis"`,
		},
		{
			name:  "type",
			token: TokenOf[*tokenTestService](),
			want:  "type:*gest.tokenTestService",
		},
		{
			name: "type and name",
			token: Token{
				Type: reflect.TypeOf((*tokenTestService)(nil)),
				Name: "primary",
			},
			want: `type:*gest.tokenTestService name:"primary"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.token.String(); got != test.want {
				t.Fatalf("String() = %q, want %q", got, test.want)
			}
		})
	}
}
