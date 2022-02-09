package utils

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"syscall"
	"testing"
)

func Test_SetupElegantExit(t *testing.T) {
	SetupElegantExit()
	require.Equal(t, 0, len(registeredChannels))
	ch1 := make(chan bool, 1)
	ch2 := make(chan bool, 1)
	ch3 := make(chan bool, 1)
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
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)

	select {
	case <-ch1:
		break
	default:
		// should not get here
		require.Error(t, fmt.Errorf("channel should not be empty"))
		break
	}
}
