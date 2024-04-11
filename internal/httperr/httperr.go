package httperr

// StatusError is the default grpcbridge error implementation of  interface { HTTPStatus() int },
// used by the routing and transcoding components to customize returned HTTP status codes.
type StatusError struct {
	Code int
	Err  error
}

func Status(code int, err error) *StatusError {
	return &StatusError{Code: code, Err: err}
}

func (e *StatusError) Error() string {
	return e.Err.Error()
}

func (e *StatusError) Unwrap() error {
	return e.Err
}

func (e *StatusError) HTTPStatus() int {
	return e.Code
}
