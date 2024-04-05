package bridgetest

import (
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StatusCodeIs checks that the error has the specified gRPC status code.
func StatusCodeIs(err error, wantCode codes.Code) error {
	if gotCode := status.Code(err); gotCode != wantCode {
		return fmt.Errorf("got status code = %s, want %s", gotCode, wantCode)
	}

	return nil
}

// StatusIs checks that the error matches the specified gRPC status.
func StatusIs(err error, wantStatus *status.Status) error {
	if codeErr := StatusCodeIs(err, wantStatus.Code()); codeErr != nil {
		return codeErr
	}

	if gotStatus := status.Convert(err); !strings.Contains(gotStatus.Message(), wantStatus.Message()) {
		return fmt.Errorf("got status message = %q, want %q", gotStatus.Message(), wantStatus.Message())
	}

	return nil
}
