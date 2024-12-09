package servers

import (
	"github.com/google/uuid"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
)

const errorMessageInvalidArgument = "invalid argument"

// Status represents an RPC status code, message, and details.
type Status struct {
	*status.Status

	id  string
	err error
}

// NewStatus returns a Status representing code, error and details.
//
// details is a string or a map[string]string otherwise panic.
//
// When details is a string, it is used as the error message.
// When details is a map[string]string, it is used as the details of validator failure and
// message is set as errorMessageInvalidArgument.
func NewStatus(c codes.Code, err error, details any) *Status {
	var fieldViolations map[string]string

	// Check if details is a string or a map[string]string
	switch det := details.(type) {
	case string:
		return &Status{
			id:     uuid.New().String(),
			Status: status.New(c, det),
			err:    err,
		}

	case map[string]string:
		fieldViolations = det
	default:
		return &Status{
			id:     uuid.New().String(),
			Status: status.New(c, http.StatusText(runtime.HTTPStatusFromCode(c))),
			err:    err,
		}
	}

	br := &errdetails.BadRequest{}

	for f, m := range fieldViolations {
		br.FieldViolations = append(br.FieldViolations, &errdetails.BadRequest_FieldViolation{
			Field:       f,
			Description: m,
		})
	}

	st, err := status.New(c, errorMessageInvalidArgument).WithDetails(br)
	if err != nil {
		panic(err)
	}

	return &Status{
		Status: st,
		err:    err,
	}
}

func (s *Status) Error() string {
	return s.Message()
}

// Is implements future error.Is functionality.
// An Error is equivalent if err message identical.
func (s *Status) Is(err error) bool {
	return s.err.Error() == err.Error()
}

// Unwrap implements errors.Unwrap for Error.
func (s *Status) Unwrap() error {
	return s.err
}

// ID returns the unique identifier of the error.
func (s *Status) ID() string {
	return s.id
}

// Tuples returns structured data of error in form of loosely-typed key-value pairs.
func (s *Status) Tuples() []any {
	return []any{
		"error_id", s.id,
		"error", s.err,
	}
}

// Fields returns structured data of error as a map.
func (s *Status) Fields() map[string]any {
	return map[string]any{
		"error_id": s.id,
		"error":    s.Error(),
	}
}
