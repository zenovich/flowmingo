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
	"sync/atomic"
	"unsafe"
)

// ChunkFromFile represents a chunk of bytes that was captured from an output file.
type ChunkFromFile struct {
	Chunk   []byte
	OutFile *os.File
}

var captureLock sync.Mutex

func pipeReader(rStream io.ReadCloser, outC chan<- *ChunkFromFile, finishCh chan<- bool, outFile *os.File) {
	reader := bufio.NewReader(rStream)

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

	_ = rStream.Close()
}

// RestoreFunc is a function that stops capturing, restores the pointers to original output files and returns the captured output.
// The boolean parameter indicates whether the captured output should be written to the original output files.
type RestoreFunc func(passThroughOuts bool) []ChunkFromFile

var hookBetweenRestoreCheckAndRestore func()

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

	var chunksFromPipesLock sync.RWMutex

	var needPassThrough bool

	var chunksFromPipes []ChunkFromFile

	outC := make(chan *ChunkFromFile)
	finishCh := make(chan bool)

	origOutFiles := make([]os.File, len(outFiles))
	outWFiles := make([]*os.File, len(outFiles))
	outFilesOrigMap := make(map[*os.File]os.File, len(outFiles))

	for i := range outFiles {
		if outFiles[i] == nil {
			panic(fmt.Sprintf("output file #%d is nil, nil pointers are not allowed", i))
		}
	}

	for outFileNumber, outFile := range outFiles {
		outFile := outFile

		outR, outW, _ := os.Pipe()
		outWFiles[outFileNumber] = outW

		replaceOutFile(outFile, outW, &origOutFiles[outFileNumber])
		outFilesOrigMap[outFile] = origOutFiles[outFileNumber]

		go pipeReader(outR, outC, finishCh, outFile)
	}

	go func() {
		for {
			chunkFromPipe := <-outC
			if chunkFromPipe == nil {
				finishCh <- true

				return
			}

			chunksFromPipesLock.Lock()
			chunksFromPipes = append(chunksFromPipes, *chunkFromPipe)

			// Pass the chunk to the original output files for the case
			// when the restore function is called with passThroughOuts=true,
			// and it has already flushed all the previous chunks,
			// but hasn't closed the outWFiles yet.
			//
			// Since there is a tiny time window between restoring the out files and closing the outWFiles,
			// there can be goroutines that have already started writing to the restored out files concurrently.
			// This means that the chunks that were captured after the restore function was called
			// can be written to the original output files after more recent concurrent writes.
			// Note: it's only related to the writes happening after the restore function restored the out files and
			// before the restore function closed outWFiles.
			if needPassThrough {
				origOutPipe := outFilesOrigMap[chunkFromPipe.OutFile]
				_, _ = origOutPipe.Write(chunkFromPipe.Chunk)
			}
			chunksFromPipesLock.Unlock()
		}
	}()

	return func(passThroughOuts bool) []ChunkFromFile {
		captureLock.Lock()
		defer captureLock.Unlock()

		if outC == nil {
			panic(fmt.Sprintf("Capture function was already called for output files %v\n", origOutFiles))
		}

		// flush the already captured chunks to the original output files before restoring out files
		if passThroughOuts {
			chunksFromPipesLock.RLock()
			flushChunksToOrigPipes(chunksFromPipes, outFilesOrigMap)

			needPassThrough = true
			chunksFromPipesLock.RUnlock()
		}

		restoreOutFiles(outFiles, outWFiles, origOutFiles)

		for _, outW := range outWFiles {
			_ = outW.Close()

			<-finishCh // wait for the out pipe reader to finish
		}

		outC <- nil // for old Golang versions

		<-finishCh // wait for the outC reader to finish
		close(outC)
		outC = nil

		close(finishCh)

		return chunksFromPipes
	}
}

func replaceOutFile(outFile, outW, origOutFileToStore *os.File) {
	// Note that we replace the contents of the pointer, not the pointer itself
	//nolint:gosec // *outFile = *outW
	origOutFile := atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(outFile)), *(*unsafe.Pointer)(unsafe.Pointer(outW)))

	//nolint:gosec // old *outFile
	*origOutFileToStore = *(*os.File)(unsafe.Pointer(&origOutFile))
}

func restoreOutFiles(outFiles, outWFiles []*os.File, origOutFiles []os.File) {
	for i, outFile := range outFiles {
		//nolint:gosec // *outFile
		loadedOutFile := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(outFile)))
		//nolint:gosec // *outFile != *outWFiles[i]
		if loadedOutFile != *(*unsafe.Pointer)(unsafe.Pointer(outWFiles[i])) {
			panic(fmt.Sprintf("cannot restore because original out file #%d was changed from the outside", i))
		}
	}

	// Note: An original out file can be replaced by a concurrent goroutine between the check above and the code below,
	// but we assume that it is highly unlikely. Our package prevents that by locking the captureLock, but there is no way
	// to prevent that in the user code. In such a case, some output files can be left with the pipes attached if the code below panics.

	// We want to be able to test this case though
	if hookBetweenRestoreCheckAndRestore != nil {
		hookBetweenRestoreCheckAndRestore()
	}

	for outFileNumber, outFile := range outFiles {
		// Note that we replace the contents of the pointers, not the pointers themselves
		//nolint:gosec // *outFile = origOutFiles[outFileNumber]
		if !atomic.CompareAndSwapPointer(
			(*unsafe.Pointer)(unsafe.Pointer(outFile)),
			*(*unsafe.Pointer)(unsafe.Pointer(outWFiles[outFileNumber])),
			*(*unsafe.Pointer)(unsafe.Pointer(&origOutFiles[outFileNumber]))) {
			// Highly unlikely case
			panic(fmt.Sprintf("cannot restore because original out file #%d was changed from the outside", outFileNumber))
		}
	}
}

func flushChunksToOrigPipes(chunks []ChunkFromFile, outFilesOrigMap map[*os.File]os.File) {
	for _, chunk := range chunks {
		origOutPipe := outFilesOrigMap[chunk.OutFile]
		_, _ = origOutPipe.Write(chunk.Chunk)
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
