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

package cprintf

// This package implements a cparse.Function that acts like the C version of printf
// by munging conversion specifiers and adding casts in order to call go's fmt.Sprintf.
// Callers can implement extensions to the printf conversion specifiers by passing a
// conversionCallback function.

import (
	"fmt"
	"strings"

	"github.com/google/traceout/ftrace/cparse"
)

type printfFunction struct {
	format string
}

type Conversion struct {
	Conversion byte
	Modifiers  string
	Suffix     string
	Arg        cparse.Expression
	Scope      cparse.Scope
}

type conversionCallback func(c Conversion) Conversion

func NewPrintfFunction(args []cparse.Expression, callback conversionCallback) (cparse.Expression, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("expected at least one argument to printf")
	}
	if !args[0].IsConstant() {
		return nil, fmt.Errorf("expected constant as first argument to printf, got " +
			args[0].Dump())
	}
	v := args[0].Value(nil)
	if !v.IsString() {
		return nil, fmt.Errorf("expected string as first argument to printf, got " + args[0].Dump())
	}
	format := v.AsString()
	args = args[1:]

	format, args = mungeConversions(format, args, callback)

	function := &printfFunction{
		format: format,
	}
	return cparse.CallFunction(function, "printf", args), nil
}

func (pf *printfFunction) Get(ctx cparse.EvalContext, args []cparse.Value) cparse.Value {
	sprintfArgs := make([]interface{}, len(args))
	for i, v := range args {
		if v.IsError() {
			return v
		}
		sprintfArgs[i] = v.AsInterface()
	}

	return cparse.NewValueString(fmt.Sprintf(pf.format, sprintfArgs...))
}

const (
	conversionSpecifiers      = "cdiopsux%"
	formatModifiers           = "0123456789-#.*"
	trimmedConversionModfiers = "hlLz"
	validModifiers            = formatModifiers + trimmedConversionModfiers
)

func mungeConversions(format string, args []cparse.Expression,
	callback conversionCallback) (string, []cparse.Expression) {

	out := ""
	arg := 0

	for format != "" {
		i := strings.IndexRune(format, '%')
		if i == -1 {
			// No more % fields, copy the rest
			out += format
			break
		}
		// Copy everything up to and including the first % into the output string
		out += format[0 : i+1]
		format = format[i+1:]

		conversionEndIndex := strings.IndexAny(format, conversionSpecifiers)
		if conversionEndIndex == -1 {
			out += "MISSING CONVERSION(" + format + ")"
			break
		}

		c := format[conversionEndIndex]
		mod := format[0:conversionEndIndex]
		format = format[conversionEndIndex+1:]

		if c == '%' && mod == "" {
			out += string(c)
			continue
		}

		trimmed := strings.TrimLeft(mod, validModifiers)
		if trimmed != "" {
			out += "UNEXPECTED MODIFIER(" + string(trimmed[0]) + ")"
		}

		conversion := Conversion{
			Conversion: c,
			Modifiers:  mod,
			Suffix:     format,
			Arg:        args[arg],
		}

		if callback != nil {
			conversion = callback(conversion)
		}
		conversion = munge(conversion)

		out += conversion.Modifiers + string(conversion.Conversion)
		format = conversion.Suffix
		args[arg] = conversion.Arg

		arg++
	}

	return out, args
}

func munge(c Conversion) Conversion {
	if c.Conversion == 'i' {
		c.Conversion = 'd'
	}

	if c.Conversion == 'd' || c.Conversion == 'u' || c.Conversion == 'x' || c.Conversion == 'X' ||
		c.Conversion == 'o' {

		// TODO: get int size
		size := 4
		signed := true
		switch {
		case strings.Contains(c.Modifiers, "ll"):
			size = 8
		case strings.Contains(c.Modifiers, "l"):
			size = 8
		case strings.Contains(c.Modifiers, "hh"):
			size = 1
		case strings.Contains(c.Modifiers, "h"):
			size = 2
		case strings.Contains(c.Modifiers, "z"):
			size = 8
		}

		if c.Conversion != 'd' {
			signed = false
		}

		if c.Conversion == 'u' {
			c.Conversion = 'd'
		}

		c.Arg = cparse.CastExpression(c.Arg, size, signed)
	}

	if c.Conversion == 'p' && c.Modifiers == "" {
		c.Conversion = 'x'
		c.Modifiers = "016"
		c.Arg = cparse.CastExpression(c.Arg, 8, false)
	}

	modifiers := []byte(c.Modifiers)
	c.Modifiers = ""
	for _, m := range modifiers {
		if strings.IndexByte(trimmedConversionModfiers, m) == -1 {
			c.Modifiers += string(m)
		}
	}

	return c
}
