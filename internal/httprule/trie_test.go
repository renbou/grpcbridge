package httprule

import (
	"fmt"
	"net/http"
	"testing"
)

// Test_Trie tests that Trie properly routes different combinations of method/path with various HttpRule templates.
func Test_Trie(t *testing.T) {
	t.Parallel()

	// Arrange
	rules := map[string][]string{
		http.MethodGet:  {"/", "/v1/users", "/v1/users:watch", "/v1/users/{id=*}", "/v1/objects/{path=**}"},
		http.MethodPost: {"/v1/users", "/v1/objects", "/v1/users/{id}:check"},
	}

	templates := make(map[string][]*Template, len(rules))
	for method, rules := range rules {
		for _, rule := range rules {
			tmpl, err := Parse(rule)
			if err != nil {
				t.Fatalf("failed to parse rule %q: %v", rule, err)
			}

			templates[method] = append(templates[method], tmpl)
		}
	}

	trie := NewTrie()
	for method, tmpls := range templates {
		for _, tmpl := range tmpls {
			trie.Add(method, tmpl)
		}
	}

	tests := []struct {
		method string
		path   string
		want   string
	}{
		{
			method: http.MethodGet,
			path:   "/",
			want:   "/",
		},
		{
			method: http.MethodGet,
			path:   "/v1/users",
			want:   "/v1/users",
		},
		{
			method: http.MethodGet,
			path:   "/v1/users/123",
			want:   "/v1/users/{id=*}",
		},
		{
			method: http.MethodGet,
			path:   "/v1/users:watch",
			want:   "/v1/users:watch",
		},
		{
			method: http.MethodGet,
			path:   "/v1/objects/a/b/c",
			want:   "/v1/objects/{path=**}",
		},
		{
			method: http.MethodPost,
			path:   "/v1/users",
			want:   "/v1/users",
		},
		{
			method: http.MethodPost,
			path:   "/v1/objects",
			want:   "/v1/objects",
		},
		{
			method: http.MethodPost,
			path:   "/v1/users/123:check",
			want:   "/v1/users/{id}:check",
		},
		{
			method: http.MethodDelete,
			path:   "/v1/tasks",
			want:   "",
		},
		{
			method: http.MethodPost,
			path:   "/v1/users/123",
			want:   "",
		},
		{
			method: http.MethodGet,
			path:   "/what",
			want:   "",
		},
		{
			method: http.MethodGet,
			path:   "/v1//users",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.method, tt.path), func(t *testing.T) {
			t.Parallel()

			// Act
			got, ok := trie.Find(tt.method, tt.path)

			// Assert
			if tt.want != "" && !ok {
				t.Fatalf("Find() returned not ok, but expected to find template for %q %q", tt.method, tt.path)
			} else if ok && got.tmpl != tt.want {
				t.Fatalf("Find() returned template %q, but expected %q", got.tmpl, tt.want)
			}
		})
	}
}
