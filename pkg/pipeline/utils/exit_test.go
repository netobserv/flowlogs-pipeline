package utils

import (
	"fmt"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_SetupElegantExit(t *testing.T) {
	SetupElegantExit()
	require.Equal(t, 0, len(registeredChannels))
	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	ch3 := make(chan struct{})
	RegisterExitChannel(ch1)
	require.Equal(t, 1, len(registeredChannels))
	RegisterExitChannel(ch2)
	require.Equal(t, 2, len(registeredChannels))
	RegisterExitChannel(ch3)
	require.Equal(t, 3, len(registeredChannels))

	select {
	case <-ch1:
		// should not get here
		require.Error(t, fmt.Errorf("channel should have been empty"))
	default:
		break
	}

	// send signal and see that it is propagated
	err := syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	require.Equal(t, nil, err)

	select {
	case <-ch1:
		break
	default:
		// should not get here
		require.Error(t, fmt.Errorf("channel should not be empty"))
		break
	}
}
