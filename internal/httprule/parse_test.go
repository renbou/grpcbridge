package httprule

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func sWildcard() segment {
	return segment{typ: segmentWildcard}
}

func sMultiWildcard() segment {
	return segment{typ: segmentMultiWildcard}
}

func sLiteral(s string) segment {
	return segment{typ: segmentLiteral, literal: s}
}

func sVariable(path []string, segments []segment) segment {
	return segment{typ: segmentVariable, variable: variable{fieldPath: path, segments: segments}}
}

// Test_Parse_Ok tests successful HttpRule parsing cases.
func Test_Parse_Ok(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tmpl string
		want *Template
	}{
		{
			tmpl: "/",
			want: &Template{
				tmpl:     "/",
				segments: []segment{sLiteral("")},
			},
		},
		{
			tmpl: "/alpha",
			want: &Template{
				tmpl:     "/alpha",
				segments: []segment{sLiteral("alpha")},
			},
		},
		{
			tmpl: "/alpha/beta",
			want: &Template{
				tmpl:     "/alpha/beta",
				segments: []segment{sLiteral("alpha"), sLiteral("beta")},
			},
		},
		{
			tmpl: "/alpha/*/test",
			want: &Template{
				tmpl:     "/alpha/*/test",
				segments: []segment{sLiteral("alpha"), sWildcard(), sLiteral("test")},
			},
		},
		{
			tmpl: "/alpha/test/**",
			want: &Template{
				tmpl:     "/alpha/test/**",
				segments: []segment{sLiteral("alpha"), sLiteral("test"), sMultiWildcard()},
			},
		},
		{
			tmpl: "/{var}",
			want: &Template{
				tmpl:     "/{var}",
				segments: []segment{sVariable([]string{"var"}, []segment{sWildcard()})},
			},
		},
		{
			tmpl: "/test/{var=*}",
			want: &Template{
				tmpl:     "/test/{var=*}",
				segments: []segment{sLiteral("test"), sVariable([]string{"var"}, []segment{sWildcard()})},
			},
		},
		{
			tmpl: "/v1/b/{var.b.inner=nested/*}/extra",
			want: &Template{
				tmpl:     "/v1/b/{var.b.inner=nested/*}/extra",
				segments: []segment{sLiteral("v1"), sLiteral("b"), sVariable([]string{"var", "b", "inner"}, []segment{sLiteral("nested"), sWildcard()}), sLiteral("extra")},
			},
		},
		{
			tmpl: "/v1/a/{vA_r1=a/*/b/**}:verb:kek",
			want: &Template{
				tmpl:     "/v1/a/{vA_r1=a/*/b/**}:verb:kek",
				segments: []segment{sLiteral("v1"), sLiteral("a"), sVariable([]string{"vA_r1"}, []segment{sLiteral("a"), sWildcard(), sLiteral("b"), sMultiWildcard()})},
				verb:     "verb:kek",
			},
		},
		{
			tmpl: "/T-E_S.T~/b;()%01/c%0a%AB:verb:test",
			want: &Template{
				tmpl:     "/T-E_S.T~/b;()%01/c%0a%AB:verb:test",
				segments: []segment{sLiteral("T-E_S.T~"), sLiteral("b;()%01"), sLiteral("c%0a%AB:verb")},
				verb:     "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.tmpl, func(t *testing.T) {
			// Act
			got, err := Parse(tt.tmpl)
			if err != nil {
				t.Fatalf("Parse() returned non-nil error = %q", err)
			}

			// Assert
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(Template{}, segment{}, variable{})); diff != "" {
				t.Errorf("Parse() returned result differing from expected (-want +got)\n%s", diff)
			}
		})
	}
}

// Test_Parse_Error tests HttpRule parsing error cases.
func Test_Parse_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tmpl        string
		wantMessage string
	}{
		{
			tmpl:        "what",
			wantMessage: `template "what" does not contain leading /`,
		},
		{
			tmpl:        "/%ax",
			wantMessage: `unexpected token "%ax" after segments "/"`,
		},
		{
			tmpl:        "/{.=*}",
			wantMessage: `unexpected token "." after segments "/{"`,
		},
		{
			tmpl:        "/{1=*}",
			wantMessage: `unexpected token "1" after segments "/{"`,
		},
		{
			tmpl:        "/{var}kek",
			wantMessage: `unexpected token "kek" after segments "/{var}"`,
		},
		{
			tmpl:        "/{var}:%0z",
			wantMessage: `unexpected token ":%0z" after segments "/{var}"`,
		},
		{
			tmpl:        "/{var",
			wantMessage: `unexpected token "\x00" after segments "/{var"`,
		},
		{
			tmpl:        "/{var.1asdf}",
			wantMessage: `unexpected token "1asdf" after segments "/{var."`,
		},
		{
			tmpl:        "/{var={kek}}",
			wantMessage: `unexpected token "{kek" after segments "/{var="`,
		},
		{
			tmpl:        "/kek{b}",
			wantMessage: `unexpected token "{" after segments "/kek"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.tmpl, func(t *testing.T) {
			// Act
			_, err := Parse(tt.tmpl)
			if err == nil {
				t.Fatalf("Parse() returned nil error")
			}

			// Assert
			if !strings.Contains(err.Error(), tt.wantMessage) {
				t.Errorf("Parse() returned unexpected error %q, want %q", err, tt.wantMessage)
			}
		})
	}
}
