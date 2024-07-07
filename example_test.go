package flowmingo_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/zenovich/flowmingo"
)

// Example_captureStdout demonstrates how to capture stdout.
//
//nolint:nosnakecase // example for the package
func Example_captureStdout() {
	// Capture os.Stdout suppressing the output
	restore := flowmingo.Capture(os.Stdout)

	// Print something to stdout
	fmt.Println("This will be captured")

	// Restore the original output and get the captured output.
	// `true` means that the captured output should be passed through
	// to the original output file (os.Stdout here).
	capturedOutput := restore(true)

	// Print something to stdout again
	fmt.Println("This will not be captured")

	// Analyze the captured output
	fmt.Print("captured: ")

	for _, chunk := range capturedOutput {
		fmt.Printf("%s", chunk.Chunk)
	}
	// Output:
	// This will be captured
	// This will not be captured
	// captured: This will be captured
}

// ExampleCaptureStdoutAndStderr demonstrates how to capture stdout and stderr.
func ExampleCaptureStdoutAndStderr() {
	// capture os.Stdout and os.Stderr suppressing the output
	// (CaptureStdoutAndStderr captures both outputs in one call,
	// it's a shorthand for `Capture(os.Stdout, os.Stderr)`)
	restore := flowmingo.CaptureStdoutAndStderr()

	// print something to stdout
	fmt.Println("This will be captured")

	// FlowMinGo guarantees that the output is captured in the order
	// it was written within the same file, but the order of the chunks
	// from different files is not guaranteed, although it should be
	// very close to the order of the output.
	// That's the reason why we capture the output to both stdout and stderr
	// together in one call to CaptureStdoutAndStderr.
	// If we captured the output to stdout and stderr separately,
	// the order of the chunks from different files would be unknown.
	//
	// Here we sleep for a while to ensure that the output to stdout
	// is captured before the output to stderr, just for the sake of this example.
	time.Sleep(10 * time.Millisecond)

	// Print something to stderr
	fmt.Fprintln(os.Stderr, "This will be captured too")

	// Here we sleep for the same reason as above
	time.Sleep(10 * time.Millisecond)

	// Print something to stdout again
	fmt.Println("This will be captured as well")

	// Restore the original output and get the captured output
	// skipping the pass-through for this time
	capturedOutput := restore(false)

	for _, chunk := range capturedOutput {
		source := "stdout"
		if chunk.OutFile == os.Stderr {
			source = "stderr"
		}

		fmt.Printf("captured: %s: %s", source, chunk.Chunk)
	}
	// Output:
	// captured: stdout: This will be captured
	// captured: stderr: This will be captured too
	// captured: stdout: This will be captured as well
}

// Example_captureCustomOutputs demonstrates how to capture custom output streams.
//
//nolint:nosnakecase // example for the package
func Example_captureCustomOutputs() {
	// Create a pipe, so we can write to it and read from it.
	// The writer will be used as a custom output stream.
	// If this part is unclear, please refer to the Go documentation on the `os.Pipe` function,
	// for now, just assume that we have a custom output stream, like an opened file.
	reader, writer, err := os.Pipe()
	if err != nil {
		fmt.Println("Error creating pipe:", err)

		return
	}

	// Capture the custom output stream suppressing the output to it
	restore := flowmingo.Capture(writer)

	// Print something to the custom output stream
	fmt.Fprintln(writer, "This will be captured")

	// Restore the original output and get the captured output
	// skipping the pass-through
	capturedOutput := restore(false)

	// Analyze the captured output
	for _, chunk := range capturedOutput {
		fmt.Printf("captured: %s", chunk.Chunk)
	}

	// Close the writer to avoid a deadlock in the next step
	_ = writer.Close()

	// Here we read from the reader to demonstrate that the writer got the empty output
	// as the output was suppressed and not passed through.
	var whatWriterGot bytes.Buffer

	_, err = io.Copy(&whatWriterGot, reader)
	if err != nil {
		fmt.Println("Error reading from pipe:", err)

		return
	}

	// Analyze what the writer got
	fmt.Printf("writer got: %s\n", whatWriterGot.String())

	// Close the reader
	_ = reader.Close()
	// Output:
	// captured: This will be captured
	// writer got:
}

// Example_stackableCapturing demonstrates how to stack multiple captures.
//
//nolint:nosnakecase // example for the package
func Example_stackableCapturing() {
	// Capture os.Stdout suppressing the output
	restore1 := flowmingo.Capture(os.Stdout)
	// Capture os.Stdout again suppressing the output to the first capture
	restore2 := flowmingo.Capture(os.Stdout)

	// Print something to stdout
	fmt.Println("This will be captured only by the second capture")

	// Restore the second capture skipping the pass-through to the first capture
	restore2(false)

	// Print something to stdout again
	fmt.Println("This will be captured only by the first capture")

	// Restore the first capture passing the captured output to the original output file
	restore1(true)
	// Output:
	// This will be captured only by the first capture
}
