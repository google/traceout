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

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/pprof"
	"sort"
	"strings"
	"time"

	"github.com/google/traceout/ftrace"
)

import _ "net/http/pprof"
import "net/http"

var (
	cpuProfile  string
	memProfile  string
	debugServer bool
	recordReads string
	timeout     time.Duration
	test        bool
)

func init() {
	flag.StringVar(&cpuProfile, "cpuprofile", "", "write cpu profile to file")
	flag.StringVar(&memProfile, "memprofile", "", "write memory profile to file")
	flag.BoolVar(&debugServer, "debugserver", false, "enable debug server on localhost:6060")
	flag.StringVar(&recordReads, "record", "", "record files read from kernel for replay testing")
	flag.DurationVar(&timeout, "t", 0, "end trace after timeout")
	flag.BoolVar(&test, "test", false, "compare kernel formatted trace to btrace output")
}

func do_main() error {
	flag.Parse()

	if debugServer {
		go func() {
			fmt.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	if cpuProfile != "" {
		f, err := os.Create(cpuProfile)
		if err != nil {
			return err
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
		defer f.Close()
	}

	if memProfile != "" {
		f, err := os.Create(memProfile)
		if err != nil {
			return err
		}
		defer pprof.WriteHeapProfile(f)
		defer f.Close()
	}

	fp := ftrace.NewLocalFileProvider()
	if recordReads != "" {
		rfp := ftrace.NewRecordingFileProvider(fp)
		fp = rfp
		f := func() {
			err := rfp.Dump(recordReads)
			if err != nil {
				fmt.Println(err)
			}
		}
		defer f()
	}

	doneCh := make(chan bool)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	f, err := ftrace.New(fp)
	if err != nil {
		return err
	}

	f.Disable()
	f.Clear()

	eventNames := []string{
		"sched/sched_switch",
		"sched/sched_wakeup",
		"task/task_newtask",
		"irq/irq_handler_entry",
		"irq/irq_handler_exit",
		"irq/softirq_entry",
		"irq/softirq_exit",
		"irq/softirq_raise",
		"workqueue/workqueue_activate_work",
		"workqueue/workqueue_execute_start",
		"workqueue/workqueue_execute_end",
		"workqueue/workqueue_queue_work",
		"power/cpu_frequency",
		"power/cpu_idle",
		"vmscan/mm_vmscan_direct_reclaim_begin",
		"vmscan/mm_vmscan_direct_reclaim_end",
		"vmscan/mm_vmscan_kswapd_sleep",
		"vmscan/mm_vmscan_kswapd_wake",
		"ext4/ext4_da_write_begin",
		"ext4/ext4_da_write_end",
		"ext4/ext4_sync_file_enter",
		"ext4/ext4_sync_file_exit",
		"block/block_rq_issue",
		"block/block_rq_complete",
		//"tlb/tlb_flush",
		"signal/signal_generate",
		"signal/signal_deliver",
	}

	eventTypes := []*ftrace.EventType{}

	for _, e := range eventNames {
		eType, err := f.NewEventType(e)
		if err != nil {
			return err
		}
		eventTypes = append(eventTypes, eType)
	}

	for _, e := range eventTypes {
		e.Enable()
	}

	go func() {
		<-sigCh
		close(doneCh)
	}()

	timeoutCh := make(<-chan time.Time)

	if timeout > 0 {
		timeoutCh = time.After(timeout)
		go func() {
			<-timeoutCh
			close(doneCh)
		}()
	}

	var kernelTrace []byte

	if test {
		f.Enable()
		<-doneCh
		f.Disable()
		kernelTrace, err = f.ReadKernelTrace()
		doneCh = make(chan bool)
		timeoutCh = time.After(time.Second)
		go func() {
			<-timeoutCh
			close(doneCh)
		}()
	}

	f.PrepareCapture(32, doneCh)

	if !test {
		f.Enable()
		f.Capture(func(e ftrace.Events) {
			for _, e := range e {
				fmt.Println(e.String())
			}
		})
		f.Disable()
	} else {
		var events ftrace.Events

		f.Capture(func(e ftrace.Events) {
			events = append(events, e...)
		})

		sort.Stable(ftrace.EventsByTime{events})

		eventStrings := []string{}
		for _, e := range events {
			eventStrings = append(eventStrings, e.String())
		}

		kernelStrings := strings.Split(string(kernelTrace), "\n")
		for kernelStrings[0][0] == '#' {
			kernelStrings = kernelStrings[1:]
		}

		i := 0
		for i = 0; i < len(eventStrings); i++ {
			if i >= len(kernelStrings) {
				err = fmt.Errorf("kernelStrings shorter than eventStrings (%d < %d)",
					len(kernelStrings), len(eventStrings))
				break
			}
			if eventStrings[i] != kernelStrings[i] {
				err = fmt.Errorf("mismatch line %d, expected\n   %s\ngot\n   %s",
					i+1, kernelStrings[i], eventStrings[i])
				break
			}
		}

		if i == len(eventStrings) {
			fmt.Printf("%d lines match\n", len(eventStrings))
			for _, t := range eventTypes {
				if !events.HasEventType(t) {
					fmt.Printf("no events of type %s\n", t.Name())
				}
			}
		}
	}

	for _, e := range eventTypes {
		e.Disable()
	}

	return err
}

func main() {
	err := do_main()
	if err != nil {
		fmt.Println(err.Error())
	}
}
