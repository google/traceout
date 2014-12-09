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
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"traceout/ftrace/cparse"
	"traceout/ftrace/cprintf"
)

type EventType struct {
	path         string
	name         string
	id           int
	fields       []eventField
	size         int
	formatter    cparse.Expression
	pidField     int
	flagsField   int
	preemptField int
	fileProvider FileProvider
}

type eventField struct {
	name    string
	size    int
	offset  int
	signed  bool
	array   bool
	ftype   string
	dataloc bool
}

type eventFieldValue struct {
	field    *eventField
	contents []byte
}

var BadEvent error = errors.New("Bad event name")
var BadEventData error = errors.New("Bad event data")

func NewHeaderType(fp FileProvider, path string) (*EventType, error) {
	etype := EventType{
		fileProvider: fp,
	}

	formatbytes, err := etype.fileProvider.ReadFtraceFile(path)
	if err != nil {
		return nil, err
	}

	err = etype.parseFormatData(formatbytes)

	etype.finishNewType()

	return &etype, nil
}

func newEventType(fp FileProvider, path string) (*EventType, error) {
	if !SafeFtracePath(path) {
		return nil, BadEvent
	}

	etype := EventType{
		fileProvider: fp,
		path:         path,
		name:         filepath.Base(path),
	}
	err := etype.parseFormatFile()
	if err != nil {
		return nil, err
	}

	etype.pidField = etype.getFieldNum("common_pid")
	etype.flagsField = etype.getFieldNum("common_flags")
	etype.preemptField = etype.getFieldNum("common_preempt_count")

	etype.finishNewType()

	return &etype, nil
}

func (etype *EventType) Name() string {
	return etype.name
}

func (etype *EventType) finishNewType() {
	for _, f := range etype.fields {
		if etype.size < f.offset+f.size {
			etype.size = f.offset + f.size
		}
	}

}

func (etype *EventType) DecodeEvent(data []byte, cpu int, when uint64) (*Event, error) {
	var e Event
	e.Cpu = cpu
	e.When = when
	if len(data) < etype.size {
		return nil, BadEventData
	}
	e.values = make([]eventFieldValue, len(etype.fields))
	for i, f := range etype.fields {
		e.values[i].field = &etype.fields[i]
		e.values[i].contents = data[f.offset : f.offset+f.size]
	}
	e.etype = etype
	e.contents = data

	e.Pid = int(e.values[e.etype.pidField].DecodeInt())
	e.Flags = uint(e.values[e.etype.flagsField].DecodeUint())
	e.Preempt = int(e.values[e.etype.preemptField].DecodeInt())

	return &e, nil
}

func (etype *EventType) Enable() error {
	return etype.writeEventFile("enable", []byte("1"))
}

func (etype *EventType) Disable() error {
	return etype.writeEventFile("enable", []byte("0"))
}

func (etype *EventType) readEventFile(filename string) ([]byte, error) {
	return etype.fileProvider.ReadFtraceFile(path.Join("events", etype.path, filename))
}

func (etype *EventType) writeEventFile(filename string, data []byte) error {
	return etype.fileProvider.WriteFtraceFile(path.Join("events", etype.path, filename), data)
}

func (etype *EventType) parseFormatData(formatbytes []byte) (err error) {
	lineNum := 0
	format := string(formatbytes)

	for format != "" {
		lineNum++

		eol := strings.IndexRune(format, '\n')
		if eol == -1 {
			eol = len(format)
		}

		line := format[:eol]
		format = format[eol+1:]

		if line == "" {
			continue
		}

		colon := strings.IndexRune(line, ':')
		if colon == -1 {
			err = fmt.Errorf("missing ':' on line %d", lineNum)
			return
		}

		key := strings.TrimSpace(line[:colon])
		value := strings.TrimSpace(line[colon+1:])

		switch key {
		case "name", "format":
			// ignored
			continue
		case "ID":
			etype.id, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "field":
			err = etype.parseField(value)
			if err != nil {
				return
			}
		case "print fmt":
			err = etype.parsePrintFmt(value)
			if err != nil {
				return
			}
		default:
			err = fmt.Errorf("unexpected key %s", key)
		}
	}

	return nil
}

// Reads the format file from sysfs and parses the necessary information out of it (id and fields for now)
func (etype *EventType) parseFormatFile() (err error) {
	formatbytes, err := etype.readEventFile("format")
	if err != nil {
		return
	}

	err = etype.parseFormatData(formatbytes)

	return
}

// Takes a line containing everything following "field:" and adds it to the field list of the event
func (etype *EventType) parseField(line string) (err error) {
	var field eventField

	s := strings.Split(line, ";")
	if s == nil || len(s) == 0 {
		err = errors.New("Missing entries in field")
		return
	}

	// Parse field type and name
	last_space := strings.LastIndex(s[0], " ")
	if last_space == -1 {
		err = errors.New("missing field type and name")
		return
	}
	field.ftype = s[0][0:last_space]
	field.name = s[0][last_space+1:]

	bracket := strings.IndexRune(field.name, '[')
	if bracket != -1 {
		endBracket := strings.IndexRune(field.name, ']')
		if endBracket == -1 || endBracket < bracket {
			err = errors.New("expected ']' after '['")
			return
		}
		field.array = true
		field.name = field.name[:bracket]
	}

	if strings.HasPrefix(field.ftype, "__data_loc char[]") {
		field.ftype = strings.TrimPrefix(field.ftype, "__data_loc char[]")
		field.dataloc = true
	}

	for _, f := range s[1:] {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		colon := strings.IndexRune(f, ':')
		if colon == -1 {
			err = fmt.Errorf("missing ':' in field entry %s", f)
			return
		}

		key := strings.TrimSpace(f[0:colon])
		value := strings.TrimSpace(f[colon+1:])
		switch key {
		case "offset":
			field.offset, err = strconv.Atoi(value)
		case "size":
			field.size, err = strconv.Atoi(value)
		case "signed":
			field.signed, err = strconv.ParseBool(value)
		default:
			err = fmt.Errorf("unknown field entry %s", key)
		}
		if err != nil {
			return
		}
	}

	if field.offset < 0 {
		err = errors.New("a negative field offset is not valid")
		return
	}

	if field.size < 0 {
		err = errors.New("a negative field size is not valid")
		return
	}

	etype.fields = append(etype.fields, field)

	return
}

func (etype *EventType) parsePrintFmt(format string) (err error) {
	args, err := cparse.Parse(format, etype)
	if err != nil {
		return err
	}
	etype.formatter, err = cprintf.NewPrintfFunction(args, mungePrintfConversions)
	if err != nil {
		return err
	}
	return
}

func mungePrintfConversions(c cprintf.Conversion) cprintf.Conversion {
	if c.Conversion == 'p' && len(c.Suffix) > 0 &&
		(c.Suffix[0] == 'f' || c.Suffix[0] == 'F' || c.Suffix[0] == 'K') {

		pointerModifier := c.Suffix[0]
		c.Suffix = c.Suffix[1:]
		name := ""
		switch pointerModifier {
		case 'f':
			name = "__printk_pf"
		case 'F':
			name = "__printk_pF"
		case 'K':
			name = "__printk_pK"
		default:
			panic("unexpected pointer modifier " + string(pointerModifier))
		}
		function := kernelFunctions[name]
		if function == nil {
			c.Suffix = "FAILED POINTER MODIFIER " + string(pointerModifier) + " " + c.Suffix
			return c
		}

		c.Arg = cparse.CallFunction(eventFunction{function}, name, []cparse.Expression{c.Arg})
		c.Conversion = 's'
		c.Modifiers = ""
	}
	return c
}

func (etype *EventType) getFieldNum(field string) int {
	for i, f := range etype.fields {
		if f.name == field {
			return i
		}
	}
	return -1
}

func (etype *EventType) Format(e Event) string {
	if etype.formatter == nil {
		return "event type " + etype.path + " has no formatter"
	}
	v := etype.formatter.Value(e)
	if !v.IsString() {
		return "formatter expected string, got " + v.Dump()
	}
	return v.AsString()
}

func (v eventFieldValue) DecodeUint() uint64 {
	switch v.field.size {
	case 1:
		return uint64(v.contents[0])
	case 2:
		return uint64(order.Uint16(v.contents))
	case 4:
		return uint64(order.Uint32(v.contents))
	case 8:
		return order.Uint64(v.contents)
	default:
		return 0
	}
}

func (v eventFieldValue) DecodeInt() int64 {
	switch v.field.size {
	case 1:
		return int64(int8(v.contents[0]))
	case 2:
		return int64(int16(order.Uint16(v.contents)))
	case 4:
		return int64(int32(order.Uint32(v.contents)))
	case 8:
		return int64(order.Uint64(v.contents))
	default:
		return 0
	}
}

func (v eventFieldValue) String() string {
	if v.field.signed {
		return fmt.Sprintf("%s: %d", v.field.name, v.DecodeInt())
	} else {
		return fmt.Sprintf("%s: %d", v.field.name, v.DecodeUint())
	}
}

type eventVariable struct {
	fieldNum int
}

func (ev eventVariable) Get(ctx cparse.EvalContext) cparse.Value {
	e := ctx.(Event)
	switch e.etype.fields[ev.fieldNum].ftype {
	case "char":
		s := string(e.values[ev.fieldNum].contents)
		zero := strings.IndexByte(s, 0)
		if zero != -1 {
			s = s[:zero]
		}
		return cparse.NewValueString(s)
	default:
		var i uint64
		if e.etype.fields[ev.fieldNum].signed {
			i = uint64(e.values[ev.fieldNum].DecodeInt())
		} else {
			i = e.values[ev.fieldNum].DecodeUint()
		}
		return cparse.NewValueInt(i, e.etype.fields[ev.fieldNum].size, e.etype.fields[ev.fieldNum].signed)
	}
}

func (etype EventType) GetVariable(name string) cparse.Variable {
	recName := strings.TrimPrefix(name, "REC->")
	f := etype.getFieldNum(recName)
	if f >= 0 {
		return eventVariable{f}
	}

	if v, ok := kernelConstants[name]; ok {
		return cparse.NewConstantVariable(cparse.NewValueInt(uint64(int64(v)), 4, true))
	}

	return nil
}

type eventFunction struct {
	f kernelFunc
}

func (ef eventFunction) Get(ctx cparse.EvalContext, args []cparse.Value) cparse.Value {
	return ef.f(ctx, args)
}

func (etype EventType) GetFunction(name string) cparse.Function {
	if f, ok := kernelFunctions[name]; ok {
		return eventFunction{f}
	}
	return nil
}

func (etype EventType) GetType(name string) string {
	return kernelTypes[name]
}
