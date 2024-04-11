package webbridge

import (
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/status"
)

func writeError(w http.ResponseWriter, err error) {
	st := status.Convert(err)

	respStatus := runtime.HTTPStatusFromCode(st.Code())
	respText := st.Message()

	if s, ok := err.(interface{ HTTPStatus() int }); ok {
		respStatus = s.HTTPStatus()
	}

	http.Error(w, respText, respStatus)
}
