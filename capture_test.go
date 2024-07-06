package flowmingo_test

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"github.com/zenovich/flowmingo"
)

func TestCaptureStdoutAndStderr_CapturesAllWritesInChronologicalOrder_WithOutput(t *testing.T) {
	for _, doOutput := range []bool{true, false} {
		doOutput := doOutput
		testName := "WithOutput"
		if !doOutput {
			testName = "WithoutOutput"
		}
		t.Run(testName, func(t *testing.T) {
			origStdout := os.Stdout
			origStderr := os.Stderr

			defer func() {
				os.Stdout = origStdout
				os.Stderr = origStderr
			}()

			outR, outW, err := os.Pipe()
			assertNoError(t, err)
			os.Stdout = outW

			errR, errW, err := os.Pipe()
			assertNoError(t, err)
			os.Stderr = errW

			restore := flowmingo.CaptureStdoutAndStderr()
			_, _ = os.Stdout.WriteString("ab")
			time.Sleep(10 * time.Millisecond)
			_, _ = os.Stderr.WriteString("cde")
			time.Sleep(10 * time.Millisecond)
			_, _ = os.Stdout.WriteString("fgh")
			time.Sleep(10 * time.Millisecond)
			_, _ = os.Stderr.WriteString("i")
			time.Sleep(10 * time.Millisecond)
			_, _ = os.Stdout.WriteString("jkl")

			chunks := restore(doOutput)

			assertEqualInts(t, 5, len(chunks))
			assertEqualStrings(t, "ab", string(chunks[0].Chunk))
			assertEqualStrings(t, "cde", string(chunks[1].Chunk))
			assertEqualStrings(t, "fgh", string(chunks[2].Chunk))
			assertEqualStrings(t, "i", string(chunks[3].Chunk))
			assertEqualStrings(t, "jkl", string(chunks[4].Chunk))
			assertEqualFiles(t, os.Stdout, chunks[0].OutFile)
			assertEqualFiles(t, os.Stderr, chunks[1].OutFile)
			assertEqualFiles(t, os.Stdout, chunks[2].OutFile)
			assertEqualFiles(t, os.Stderr, chunks[3].OutFile)
			assertEqualFiles(t, os.Stdout, chunks[4].OutFile)

			_ = outW.Close()
			var outBuf bytes.Buffer
			_, err = io.Copy(&outBuf, outR)
			assertNoError(t, err)
			expectedStdout := "abfghjkl"
			if !doOutput {
				expectedStdout = ""
			}
			assertEqualStrings(t, expectedStdout, outBuf.String())

			_ = errW.Close()
			var errBuf bytes.Buffer
			_, err = io.Copy(&errBuf, errR)
			assertNoError(t, err)
			expectedStderr := "cdei"
			if !doOutput {
				expectedStderr = ""
			}
			assertEqualStrings(t, expectedStderr, errBuf.String())
		})
	}
}

func TestCapture_Nil(t *testing.T) {
	assertPanics(t, func() { flowmingo.Capture(nil) })
}

func TestRestoreFunc_DoesntAllowToBeCalledTwice(t *testing.T) {
	restoreFunc := flowmingo.Capture(os.Stdout)
	restoreFunc(true)
	assertPanics(t, func() { restoreFunc(true) })
}

func TestRestoreFunc_ChecksIfOutputIsReplaced(t *testing.T) {
	restoreFunc1 := flowmingo.CaptureStdoutAndStderr()
	restoreFunc2 := flowmingo.Capture(os.Stderr)

	assertPanics(t, func() { restoreFunc1(true) })

	restoreFunc2(false)
	restoreFunc1(true)
}

func assertEqualStrings(t *testing.T, expected, actual string) {
	if expected != actual {
		t.Errorf("Not equal: \n"+
			"expected: %s\n"+
			"actual  : %s", expected, actual)
	}
}

func assertEqualInts(t *testing.T, expected, actual int) {
	if expected != actual {
		t.Errorf("Not equal: \n"+
			"expected: %d\n"+
			"actual  : %d", expected, actual)
	}
}

func assertEqualFiles(t *testing.T, expected, actual *os.File) {
	if expected != actual {
		t.Errorf("Not equal: \n"+
			"expected: %v\n"+
			"actual  : %v", expected, actual)
	}
}

func assertNoError(t *testing.T, err error) {
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
}

func assertPanics(t *testing.T, f func()) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	f()
}
