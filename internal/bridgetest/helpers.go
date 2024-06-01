package bridgetest

import (
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StatusCodeIs checks that the error has the specified gRPC status code.
func StatusCodeIs(err error, wantCode codes.Code) error {
	if gotStatus := status.Convert(err); gotStatus.Code() != wantCode {
		return fmt.Errorf("got status code = %s (message = %q), want %s", gotStatus.Code(), gotStatus.Message(), wantCode)
	}

	return nil
}

// StatusCodeOneOf checks that the error has one of the specified gRPC status codes.
// This is useful for checks like Canceled/Unavailable, where one of these codes is valid for a closing connection.
func StatusCodeOneOf(err error, wantCodes ...codes.Code) error {
	for _, wantCode := range wantCodes {
		if StatusCodeIs(err, wantCode) == nil {
			return nil
		}
	}

	gotStatus := status.Convert(err)
	return fmt.Errorf("got status code = %s (message = %q), want one of %s", gotStatus.Code(), gotStatus.Message(), wantCodes)
}

// StatusIs checks that the error matches the specified gRPC status.
func StatusIs(err error, wantStatus *status.Status) error {
	if codeErr := StatusCodeIs(err, wantStatus.Code()); codeErr != nil {
		return codeErr
	}

	if gotStatus := status.Convert(err); !strings.Contains(gotStatus.Message(), wantStatus.Message()) {
		return fmt.Errorf("got status message = %q (code = %s), want %q", gotStatus.Message(), gotStatus.Code(), wantStatus.Message())
	}

	return nil
}
