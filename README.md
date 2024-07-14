# FlowMinGo

[![Go Reference](https://pkg.go.dev/badge/github.com/zenovich/flowmingo.svg)](https://pkg.go.dev/github.com/zenovich/flowmingo)

FlowMinGo (output Flow Minimizer for Go) is a powerful and flexible package designed with the goal of simplifying the process of capturing output stream files in Go applications. FlowMinGo provides robust utilities to capture standard output and error streams as well as other outputs efficiently.

## Features

- **Capture Output Streams**: Easily capture and manipulate `stdout` and `stderr` streams or other file outputs.
- **Restore Original State**: Restore the original state of output streams after capturing easily.
- **Flexible Integration**: Integrate seamlessly with your existing Go projects.
- **No Dependencies**: FlowMinGo is a standalone package with no dependencies.
- **Non-Blocking**: FlowMinGo is non-blocking and doesn't interfere with the flow of your application.
- **High Performance**: Optimized for performance and minimal overhead.

## TODO

- **Make FlowMinGo thread-safe**.

## Installation

To install FlowMinGo, use `go get`:
```shell
go get github.com/zenovich/flowmingo
```

## Usage

### Capturing Standard Output

Let's start by capturing the standard output:

```go
package main

import (
	"fmt"
	"os"

	"github.com/zenovich/flowmingo"
)

func main() {
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
}
```

Output:
```
This will be captured
This will not be captured
captured: This will be captured
```

### Capturing Both Standard Outputs

FlowMinGo allows you to capture the output to `stdout` and `stderr` or any other output stream with ease:

```go
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/zenovich/flowmingo"
)

func main() {
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
}
```

Output:
```
captured: stdout: This will be captured
captured: stderr: This will be captured too
captured: stdout: This will be captured as well
```

### Capturing Custom Output

You can also capture custom output streams with FlowMinGo. Here's an example:

```go
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/zenovich/flowmingo"
)

func main() {
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
}
```

Output:
```
captured: This will be captured
writer got:
```

### Capturing Command Output

You can also capture the output of an external command with FlowMinGo. Here's an example:

```go
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/zenovich/flowmingo"
)

func main() {
	// Create pipes for out and err
	// This part is for the stdout
	cmdOutReader, cmdOutWriter, err := os.Pipe()
	if err != nil {
		panic(fmt.Sprintf("Error creating pipe: %s", err))
	}
	defer func() { _ = cmdOutReader.Close() }()
	defer func() { _ = cmdOutWriter.Close() }()

	// This part is for the stderr (optional)
	cmdErrReader, cmdErrWriter, err := os.Pipe()
	if err != nil {
		panic(fmt.Sprintf("Error creating pipe: %s", err))
	}
	defer func() { _ = cmdErrReader.Close() }()
	defer func() { _ = cmdErrWriter.Close() }()

	// Create a command to run
	cmd := exec.Command("echo", "test")

	// Set the command's stdout and stderr to the writer ends of the pipes
	cmd.Stdout = cmdOutWriter
	cmd.Stderr = cmdErrWriter // optional

	// Capture the command's stdout and stderr
	getOuts := flowmingo.Capture(cmdOutWriter, cmdErrWriter /*optional*/)

	// Run the command
	err = cmd.Run()
	if err != nil {
		panic(fmt.Sprintf("Error running the command: %s", err))
	}

	capturedOutput := getOuts(false)

	for _, chunk := range capturedOutput {
		source := "out"
		if chunk.OutFile == cmdErrWriter {
			source = "err"
		}

		fmt.Printf("captured: %s: %s", source, chunk.Chunk)
	}

}
```

Output:
```
captured: out: test
```

### Stackable Capturing

FlowMinGo allows you to stack multiple captures on top of each other. Let's see how it works:

```go
package main

import (
	"fmt"
	"os"

	"github.com/zenovich/flowmingo"
)

func main() {
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
}
```

Output:
```
This will be captured only by the first capture
```

## Contributing

We welcome contributions from the community! If you have any ideas, suggestions, or bug reports, please open an issue or submit a pull request on GitHub.

## License

FlowMinGo is licensed under the MIT License. See the `LICENSE` file for more details.

---
Automatically generated by [autoreadme](https://github.com/jimmyfrasche/autoreadme)

