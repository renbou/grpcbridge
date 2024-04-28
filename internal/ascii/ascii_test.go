// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ascii

import "testing"

func TestEqualFold(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{
			name: "empty",
			want: true,
		},
		{
			name: "simple match",
			a:    "CHUNKED",
			b:    "chunked",
			want: true,
		},
		{
			name: "same string",
			a:    "chunked",
			b:    "chunked",
			want: true,
		},
		{
			name: "Unicode Kelvin symbol",
			a:    "chunKed", // This "K" is 'KELVIN SIGN' (\u212A)
			b:    "chunked",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EqualFold(tt.a, tt.b); got != tt.want {
				t.Errorf("AsciiEqualFold(%q,%q): got %v want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
