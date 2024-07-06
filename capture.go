/*
Package flowmingo (output Flow Minimizer for Go) provides a way to capture the output to the given output files.
*/
package flowmingo

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"
)

// ChunkFromFile represents a chunk of bytes that was captured from an output file.
type ChunkFromFile struct {
	Chunk   []byte
	OutFile *os.File
}

var captureLock sync.Mutex

func pipeReader(r io.ReadCloser, outC chan<- *ChunkFromFile, finishCh chan<- bool, outFile *os.File) {
	reader := bufio.NewReader(r)
	for {
		readByte, err := reader.ReadByte()
		if err != nil {
			finishCh <- true
			break
		}
		buffered := reader.Buffered()
		bytesBlock := make([]byte, 0, buffered+1)
		bytesBlock = append(bytesBlock, readByte)
		for ; buffered > 0; buffered = reader.Buffered() {
			peeked, _ := reader.Peek(buffered)
			bytesBlock = append(bytesBlock, peeked...)
			_, _ = reader.Discard(len(peeked))
		}
		outC <- &ChunkFromFile{Chunk: bytesBlock, OutFile: outFile}
	}
	_ = r.Close()
}

// RestoreFunc is a function that stops capturing, restores the pointers to original output files and returns the captured output.
// The boolean parameter indicates whether the captured output should be written to the original output files.
type RestoreFunc func(passThroughOuts bool) []ChunkFromFile

// Capture captures the output to the given output files and returns a function
// for stopping capturing, restoring the pointers to original output files and getting the captured output.
//
// The output to the given output files is suppressed while capturing.
//
// The output is captured in "chunks from file", where each chunk contains a slice of bytes and the file
// it was supposed to be written to. The chunks are stored and returned in the order they were captured,
// not grouped by the file. This order should be very close to the order of the output.
// The order of the chunks within the same file is guaranteed.
//
// The function returned by Capture can be called with a boolean parameter that indicates whether the captured
// output should be written to the original output files.
//
// The function returned by Capture should be called only once.
//
// You can call Capture multiple times to capture the output to multiple files.
// You can even call Capture with the already captured output files to stack the captures.
// In this case, the returned "restore" functions should be called in the reverse order of the calls to Capture.
func Capture(outFiles ...*os.File) RestoreFunc {
	captureLock.Lock()
	defer captureLock.Unlock()

	outC := make(chan *ChunkFromFile)
	finishCh := make(chan bool)
	var chunksFromPipes []ChunkFromFile

	origOutFiles := make([]os.File, len(outFiles))
	outWFiles := make([]*os.File, len(outFiles))
	for i, outFile := range outFiles {
		outFile := outFile
		origOutFiles[i] = *outFile
		outR, outW, _ := os.Pipe()
		outWFiles[i] = outW
		// Note that we replace the contents of the pointer, not the pointer itself
		*outFile = *outW
		go pipeReader(outR, outC, finishCh, outFile)
	}

	go func() {
		for {
			chunkFromPipe := <-outC
			if chunkFromPipe == nil {
				finishCh <- true
				return
			}
			chunksFromPipes = append(chunksFromPipes, *chunkFromPipe)
		}
	}()

	return func(passThroughOuts bool) []ChunkFromFile {
		captureLock.Lock()
		defer captureLock.Unlock()

		if outC == nil {
			panic(fmt.Sprintf("Capture function was already called for output files%v\n", origOutFiles))
		}

		for i, outFile := range outFiles {
			if *outFile != *outWFiles[i] {
				panic(fmt.Sprintf("cannot restore because original out file #%d was changed from the outside", i))
			}
		}
		for i, outFile := range outFiles {
			// Note that we replace the contents of the pointers, not the pointers themselves
			*outFile = origOutFiles[i]
		}

		for _, outW := range outWFiles {
			_ = outW.Close()
			<-finishCh // wait for the out pipe reader to finish
		}

		outC <- nil // for old Golang versions
		<-finishCh  // wait for the outC reader to finish
		close(outC)
		outC = nil

		close(finishCh)

		if passThroughOuts {
			flushChunks(chunksFromPipes)
		}

		return chunksFromPipes
	}
}

func flushChunks(chunks []ChunkFromFile) {
	for _, chunk := range chunks {
		_, _ = chunk.OutFile.Write(chunk.Chunk)
	}
}

// CaptureStdoutAndStderr captures the output to STDOUT and STDERR and
// returns a function for stopping capturing, restoring the pointers
// to original output files and getting the captured output.
//
// The output to STDOUT and STDERR is suppressed while capturing.
//
// It's important to capture the output to both STDOUT and STDERR together
// in one call to CaptureStdoutAndStderr if you want to know relative order
// of the chunks written to different files.
//
// CaptureStdoutAndStderr is just a shorthand for `Capture(os.Stdout, os.Stderr)`.
// For more information, see the documentation of Capture.
func CaptureStdoutAndStderr() RestoreFunc {
	return Capture(os.Stdout, os.Stderr)
}
