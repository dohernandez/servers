package servers

import (
	"context"
	"errors"

	"github.com/bool64/ctxd"
	"github.com/google/uuid"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// Status references the status.Status from google.golang.org/grpc/status.
type Status struct {
	*status.Status

	err error
}

// Err returns an immutable error representing s; returns nil if s.Code() is OK.
func (s *Status) Err() error {
	if s.Code() == codes.OK {
		return nil
	}

	return &errSt{s: s}
}

// newStatus returns a Status representing c and msg.
//
// details is a string or a map[string]string otherwise panic.
//
// When details is a string, it is used as the error message.
// When details is a map[string]string, it is used as the details of validator failure and
// message is set as http.StatusText(runtime.HTTPStatusFromCode(c)).
func newStatus(c codes.Code, err error, details map[string]string) *Status {
	msg := err.Error()

	var lerr ctxd.SentinelError

	if errors.As(err, &lerr) {
		msg = lerr.Error()
	}

	id := uuid.New().String()

	errInfo := &errdetails.ErrorInfo{
		Reason: err.Error(),
		Metadata: map[string]string{
			"error_id": id,
		},
	}

	// Creating kvs for structured logging with all possible details.
	kvs := make([]any, 0, len(details)*2+2)

	// Adding the error id to the metadata of the error.
	kvs = append(kvs, "error_id", id)

	// Adding the error id to the metadata of the error.
	werr := errors.Unwrap(err)
	if werr == nil {
		werr = err
	}

	if len(details) == 0 {
		grpcst, err := status.New(c, msg).WithDetails(errInfo)
		if err != nil {
			panic(err)
		}

		return &Status{
			Status: grpcst,
			err:    ctxd.WrapError(context.Background(), werr, msg, kvs...),
		}
	}

	kvs = append(kvs, "details", details)

	for f, m := range details {
		errInfo.Metadata[f] = m
	}

	grpcst, err := status.New(c, msg).WithDetails(errInfo)
	if err != nil {
		panic(err)
	}

	return &Status{
		Status: grpcst,
		err:    ctxd.WrapError(context.Background(), werr, msg, kvs...),
	}
}

// Error creates a new error with the given code, message and details if this is provided.
// If more than one details are provided, they are merged into a single map.
func Error(c codes.Code, msg string, details ...map[string]string) error {
	if len(details) == 0 {
		return newError(c, ctxd.SentinelError(msg), details...)
	}

	return newError(c, ctxd.SentinelError(msg), details...)
}

func newError(c codes.Code, err error, details ...map[string]string) error {
	merge := make(map[string]string)

	for _, d := range details {
		for k, v := range d {
			merge[k] = v
		}
	}

	return newStatus(c, err, merge).Err()
}

// WrapError wraps an error with the given code, message and details if this is provided.
// If more than one details are provided, they are merged into a single map.
func WrapError(c codes.Code, err error, msg string, details ...map[string]string) error {
	err = ctxd.LabeledError(err, ctxd.SentinelError(msg))

	if len(details) == 0 {
		return newError(c, err, details...)
	}

	return newError(c, err, details...)
}

type errSt struct {
	s *Status
}

// Error returns the error message.
func (e *errSt) Error() string {
	return e.s.Message()
}

// GRPCStatus returns the Status represented by se.
func (e *errSt) GRPCStatus() *status.Status {
	return e.s.Status
}

// Is implements future error.Is functionality.
// A Error is equivalent if the code and message are identical.
func (e *errSt) Is(target error) bool {
	tse, ok := target.(*errSt)
	if !ok {
		return false
	}

	return proto.Equal(e.s.Status.Proto(), tse.s.Status.Proto())
}

// Unwrap implements errors wrapper.
func (e *errSt) Unwrap() error {
	return e.s.err
}

// Tuples returns structured data of error in form of loosely-typed key-value pairs.
func (e *errSt) Tuples() []any {
	var errStuctured ctxd.StructuredError

	if !errors.As(e.s.err, &errStuctured) {
		return nil
	}

	return errStuctured.Tuples()
}

// Fields returns structured data of error as a map.
func (e *errSt) Fields() map[string]any {
	var errStuctured ctxd.StructuredError

	if !errors.As(e.s.err, &errStuctured) {
		return nil
	}

	return errStuctured.Fields()
}
