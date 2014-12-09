// Copyright 2014 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ftrace

import (
	"fmt"
	"io"
	"os"
	"syscall"
)

const (
	perCpuRawPipeFmt = "per_cpu/cpu%d/trace_pipe_raw"
)

// Returns a channel that provides [page size]byte chunks from a cpu raw ftrace pipe
// Write to doneCh to end
func getRawFtraceChan(fp FileProvider, cpu int, doneCh <-chan bool) (<-chan []byte, error) {
	ch := make(chan []byte)

	f, err := fp.OpenFtrace(fmt.Sprintf(perCpuRawPipeFmt, cpu))
	if err != nil {
		return nil, err
	}

	go func() {
		defer f.Close()
		defer close(ch)

		for {
			var buf = make([]byte, syscall.Getpagesize())
			n, err := f.Read(buf)
			if e, ok := err.(*os.PathError); ok && e.Err == syscall.EINTR {
				continue
			}
			if err == io.EOF || err != nil || n == 0 {
				fmt.Println(err)
				// TODO: error over channel?
				break
			}

			select {
			case <-doneCh:
				// This goroutine may be blocked in the Read above, so this may never fire if no
				// trace events are pending
				break
			case ch <- buf[0:n]:
			}
		}
	}()

	return ch, nil
}
