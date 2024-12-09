package servers

import (
	"github.com/google/uuid"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
)

// newStatus returns a status.Status representing c and msg.
//
// details is a string or a map[string]string otherwise panic.
//
// When details is a string, it is used as the error message.
// When details is a map[string]string, it is used as the details of validator failure and
// message is set as http.StatusText(runtime.HTTPStatusFromCode(c)).
func newStatus(c codes.Code, err error, details any) *status.Status {
	var (
		fieldViolations map[string]string
		msg             string
	)

	// Check if details is a string or a map[string]string
	switch det := details.(type) {
	case string:
		msg = det

	case map[string]string:
		msg = http.StatusText(runtime.HTTPStatusFromCode(c))
		fieldViolations = det
	default:
		msg = http.StatusText(runtime.HTTPStatusFromCode(c))
	}

	id := uuid.New().String()

	errInfo := &errdetails.ErrorInfo{
		Reason: err.Error(),
		Metadata: map[string]string{
			"error_id": id,
		},
	}

	if fieldViolations == nil {
		st, err := status.New(c, msg).WithDetails(errInfo)
		if err != nil {
			panic(err)
		}

		return st
	}

	for f, m := range fieldViolations {
		errInfo.Metadata[f] = m
	}

	st, err := status.New(c, msg).WithDetails(errInfo)
	if err != nil {
		panic(err)
	}

	return st
}

func Error(c codes.Code, err error, details ...any) error {
	if len(details) == 0 {
		return newStatus(c, err, nil).Err()
	}

	return newStatus(c, err, details[0]).Err()
}
