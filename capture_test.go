package flowmingo_test

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
			assert.NoError(t, err)
			os.Stdout = outW

			errR, errW, err := os.Pipe()
			assert.NoError(t, err)
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

			assert.Equal(t, 5, len(chunks))
			assert.Equal(t, "ab", string(chunks[0].Chunk))
			assert.Equal(t, "cde", string(chunks[1].Chunk))
			assert.Equal(t, "fgh", string(chunks[2].Chunk))
			assert.Equal(t, "i", string(chunks[3].Chunk))
			assert.Equal(t, "jkl", string(chunks[4].Chunk))
			assert.Equal(t, os.Stdout, chunks[0].OutFile)
			assert.Equal(t, os.Stderr, chunks[1].OutFile)
			assert.Equal(t, os.Stdout, chunks[2].OutFile)
			assert.Equal(t, os.Stderr, chunks[3].OutFile)
			assert.Equal(t, os.Stdout, chunks[4].OutFile)

			_ = outW.Close()
			var outBuf bytes.Buffer
			_, err = io.Copy(&outBuf, outR)
			assert.NoError(t, err)
			expectedStdout := "abfghjkl"
			if !doOutput {
				expectedStdout = ""
			}
			assert.Equal(t, expectedStdout, outBuf.String())

			_ = errW.Close()
			var errBuf bytes.Buffer
			_, err = io.Copy(&errBuf, errR)
			assert.NoError(t, err)
			expectedStderr := "cdei"
			if !doOutput {
				expectedStderr = ""
			}
			assert.Equal(t, expectedStderr, errBuf.String())
		})
	}
}

func TestRestoreFunc_DoesntAllowToBeCalledTwice(t *testing.T) {
	restoreFunc := flowmingo.Capture(os.Stdout)
	restoreFunc(true)
	assert.Panics(t, func() { restoreFunc(true) })
}

func TestRestoreFunc_ChecksIfOutputIsReplaced(t *testing.T) {
	restoreFunc1 := flowmingo.CaptureStdoutAndStderr()
	restoreFunc2 := flowmingo.Capture(os.Stderr)

	assert.Panics(t, func() { restoreFunc1(true) })

	restoreFunc2(false)
	restoreFunc1(true)
}
