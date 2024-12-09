package servers_test

import (
	"errors"
	"github.com/dohernandez/servers"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"testing"
)

func TestNewStatus(t *testing.T) {
	st := servers.NewStatus(codes.AlreadyExists, errors.New("test"), nil)

	require.Equal(t, codes.AlreadyExists, st.Code())
	require.Equal(t, "Conflict", st.Message())
	require.Equal(t, []any{}, st.Details())

	require.Equal(t, "Conflict", st.Error())
	require.ErrorIs(t, st, errors.New("test"))
}
