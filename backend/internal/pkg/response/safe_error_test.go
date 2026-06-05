package response

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSafeErrorMsgReturnsFallbackForUnrecognizedErrors(t *testing.T) {
	msg := SafeErrorMsg(errors.New("dial tcp 10.0.0.1:5432: i/o timeout"), "safe")
	require.Equal(t, "safe", msg)
}
