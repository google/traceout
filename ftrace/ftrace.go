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
	"reflect"
	"strconv"
	"strings"
)

type ftrace struct {
	fp                   FileProvider
	eventTypes           map[int]*EventType
	selectCases          []reflect.SelectCase
	cachedProcessNames   map[int]string
	isCachedProcessNames bool
	cachedKallsyms       map[uint64]string

	pageHeader               *EventType
	pageHeaderFieldTimestamp int
	pageHeaderFieldCommit    int
	pageHeaderFieldData      int
}

func Ftrace(fp FileProvider) (*ftrace, error) {
	f := &ftrace{
		fp:         fp,
		eventTypes: make(map[int]*EventType),
	}

	err := f.init()
	if err != nil {
		return nil, err
	}

	return f, nil
}

func (f *ftrace) init() error {
	var err error

	f.pageHeader, err = NewHeaderType(f.fp, "events/header_page")
	if err != nil {
		return err
	}
	f.pageHeaderFieldTimestamp = f.pageHeader.getFieldNum("timestamp")
	f.pageHeaderFieldCommit = f.pageHeader.getFieldNum("commit")
	f.pageHeaderFieldData = f.pageHeader.getFieldNum("data")

	f.cachedProcessNames = make(map[int]string)

	return nil
}

func (f *ftrace) NewEventType(path string) (*EventType, error) {
	etype, err := newEventType(f.fp, path)
	if err != nil {
		return nil, err
	}

	if f.eventTypes[etype.id] != nil {
		err := fmt.Errorf("event id %d already exists", etype.id)
		return nil, err
	}

	f.eventTypes[etype.id] = etype
	return etype, nil
}

func (f *ftrace) Enable() error {
	return f.fp.WriteFtraceFile("tracing_on", []byte("1"))
}

func (f *ftrace) Disable() error {
	return f.fp.WriteFtraceFile("tracing_on", []byte("0"))
}

func (f *ftrace) Clear() error {
	return f.fp.WriteFtraceFile("trace", []byte(""))
}

func (f *ftrace) ReadKernelTrace() ([]byte, error) {
	return f.fp.ReadFtraceFile("trace")
}

func (f *ftrace) PrepareCapture(cpus int, doneCh <-chan bool) error {
	f.selectCases = []reflect.SelectCase{
		reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(doneCh),
		},
	}

	for cpu := 0; cpu < cpus; cpu++ {
		ch, err := f.getEvents(cpu, doneCh)
		if err != nil {
			return err
		}
		f.selectCases = append(f.selectCases,
			reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(ch),
			})
	}

	return nil
}

func (f *ftrace) Capture(callback func(Events)) {
	eventArrayType := reflect.TypeOf(Events{})

	for len(f.selectCases) > 1 {
		chosen, recv, recvOK := reflect.Select(f.selectCases)
		if chosen == 0 {
			break
		}
		if !recvOK {
			f.selectCases = append(f.selectCases[:chosen], f.selectCases[chosen+1:]...)
			continue
		}
		if recv.Type() == eventArrayType {
			events := recv.Interface().(Events)
			callback(events)
		}
	}
}

func (f *ftrace) processName(pid int) string {
	if !f.isCachedProcessNames {
		f.isCachedProcessNames = true
		processNameFile, err := f.fp.ReadFtraceFile("saved_cmdlines")
		if err != nil {
			return ""
		}
		processNames := strings.Split(string(processNameFile), "\n")
		for _, n := range processNames {
			v := strings.SplitN(n, " ", 2)
			if len(v) != 2 {
				continue
			}
			p, err := strconv.Atoi(v[0])
			if err != nil {
				continue
			}
			f.cachedProcessNames[p] = v[1]
		}
	}

	return f.cachedProcessNames[pid]
}

func (f *ftrace) kernelSymbol(addr uint64) string {
	if f.cachedKallsyms == nil {
		f.cachedKallsyms = make(map[uint64]string)
		// TODO: through fp
		kallsymsFile, err := f.fp.ReadProcFile("kallsyms")
		if err != nil {
			return ""
		}
		kallsyms := strings.Split(string(kallsymsFile), "\n")
		for _, k := range kallsyms {
			v := strings.SplitN(k, " ", 3)
			if len(v) != 3 {
				continue
			}
			a, err := strconv.ParseUint(v[0], 16, 64)
			if err != nil {
				continue
			}
			f.cachedKallsyms[a] = strings.Replace(v[2], "\t", " ", -1)
		}
	}
	return f.cachedKallsyms[addr]
}
