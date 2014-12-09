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
	"strings"

	"traceout/ftrace/cparse"
)

type kernelFunc func(cparse.EvalContext, []cparse.Value) cparse.Value

var kernelFunctions = map[string]kernelFunc{
	"__print_flags":    printFlags,
	"__print_symbolic": printSymbolic,
	"__get_str":        getString,
	"__printk_pf":      printkFunctionPointer,
	"__printk_pF":      printkFunctionPointerOffset,
	"__printk_pk":      printkKernelSymbol,
	/* TODO:
	   __print_symbolic
	   __print_hex
	   __get_dynamic_array?
	*/
}

var kernelConstants = map[string]int{
	/* soft irq types */
	"HI_SOFTIRQ":           0,
	"TIMER_SOFTIRQ":        1,
	"NET_TX_SOFTIRQ":       2,
	"NET_RX_SOFTIRQ":       3,
	"BLOCK_SOFTIRQ":        4,
	"BLOCK_IOPOLL_SOFTIRQ": 5,
	"TASKLET_SOFTIRQ":      6,
	"SCHED_SOFTIRQ":        7,
	"HRTIMER_SOFTIRQ":      8,
	"RCU_SOFTIRQ":          9,

	/* tlb flush types */
	"TLB_FLUSH_ON_TASK_SWITCH": 0,
	"TLB_REMOTE_SHOOTDOWN":     1,
	"TLB_LOCAL_SHOOTDOWN":      2,
	"TLB_LOCAL_MM_SHOOTDOWN":   3,
}

var kernelTypes = map[string]string{
	"gfp_t": "unsigned int",
}

func printFlags(ctx cparse.EvalContext, args []cparse.Value) cparse.Value {
	if len(args) < 3 {
		return cparse.NewValueError("expected at least 3 arguments to __print_flags")
	}

	if !args[0].IsInt() {
		return cparse.NewValueError("expected integer as first argument to __print_flags")
	}
	v := args[0].AsInt()

	if !args[1].IsString() {
		return cparse.NewValueError("expected string as second argument to __print_flags")
	}
	delim := args[1].AsString()

	first := true
	ret := ""

	for _, f := range args[2:] {
		if !f.IsList() {
			return cparse.NewValueError("expected list as argument to __print_flags")
		}
		l := f.AsList()

		if len(l) != 2 {
			return cparse.NewValueError("expected list of two elements as argument to __print_flags")
		}
		if !l[0].IsInt() {
			return cparse.NewValueError("expected first element of list to be int as argument to __print_flags")
		}
		if !l[1].IsString() {
			return cparse.NewValueError("expected second element of list to be string as argument to __print_flags")
		}
		if (v & l[0].AsInt()) != 0 {
			if first {
				ret = l[1].AsString()
				first = false
			} else {
				ret += delim + l[1].AsString()
			}
		}
	}
	return cparse.NewValueString(ret)
}

func printSymbolic(ctx cparse.EvalContext, args []cparse.Value) cparse.Value {
	if len(args) < 2 {
		return cparse.NewValueError("expected at least 2 arguments to __print_symbolic")
	}

	if !args[0].IsInt() {
		return cparse.NewValueError("expected integer as first argument to __print_symbolic")
	}
	v := args[0].AsInt()

	for _, f := range args[1:] {
		if !f.IsList() {
			return cparse.NewValueError("expected list as argument to __print_symbolic ")
		}
		l := f.AsList()

		if len(l) != 2 {
			return cparse.NewValueError("expected list of two elements as argument to __print_symbolic")
		}
		if !l[0].IsInt() {
			return cparse.NewValueError("expected first element of list to be int as argument to __print_symbolic, got " + l[0].Dump())
		}
		if !l[1].IsString() {
			return cparse.NewValueError("expected second element of list to be string as argument to __print_symbolic")
		}
		if v == l[0].AsInt() {
			return cparse.NewValueString(l[1].AsString())
		}
	}
	return cparse.NewValueString("")
}

func getString(ctx cparse.EvalContext, args []cparse.Value) cparse.Value {
	e := ctx.(Event)

	if len(args) != 1 {
		return cparse.NewValueError("expected 1 argument to __get_str")
	}

	if !args[0].IsInt() {
		return cparse.NewValueError("expected integer as first argument to __get_str")
	}
	i := int(args[0].AsInt())

	offset := i & 0xffff
	length := i >> 16

	if offset > len(e.contents)-1 {
		return cparse.NewValueError("__get_str offset %d too large", offset)
	}

	if offset+length > len(e.contents) {
		return cparse.NewValueError("__get_str length %d too large", length)
	}

	s := string(e.contents[offset : offset+length])
	zero := strings.IndexByte(s, 0)
	if zero != -1 {
		s = s[:zero]
	}
	return cparse.NewValueString(s)
}

func printkFunctionPointer(ctx cparse.EvalContext, args []cparse.Value) cparse.Value {
	e := ctx.(Event)

	if len(args) != 1 {
		return cparse.NewValueError("expected 1 argument to __printk_pf")
	}

	if !args[0].IsInt() {
		return cparse.NewValueError("expected integer as first argument to __printk_pf")
	}
	addr := uint64(args[0].AsInt())

	return cparse.NewValueString(e.ftrace.kernelSymbol(addr))
}

func printkFunctionPointerOffset(ctx cparse.EvalContext, args []cparse.Value) cparse.Value {
	e := ctx.(Event)

	if len(args) != 1 {
		return cparse.NewValueError("expected 1 argument to __printk_pf")
	}

	if !args[0].IsInt() {
		return cparse.NewValueError("expected integer as first argument to __printk_pf")
	}
	addr := uint64(args[0].AsInt())

	// TODO: find function before addr, print offset
	return cparse.NewValueString(e.ftrace.kernelSymbol(addr))
}

func printkKernelSymbol(ctx cparse.EvalContext, args []cparse.Value) cparse.Value {
	e := ctx.(Event)

	if len(args) != 1 {
		return cparse.NewValueError("expected 1 argument to __printk_pf")
	}

	if !args[0].IsInt() {
		return cparse.NewValueError("expected integer as first argument to __printk_pf")
	}
	addr := uint64(args[0].AsInt())

	return cparse.NewValueString(e.ftrace.kernelSymbol(addr))
}
