package utils

import (
	"fmt"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_SetupElegantExit(t *testing.T) {
	SetupElegantExit()

	select {
	case <-ExitChannel():
		// should not get here
		require.Error(t, fmt.Errorf("channel should have been empty"))
	default:
		break
	}

	// send signal and see that it is propagated
	err := syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	require.Equal(t, nil, err)

	select {
	case <-ExitChannel():
		break
	default:
		// should not get here
		require.Error(t, fmt.Errorf("channel should not be empty"))
		break
	}
}
