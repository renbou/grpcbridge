package httprule

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Test_tokenize tests the tokenizer for HttpRule templates.
func Test_tokenize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tmpl string
		want []string
	}{
		{"", []string{eof}},
		{"alpha", []string{"alpha", eof}},
		{"alpha/beta", []string{"alpha", "/", "beta", eof}},
		{"alpha/*/test", []string{"alpha", "/", "*", "/", "test", eof}},
		{"alpha/test/**", []string{"alpha", "/", "test", "/", "**", eof}},
		{"{var}", []string{"{", "var", "}", eof}},
		{"test/{var=*}", []string{"test", "/", "{", "var", "=", "*", "}", eof}},
		{"v1/b/{var=nested/*}/extra", []string{"v1", "/", "b", "/", "{", "var", "=", "nested", "/", "*", "}", "/", "extra", eof}},
		{"v1/a/{var=a/*/b/**}:verb:kek", []string{"v1", "/", "a", "/", "{", "var", "=", "a", "/", "*", "/", "b", "/", "**", "}", ":verb:kek", eof}},
	}

	for _, tt := range tests {
		t.Run(tt.tmpl, func(t *testing.T) {
			// Act
			got := tokenize(tt.tmpl)

			// Assert
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("lex() returned lexemes differing from expected (-want +got)\n%s", diff)
			}
		})
	}
}
