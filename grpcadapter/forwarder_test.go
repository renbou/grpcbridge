package grpcadapter

import (
	"math"
	"strconv"
	"testing"
	"time"
)

// Test_decodeTimeout covers the grpc-timeout header decoding mechanism according to the PROTOCOL-HTTP2 spec.
func Test_decodeTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  time.Duration
		ok    bool
	}{
		{value: "10S", want: 10 * time.Second, ok: true},
		{value: "13M", want: 13 * time.Minute, ok: true},
		{value: "4H", want: 4 * time.Hour, ok: true},
		{value: "111m", want: 111 * time.Millisecond, ok: true},
		{value: "42u", want: 42 * time.Microsecond, ok: true},
		{value: "1337n", want: 1337 * time.Nanosecond, ok: true},
		{value: "1337k", want: 0, ok: false},
		{value: "1", want: 0, ok: false},
		{value: "111111111111s", want: 0, ok: false}, // grpc-timeout specifies limit of at most 8 chars
		{value: "10s,10m", want: 0, ok: false},
		{value: strconv.FormatInt(math.MaxInt64/int64(time.Hour)*2, 10) + "H", want: math.MaxInt64, ok: true}, // overflow
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			// Act
			got, ok := decodeTimeout(tt.value)

			// Assert
			if got != tt.want {
				t.Errorf("decodeTimeout(%q) returned duration = %v, want %v", tt.value, got, tt.want)
			}

			if ok != tt.ok {
				t.Errorf("decodeTimeout(%q) returned ok = %v, want %v", tt.value, ok, tt.ok)
			}
		})
	}
}
