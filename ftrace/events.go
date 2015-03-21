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
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const (
	entryTypeDataMax    = 28
	entryTypePadding    = 29
	entryTypeTimeExt    = 30
	entryTypeLenBits    = 5
	entryTimeDeltaBits  = 27
	entryTypeLenShift   = 0
	entryTimeDeltaShift = entryTypeLenBits
	entryTypeLenMask    = uint32((1 << entryTypeLenBits) - 1)
	entryTimeDeltaMask  = uint32((1 << entryTimeDeltaBits) - 1)
)

type BadEventHeader struct {
	What   string
	Page   []byte
	Offset int
}

func (e BadEventHeader) Error() string {
	return fmt.Sprintf("Bad event header at %x: %s\n", e.Offset, e.What) + hex.Dump(e.Page)
}

var BadPageHeader = errors.New("Bad page header")

var order = binary.LittleEndian

// Returns a channel that provides individual events from a cpu raw ftrace pipe
// Requires all enabled events to be registered or it will fail to parse
// TODO: automatically attempt to resync?  Try every byte as a header_page, look for valid type IDs?
// Or just drop the page, mark lost events, and continue with the next page?
// Write to doneCh to end
func (f *Ftrace) getEvents(cpu int, doneCh <-chan bool) (<-chan Events, error) {
	rawDoneCh := make(chan bool)
	eventCh := make(chan Events)

	rawCh, err := getRawFtraceChan(f.fp, cpu, rawDoneCh)
	if err != nil {
		return nil, err
	}

	go func() {
		defer close(rawDoneCh)
		defer close(eventCh)

		for {
			select {
			case <-doneCh:
				return
			case buf, ok := <-rawCh:
				if !ok {
					// raw channel failed
					return
				}
				events, err := f.decodePage(cpu, buf)
				if err != nil {
					fmt.Println(err.Error())
					// TODO: error over channel?
				}
				eventCh <- events
			}
		}
	}()

	return eventCh, nil
}

func (f *Ftrace) decodePage(cpu int, data []byte) (events Events, err error) {
	page, err := f.pageHeader.DecodeEvent(data, 0, 0)
	if err != nil {
		return nil, err
	}
	when := page.values[f.pageHeaderFieldTimestamp].DecodeUint()
	pageLen := int(page.values[f.pageHeaderFieldCommit].DecodeUint() & ((1 << 30) - 1))
	pageOffset := page.values[f.pageHeaderFieldData].field.offset

	if pageLen < 0 || len(data) < pageOffset+pageLen {
		return nil, BadPageHeader
	}

	fullData := data[0 : pageOffset+pageLen]
	data = data[pageOffset : pageOffset+pageLen]

	events = make(Events, 0, 64)

	var lazyErr error
dataLoop:
	for len(data) > 0 {
		if len(data) < 4 {
			err = BadPageHeader
			return
		}

		offset := len(fullData[:cap(fullData)]) - len(data[:cap(data)])

		entryHeader := order.Uint32(data)
		data = data[4:]

		typeLen := (entryHeader >> entryTypeLenShift) & entryTypeLenMask
		timeDelta := uint64((entryHeader >> entryTimeDeltaShift) & entryTimeDeltaMask)

		switch {
		case typeLen <= entryTypeDataMax:
			when += timeDelta

			var dataLen int
			if typeLen == 0 {
				// TODO: find test event for this
				if len(data) < 4 {
					err = BadEventHeader{"Not enough data for type len == 0", fullData, offset}
					return
				}

				dataLen = int(order.Uint32(data))
				data = data[4:]
			} else {
				dataLen = int(typeLen) * 4
			}

			if len(data) < dataLen || dataLen < 2 {
				err = BadEventHeader{fmt.Sprintf("Not enough data (%d, 0x%x) for len (%d, 0x%x) pageLen %x pageOffset+pageLen %x", len(data), len(data), dataLen, dataLen, pageLen, pageOffset+pageLen), fullData, offset}
				return
			}

			eventData := data[:dataLen]
			data = data[(dataLen+3)&^0x3:]

			typeId := int(order.Uint16(eventData))

			etype := f.eventTypes[typeId]
			if etype == nil {
				lazyErr = fmt.Errorf("unknown type ID: %d (0x%x)", typeId, typeId)
				continue
			}

			var event *Event
			event, err = etype.DecodeEvent(eventData, cpu, when)
			if err != nil {
				lazyErr = err
				continue
			}
			event.ftrace = f
			events = append(events, event)

		case typeLen == entryTypePadding:
			if timeDelta == 0 {
				break dataLoop
			} else {
				if len(data) < 4 {
					err = BadEventHeader{"Not enough data for type padding", fullData, offset}
					return
				}

				padding := order.Uint32(data)
				data = data[padding:]
			}

		case typeLen == entryTypeTimeExt:
			if len(data) < 4 {
				err = BadEventHeader{"Not enough data for type time ext", fullData, offset}
				return
			}

			timeDeltaExt := order.Uint32(data)
			data = data[4:]

			timeDelta += uint64(timeDeltaExt) << entryTimeDeltaBits
			when += timeDelta
		}
	}

	err = lazyErr
	return
}

type Event struct {
	ftrace   *Ftrace
	etype    *EventType
	values   []eventFieldValue
	Cpu      int
	When     uint64
	Pid      int
	Flags    uint
	Preempt  int
	contents []byte
}

func (e Event) String() string {
	return fmt.Sprintf("%16s-%-5d [%03d] %s %6d.%06d: %s: %s",
		e.ProcessName(), e.Pid, e.Cpu, e.FlagChars(), e.Seconds(), e.Microseconds(),
		e.etype.name, e.etype.Format(e))
}

func (e Event) Seconds() int {
	return int(e.whenInMicroseconds() / 1e6)
}

func (e Event) Microseconds() int {
	return int(e.whenInMicroseconds() % 1e6)
}

func (e Event) whenInMicroseconds() int64 {
	t := (time.Duration(e.When) * time.Nanosecond)
	return int64((t + time.Microsecond/2) / time.Microsecond)
}

func (e Event) FlagChars() string {
	const (
		flagIrqsOff        = 0x1
		flagIrqsNoSupport  = 0x2
		flagNeedResched    = 0x4
		flagHardIrq        = 0x8
		flagSoftIrq        = 0x10
		flagPreemptResched = 0x20

		flagHardSoftIrq        = flagHardIrq | flagSoftIrq
		flagPreemptNeedResched = flagNeedResched | flagPreemptResched
	)
	var f []byte = []byte("....")

	if e.Flags&flagIrqsOff == flagIrqsOff {
		f[0] = 'd'
	} else if e.Flags&flagIrqsNoSupport == flagIrqsNoSupport {
		f[0] = 'X'
	}

	if e.Flags&flagPreemptNeedResched == flagPreemptNeedResched {
		f[1] = 'N'
	} else if e.Flags&flagNeedResched == flagNeedResched {
		f[1] = 'n'
	} else if e.Flags&flagPreemptResched == flagPreemptResched {
		f[1] = 'p'
	}

	if e.Flags&flagHardSoftIrq == flagHardSoftIrq {
		f[2] = 'H'
	} else if e.Flags&flagHardIrq == flagHardIrq {
		f[2] = 'h'
	} else if e.Flags&flagSoftIrq == flagSoftIrq {
		f[2] = 's'
	}

	if e.Preempt > 0 {
		f[3] = '0' + byte(e.Preempt)
	}

	return string(f)
}

func (e Event) ProcessName() string {
	if e.Pid == 0 {
		return "<idle>"
	} else if n := e.ftrace.processName(e.Pid); n != "" {
		return n
	} else {
		return "<...>"
	}
}

type EventsByTime struct{ Events }

func (e EventsByTime) Less(i, j int) bool {
	if e.Events[i].When == e.Events[j].When {
		return e.Events[i].Cpu < e.Events[j].Cpu
	}
	return e.Events[i].When < e.Events[j].When
}

type Events []*Event

func (e Events) Len() int      { return len(e) }
func (e Events) Swap(i, j int) { e[i], e[j] = e[j], e[i] }

func (e Events) HasEventType(etype *EventType) bool {
	for _, event := range e {
		if event.etype == etype {
			return true
		}
	}
	return false
}
