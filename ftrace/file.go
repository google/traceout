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
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
)

type FileProvider interface {
	ReadFtraceFile(string) ([]byte, error)
	WriteFtraceFile(string, []byte) error
	ReadProcFile(string) ([]byte, error)
	OpenFtrace(string) (io.ReadCloser, error)
}

const ftracePath = "/sys/kernel/debug/tracing"
const procPath = "/proc"

var BadFtraceFileName error = errors.New("Bad file name")
var BadProcFileName error = errors.New("Bad file name")

type localFileProvider struct{}

func NewLocalFileProvider() FileProvider {
	return &localFileProvider{}
}

func (localFileProvider) ReadFtraceFile(filename string) ([]byte, error) {
	if !SafeFtracePath(filename) {
		return nil, BadFtraceFileName
	}
	return ioutil.ReadFile(path.Join(ftracePath, filename))
}

func (localFileProvider) ReadProcFile(filename string) ([]byte, error) {
	if !SafeProcPath(filename) {
		return nil, BadProcFileName
	}
	return ioutil.ReadFile(path.Join(procPath, filename))
}

func (localFileProvider) WriteFtraceFile(filename string, data []byte) error {
	if !SafeFtracePath(filename) {
		return BadFtraceFileName
	}
	return ioutil.WriteFile(path.Join(ftracePath, filename), data, 0)
}

func (localFileProvider) OpenFtrace(filename string) (io.ReadCloser, error) {
	if !SafeFtracePath(filename) {
		return nil, BadFtraceFileName
	}

	return os.Open(path.Join(ftracePath, filename))
}

// recordingFileProvider
type recordingFileProvider struct {
	FileProvider
	sync.Mutex
	files map[string]*recordedFileContents
}

func NewRecordingFileProvider(fp FileProvider) *recordingFileProvider {
	return &recordingFileProvider{
		FileProvider: fp,
		files:        make(map[string]*recordedFileContents),
	}
}

func (fp *recordingFileProvider) ReadFtraceFile(filename string) ([]byte, error) {
	buf, err := fp.FileProvider.ReadFtraceFile(filename)

	if err == nil {
		fp.Lock()
		fp.files[path.Join(ftracePath, filename)] = &recordedFileContents{
			buf: buf,
		}
		fp.Unlock()
	}

	return buf, err
}

func (fp *recordingFileProvider) ReadProcFile(filename string) ([]byte, error) {
	buf, err := fp.FileProvider.ReadProcFile(filename)

	if err == nil {
		fp.Lock()
		fp.files[path.Join(procPath, filename)] = &recordedFileContents{
			buf: buf,
		}
		fp.Unlock()
	}

	return buf, err
}

func (fp *recordingFileProvider) WriteFtraceFile(filename string, data []byte) error {
	return fp.FileProvider.WriteFtraceFile(filename, data)
}

func (fp *recordingFileProvider) OpenFtrace(filename string) (io.ReadCloser, error) {
	f, err := fp.FileProvider.OpenFtrace(filename)
	if err != nil {
		return f, err
	}

	contents := &recordedFileContents{}
	fp.Lock()
	fp.files[filename] = contents
	fp.Unlock()

	return &recordingReadCloser{
		ReadCloser: f,
		contents:   contents,
	}, nil
}

func (fp *recordingFileProvider) Dump(filename string) error {
	fp.Lock()
	defer fp.Unlock()

	filenames := []string{}
	for k := range fp.files {
		filenames = append(filenames, k)
	}
	sort.Strings(filenames)

	out, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0664)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.WriteString("var data = map[string]string{\n")
	if err != nil {
		return err
	}

	for _, f := range filenames {
		fp.files[f].Lock()
		data := fp.files[f].buf

		out.WriteString("`" + f + "`: ")

		s := string(data)

		if canMultilineBackquote(s) {
			out.WriteString("`" + s + "`")
		} else {
			compressedData := new(bytes.Buffer)
			writer := gzip.NewWriter(compressedData)
			writer.Write(data)
			writer.Close()
			data = compressedData.Bytes()
			out.WriteString(strconv.QuoteToASCII(string(data)))
		}

		out.WriteString(",\n")

		fp.files[f].Unlock()
		if err != nil {
			return err
		}
	}
	_, err = out.WriteString("}\n")

	return err
}

type recordingReadCloser struct {
	io.ReadCloser
	contents *recordedFileContents
}

func (r *recordingReadCloser) Read(buf []byte) (int, error) {
	n, err := r.ReadCloser.Read(buf)
	if n > 0 {
		r.contents.Lock()
		r.contents.buf = append(r.contents.buf, buf...)
		r.contents.Unlock()
	}
	return n, err
}

type recordedFileContents struct {
	sync.Mutex
	buf []byte
}

// testFileProvider
type testFileProvider struct {
	files map[string]string
}

func NewTestFileProvider(files map[string]string) FileProvider {
	return &testFileProvider{
		files: files,
	}
}

func (fp *testFileProvider) ReadFtraceFile(filename string) ([]byte, error) {
	if !SafeFtracePath(filename) {
		return nil, BadFtraceFileName
	}

	return []byte(fp.files[path.Join(ftracePath, filename)]), nil
}

func (fp *testFileProvider) ReadProcFile(filename string) ([]byte, error) {
	if !SafeProcPath(filename) {
		return nil, BadProcFileName
	}

	return []byte(fp.files[path.Join(procPath, filename)]), nil
}

func (fp *testFileProvider) WriteFtraceFile(filename string, data []byte) error {
	if !SafeFtracePath(filename) {
		return BadFtraceFileName
	}

	return nil
}

func (fp *testFileProvider) OpenFtrace(filename string) (io.ReadCloser, error) {
	if !SafeFtracePath(filename) {
		return nil, BadFtraceFileName
	}

	data := []byte(fp.files[filename])
	if len(data) > 4 && data[0] == 0x1f && data[1] == 0x8b && data[2] == 0x08 && data[3] == 0x00 {
		return gzip.NewReader(bytes.NewBuffer(data))
	}

	return &testReader{
		Reader: bytes.NewReader(data),
	}, nil
}

type testReader struct {
	*bytes.Reader
}

func (t *testReader) Close() error {
	return nil
}

// Utility functions
func SafeFtracePath(path string) bool {
	components := filepath.SplitList(path)
	for _, d := range components {
		if d == ".." {
			return false
		}
	}
	return true
}

func SafeProcPath(path string) bool {
	return procFileWhitelist[path]
}

var procFileWhitelist = map[string]bool{
	"kallsyms": true,
}

func canMultilineBackquote(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < ' ' && c != '\t' && c != '\n') || c == '`' || c == '\u007F' {
			return false
		}
	}
	return true
}
