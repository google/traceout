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

package cparse

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type valueType int

const intSize = 4

const (
	valueInt valueType = iota
	valueString
	valueList
	valueError
)

// a placeholder for a string, an int64, an array of values, or a valueError
type Value struct {
	typ       valueType
	stringVal string
	intVal    uint64
	intType   intType
	listVal   []Value
}

// Error
func NewValueError(error string, args ...interface{}) Value {
	if len(args) > 0 {
		error = fmt.Sprintf(error, args...)
	}
	return Value{
		typ:       valueError,
		stringVal: error,
	}
}

func (v Value) IsError() bool {
	return v.typ == valueError
}

func (v Value) AsError() error {
	return errors.New("value error: " + v.stringVal)
}

// Integer
func NewValueInt(val uint64, size int, signed bool) Value {
	return Value{
		typ:    valueInt,
		intVal: val,
		intType: intType{
			size:   size,
			signed: signed,
		},
	}
}

func newValueIntLike(i Value, val uint64) Value {
	i.intVal = val
	i.intVal = intSignExtend(intClamp(i.intVal, i.intType), i.intType)
	return i
}

func newValueIntCast(i Value, intType intType) Value {
	i.intType = intType
	i.intVal = intSignExtend(intClamp(i.intVal, i.intType), i.intType)
	return i
}

func (v Value) IsInt() bool {
	return v.typ == valueInt
}

func (v Value) AsInt() int64 {
	return int64(v.intVal)
}

func (v Value) AsUint64() uint64 {
	return intClamp(v.intVal, v.intType)
}

// Boolean, always promoted to int for now
func NewValueBool(b bool) Value {
	if b {
		return valueTrue
	}
	return valueFalse
}

var valueTrue = NewValueInt(1, 4, true)
var valueFalse = NewValueInt(0, 4, true)

func (v Value) AsBool() bool {
	return v.intVal != 0
}

// String
func NewValueString(s string) Value {
	return Value{
		typ:       valueString,
		stringVal: s,
	}
}

func (v Value) IsString() bool {
	return v.typ == valueString
}

func (v Value) AsString() string {
	return v.stringVal
}

// List
func NewValueList(vals []Value) Value {
	return Value{
		typ:     valueList,
		listVal: vals,
	}
}
func (v Value) IsList() bool {
	return v.typ == valueList
}

func (v Value) AsList() []Value {
	return v.listVal
}

// As interface (for use in sprintf)
func (v Value) AsInterface() interface{} {
	switch {
	case v.IsInt():
		if v.intType.signed {
			return v.AsInt()
		} else {
			return v.AsUint64()
		}
	case v.IsString():
		return v.AsString()
	case v.IsList():
		return v.AsList()
	case v.IsError():
		return v.AsError()
	default:
		panic("unknown value type: " + string(int(v.typ)))
	}
}

func (v Value) Dump() string {
	return v.dump()
}

func (v Value) dump() string {
	switch {
	case v.IsInt():
		typ := "(" + v.intType.dump() + ")"
		if v.intType.signed {
			return typ + strconv.FormatInt(v.AsInt(), 10)
		} else {
			return typ + strconv.FormatUint(v.AsUint64(), 10)
		}
	case v.IsString():
		return "\"" + v.AsString() + "\""
	case v.IsList():
		var s []string
		for _, a := range v.AsList() {
			s = append(s, a.dump())
		}
		return "{" + strings.Join(s, ", ") + "}"
	case v.IsError():
		return v.AsError().Error()
	default:
		panic("unknown value type: " + string(int(v.typ)))
	}
}

// Integer types
type intType struct {
	size   int
	signed bool
}

// Integer promotion and conversion
func intPromote(i intType) intType {
	if i.size < intSize {
		i.size = intSize
		i.signed = true
	}
	return i
}

func intBalance(a, b intType) (intType, intType) {
	switch {
	// If both operands have the same type, then no further conversion is needed.
	case a.signed == b.signed && a.size == b.size:
	// Otherwise, if both operands have signed integer types or both have unsigned integer
	// types, the operand with the type of lesser integer conversion rank is converted to the
	// type of the operand with greater rank.
	case a.signed == b.signed && a.size > b.size:
		b = intResize(b, a.size)
	case a.signed == b.signed && b.size > a.size:
		a = intResize(a, b.size)
	// Otherwise, if the operand that has unsigned integer type has rank greater or equal to the
	// rank of the type of the other operand, then the operand with signed integer type is
	// converted to the type of the operand with unsigned integer type.
	case !a.signed && a.size >= b.size:
		b = intResize(b, a.size)
		b = intUnsigned(b)
	case !b.signed && b.size >= a.size:
		a = intResize(a, b.size)
		a = intUnsigned(a)
	// Otherwise, if the type of the operand with signed integer type can represent all of the
	// values of the type of the operand with unsigned integer type, then the operand with
	// unsigned integer type is converted to the type of the operand with signed integer type.
	case a.signed && a.size > b.size:
		b = intResize(b, a.size)
		b = intSigned(b)
	case b.signed && b.size > a.size:
		a = intResize(a, b.size)
		a = intSigned(a)
	// Otherwise, both operands are converted to the unsigned integer type corresponding to
	// the type of the operand with signed integer type.
	case a.signed:
		a = intUnsigned(a)
		b = intResize(b, a.size)
	case b.signed:
		b = intUnsigned(b)
		a = intResize(a, b.size)
	default:
		panic("should never get here")
	}

	if a.signed != b.signed {
		panic("signedness mismatch after balancing")
	}

	if a.size != b.size {
		panic("size mismatch after balancing")
	}

	return a, b
}

func intResize(i intType, size int) intType {
	if i.size > size {
		panic("truncating in intResize")
	}
	i.size = size
	return i
}

func intSigned(i intType) intType {
	i.signed = true
	return i
}

func intUnsigned(i intType) intType {
	i.signed = false
	return i
}

func intSignExtend(val uint64, typ intType) uint64 {
	mask := uint64(1<<uint(typ.size*8)) - 1
	signMask := uint64(1 << uint((typ.size*8)-1))

	if typ.signed && (val&signMask) != 0 {
		return val | ^mask
	}

	return val
}

func intClamp(val uint64, typ intType) uint64 {
	mask := uint64(1<<uint(typ.size*8)) - 1

	return val & mask
}

func (i intType) dump() string {
	sign := ""
	if !i.signed {
		sign = "u"
	}
	return sign + "int" + strconv.Itoa(int(i.size*8))
}

var (
	intCharType      = intType{1, true}
	intUCharType     = intType{1, false}
	intShortType     = intType{2, true}
	intUShortType    = intType{2, false}
	intIntType       = intType{4, true}
	intUIntType      = intType{4, false}
	intLongType      = intType{8, true}
	intULongType     = intType{8, false}
	intLongLongType  = intType{8, true}
	intULongLongType = intType{8, false}
)

var intTypes = map[string]intType{
	"char":                   intCharType,
	"signed char":            intCharType,
	"unsigned char":          intUCharType,
	"short":                  intShortType,
	"signed short":           intShortType,
	"short int":              intShortType,
	"signed short int":       intShortType,
	"unsigned short":         intUShortType,
	"unsigned short int":     intUShortType,
	"int":                    intIntType,
	"signed":                 intIntType,
	"signed int":             intIntType,
	"unsigned":               intUIntType,
	"unsigned int":           intUIntType,
	"long":                   intLongType,
	"signed long":            intLongType,
	"long int":               intLongType,
	"signed long int":        intLongType,
	"unsigned long":          intULongType,
	"unsigned long int":      intULongType,
	"long long":              intLongLongType,
	"signed long long":       intLongLongType,
	"long long int":          intLongLongType,
	"signed long long int":   intLongLongType,
	"unsigned long long":     intULongLongType,
	"unsigned long long int": intULongLongType,
}

var intTypeSpecifiers = map[string]int{
	"void":     3,
	"char":     3,
	"short":    2,
	"int":      3,
	"long":     2,
	"signed":   1,
	"unsigned": 1,
	"_Bool":    3,
}

func keywordsToIntType(keywords []string) (intType, error) {
	for _, k := range keywords {
		if _, ok := intTypeSpecifiers[k]; !ok {
			return intType{}, fmt.Errorf("invalid type keyword: %s", k)
		}
	}

	keywords = append([]string(nil), keywords...)
	sort.Sort(canonicalIntTypeOrder{keywords})
	typeString := strings.Join(keywords, " ")

	if typ, ok := intTypes[typeString]; ok {
		return typ, nil
	}

	return intType{}, fmt.Errorf("invalid type: %s", strings.Join(keywords, " "))
}

type canonicalIntTypeOrder struct {
	sort.StringSlice
}

func (c canonicalIntTypeOrder) Less(i, j int) bool {
	return intTypeSpecifiers[c.StringSlice[i]] < intTypeSpecifiers[c.StringSlice[j]]
}

func isTypeKeyword(k string) bool {
	_, ok := intTypeSpecifiers[k]
	return ok
}
