package flowmingo

import (
	"os"
	"testing"
)

func TestRestoreFunc_ChecksIfOutputIsReplacedRightAfterChecking(t *testing.T) {
	restoreFunc1 := CaptureStdoutAndStderr()
	restoreFunc2 := Capture(os.Stderr)

	// This call should panic before restoring anything
	assertPanics(t, func() { restoreFunc1(true) })

	restoreFunc2(false)

	hookBetweenRestoreCheckAndRestore = func() {
		*os.Stderr = *os.Stdout
	}
	defer func() { hookBetweenRestoreCheckAndRestore = nil }()

	// This call should panic right after restoring stdout and before restoring stderr
	assertPanics(t, func() { restoreFunc1(true) })
}

func assertPanics(t *testing.T, funcToCall func()) {
	t.Helper()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	funcToCall()
}
