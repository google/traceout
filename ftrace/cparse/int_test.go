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

// Tests integer promotion, balancing, overflow, and return types on various operators
// TODO: multiplication overflow, shift types

import (
	"math"
	"testing"
)

const (
	negOne = -1
	minS8  = math.MinInt8
	minS16 = -0x8000
	minS32 = math.MinInt32
	minS64 = math.MinInt64
	maxS8  = math.MaxInt8
	maxS16 = 0x7fff
	maxS32 = math.MaxInt32
	maxS64 = math.MaxInt64
	maxU8  = math.MaxUint8
	maxU16 = 0xffff
	maxU32 = math.MaxUint32
	maxU64 = math.MaxUint64
)

type binaryOperatorTest struct {
	v1, v2 []Value
	result Value
}

var conversionTests = []binaryOperatorTest{
	{
		values(0, S8, U8, S16, U16, S32),
		values(0, S8, U8, S16, U16, S32),
		S32(0),
	},
	{
		values(0, S8, U8, S16, U16, S32, U32),
		values(0, U32),
		U32(0),
	},
	{
		values(0, S8, U8, S16, U16, S32, U32, S64),
		values(0, S64),
		S64(0),
	},
	{
		values(0, S8, U8, S16, U16, S32, U32, S64, U64),
		values(0, U64),
		U64(0),
	},
}

func TestOperatorConversion(t *testing.T) {
	testBinaryOperatorValueArray(t, "+", conversionTests, false)
	testBinaryOperatorValueArray(t, "+", conversionTests, true)
}

var additionTests = []binaryOperatorTest{
	// 0 + 1 == 1
	{
		values(0, S8, U8, S16, U16, S32),
		values(1, S8, U8, S16, U16, S32),
		S32(1),
	},
	{
		values(0, U32),
		values(1, S8, U8, S16, U16, S32),
		U32(1),
	},
	{
		values(0, U32),
		values(1, S8, U8, S16, U16, S32),
		U32(1),
	},

	// 1 + -1 == 0
	{
		values(1, S8, U8, S16, U16, S32),
		values(-1, S8, S16, S32),
		S32(0),
	},
	{
		values(1, U32),
		values(-1, S8, S16, S32),
		U32(0),
	},

	// 0xff + 1 == 0x100
	{
		values(maxU8, S8, U8, S16, U16, S32),
		values(1, S8, U8, S16, U16, S32),
		S32(maxU8 + 1),
	},
	{
		values(maxU8, S8, U8, S16, U16, S32),
		values(1, U32),
		U32(maxU8 + 1),
	},
	{
		values(maxU8, U32),
		values(1, S8, U8, S16, U16, S32),
		U32(maxU8 + 1),
	},
	{
		values(maxU8, S8, U8, S16, U16, S32, U32, S64),
		values(1, U64),
		U64(maxU8 + 1),
	},
	{
		values(maxU8, U64),
		values(1, S8, U8, S16, U16, S32, U32, S64),
		U64(maxU8 + 1),
	},

	// 0x7f + 1 == 0x80
	{
		values(maxS8, S8, U8, S16, U16, S32),
		values(1, S8, U8, S16, U16, S32),
		S32(maxS8 + 1),
	},
	{
		values(maxS8, S8, U8, S16, U16, S32),
		values(1, U32),
		U32(maxS8 + 1),
	},
	{
		values(maxS8, U32),
		values(1, S8, U8, S16, U16, S32),
		U32(maxS8 + 1),
	},

	// -0x80 + -1 == -0x81
	{
		values(minS8, S8, S16, S32),
		values(-1, S8, S16, S32),
		S32(minS8 - 1),
	},
	{
		values(minS8, S64),
		values(-1, S8, S16, S32, S64),
		S64(minS8 - 1),
	},

	// 0xffff + 1 == 0x10000
	{
		values(maxU16, S8, U8, S16, U16, S32),
		values(1, S8, U8, S16, U16, S32),
		S32(maxU16 + 1),
	},
	{
		values(maxU16, S8, U8, S16, U16, S32),
		values(1, U32),
		U32(maxU16 + 1),
	},
	{
		values(maxU16, U32),
		values(1, S8, U8, S16, U16, S32),
		U32(maxU16 + 1),
	},
	{
		values(maxU16, S8, U8, S16, U16, S32, U32, S64),
		values(1, U64),
		U64(maxU16 + 1),
	},
	{
		values(maxU16, U64),
		values(1, S8, U8, S16, U16, S32, U32, S64),
		U64(maxU16 + 1),
	},

	// 0x7fff + 1 == 0x8000
	{
		values(maxS16, S8, U8, S16, U16, S32),
		values(1, S8, U8, S16, U16, S32),
		S32(maxS16 + 1),
	},
	{
		values(maxS16, S8, U8, S16, U16, S32),
		values(1, U32),
		U32(maxS16 + 1),
	},
	{
		values(maxS16, U32),
		values(1, S8, U8, S16, U16, S32),
		U32(maxS16 + 1),
	},

	// -0x8000 + -1 == -0x8001
	{
		values(minS16, S16, S32),
		values(-1, S8, S16, S32),
		S32(minS16 - 1),
	},
	{
		values(minS16, S64),
		values(-1, S8, S16, S32, S64),
		S64(minS16 - 1),
	},

	// 0xffffffff + 1 == 0
	{
		values(maxU32, U32),
		values(1, S8, U8, S16, U16, S32, U32),
		U32(0),
	},
	{
		values(maxU32, U32, S64),
		values(1, U64),
		U64(maxU32 + 1),
	},
	{
		values(maxU32, U64),
		values(1, S8, U8, S16, U16, S32, U32, S64),
		U64(maxU32 + 1),
	},

	// 0x7fffffff + 1 == 0x80000000
	{
		values(maxS32, S32),
		values(1, S8, U8, S16, U16, S32),
		S32(minS32),
	},
	{
		values(maxS32, S32),
		values(1, U32),
		U32(maxS32 + 1),
	},
	{
		values(maxS32, U32),
		values(1, S8, U8, S16, U16, S32),
		U32(maxS32 + 1),
	},

	// -0x80000000 + -1 == -0x80000001
	{
		values(minS32, S32),
		values(-1, S8, S16, S32),
		S32(maxS32),
	},
	{
		values(minS32, S64),
		values(-1, S8, S16, S32, S64),
		S64(minS32 - 1),
	},

	// 0xffffffffffffffff + 1 == 0
	{
		[]Value{NewValueInt(maxU64, 8, false)},
		values(1, S8, U8, S16, U16, S32, U32, S64, U64),
		U64(0),
	},

	// 0x7fffffffffffffff + 1 == 0x8000000000000000
	{
		values(maxS64, S64),
		values(1, S8, U8, S16, U16, S32, U32, S64),
		S64(minS64),
	},
	{
		values(maxS64, S64),
		values(1, U64),
		NewValueInt(maxS64+1, 8, false),
	},
	{
		values(maxS64, U64),
		values(1, S8, U8, S16, U16, S32, S64, U64),
		NewValueInt(maxS64+1, 8, false),
	},

	// -0x8000000000000000 + -1 == -0x8000000000000001
	{
		values(minS64, S64),
		values(-1, S8, S16, S32, S64),
		S64(maxS64),
	},
}

func TestOperatorAddition(t *testing.T) {
	testBinaryOperatorValueArray(t, "+", additionTests, false)
	testBinaryOperatorValueArray(t, "+", additionTests, true)
}

func testBinaryOperatorValueArray(t *testing.T, op string, tests []binaryOperatorTest, swap bool) {
	token := token{
		typ: stringToToken[op],
		val: op,
	}

	for _, test := range tests {
		for _, _v1 := range test.v1 {
			for _, v2 := range test.v2 {
				v1 := _v1
				if swap {
					v1, v2 = v2, v1
				}
				args := []Expression{
					newConstantExpression(nil, v1),
					newConstantExpression(nil, v2),
				}
				Expression := newOperatorExpression(token, args)
				result := Expression.Value(testScope{})
				if !result.IsInt() {
					t.Errorf("%v %s %v result is not int", v1.intVal, op, v2.intVal)
				}
				if result.intVal != test.result.intVal {
					t.Errorf("%v %s %v expected %v got %v", v1.intVal, op, v2.intVal,
						test.result.intVal, result.intVal)
				}
			}
		}
	}
}

func values(val int64, types ...func(int64) Value) []Value {
	ret := make([]Value, len(types))
	for i, t := range types {
		ret[i] = t(val)
	}

	return ret
}

func S8(i int64) Value {
	return NewValueInt(uint64(i), 1, true)
}

func S16(i int64) Value {
	return NewValueInt(uint64(i), 2, true)
}

func S32(i int64) Value {
	return NewValueInt(uint64(i), 4, true)
}

func S64(i int64) Value {
	return NewValueInt(uint64(i), 8, true)
}

func U8(i int64) Value {
	return NewValueInt(uint64(i), 1, false)
}

func U16(i int64) Value {
	return NewValueInt(uint64(i), 2, false)
}

func U32(i int64) Value {
	return NewValueInt(uint64(i), 4, false)
}

func U64(i int64) Value {
	return NewValueInt(uint64(i), 8, false)
}
