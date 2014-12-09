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
	"strings"
	"testing"
)

func TestParseSchedSwitch(t *testing.T) {
	e, err := Parse(schedSwitchFormat, testScope{})
	if err != nil {
		t.Error(err.Error())
		return
	}

	v := e[0].Value(nil)
	if !v.IsString() {
		t.Error("first argument not a string")
	}
}

type parseTest struct {
	in  string
	out string
}

var operatorParseTests = []parseTest{
	{"+ a", "(+a)"},
	{"- a", "(-a)"},
	{"! a", "(!a)"},
	{"~ a", "(~a)"},
	{"a*b", "(a * b)"},
	{"a/b", "(a / b)"},
	{"a%b", "(a % b)"},
	{"a+b", "(a + b)"},
	{"a-b", "(a - b)"},
	{"a<<b", "(a << b)"},
	{"a>>b", "(a >> b)"},
	{"a<b", "(a < b)"},
	{"a<=b", "(a <= b)"},
	{"a>b", "(a > b)"},
	{"a>=b", "(a >= b)"},
	{"a==b", "(a == b)"},
	{"a!=b", "(a != b)"},
	{"a&b", "(a & b)"},
	{"a^b", "(a ^ b)"},
	{"a|b", "(a | b)"},
	{"a&&b", "(a && b)"},
	{"a||b", "(a || b)"},
	{"a?b:c", "(a ? b : c)"},
	{"(a)", "a"},
	{"a,b", "a, b"},
	{"{a}", "{a}"},
	{"(int) a", "(int32)a"},
	{"(a)-b", "(a - b)"},
	{"(t)-b", "(int32)(-b)"},
	{"f (a)", "f(a)"},
	{"f(a,b)", "f(a, b)"},
	{"f ()", "f()"},
}

func TestParseOperators(t *testing.T) {
	testParseArray(t, operatorParseTests)
}

var operatorAssociativityTests = []parseTest{
	{"a * b / c % d", "(((a * b) / c) % d)"},
	{"a % b / c * d", "(((a % b) / c) * d)"},
	{"a + b - c", "((a + b) - c)"},
	{"a - b + c", "((a - b) + c)"},
	{"a << b >> c", "((a << b) >> c)"},
	{"a >> b << c", "((a >> b) << c)"},
	{"a < b <= c > d >= e", "((((a < b) <= c) > d) >= e)"},
	{"a >= b > c <= d < e", "((((a >= b) > c) <= d) < e)"},
	{"a == b != c", "((a == b) != c)"},
	{"a != b == c", "((a != b) == c)"},
	{"a & b & c", "((a & b) & c)"},
	{"a ^ b ^ c", "((a ^ b) ^ c)"},
	{"a | b | c", "((a | b) | c)"},
	{"a && b && c", "((a && b) && c)"},
	{"a || b || c", "((a || b) || c)"},
	{"a ? b ? c : d : e", "(a ? (b ? c : d) : e)"},
}

func TestParseOperatorAssociativity(t *testing.T) {
	testParseArray(t, operatorAssociativityTests)
}

var parseIntTests = []parseTest{
	{"1", "(int32)1"},
	{"1U", "(uint32)1"},
	{"1L", "(int64)1"},
	{"1UL", "(uint64)1"},
	{"1LL", "(int64)1"},
	{"1ULL", "(uint64)1"},
	{"1LLU", "(uint64)1"},
	{"1llu", "(uint64)1"},
	{"0xffffffffffffffffLLU", "(uint64)18446744073709551615"},
	{"0xff", "(int32)255"},
	{"077", "(int32)63"},
}

func TestParseInts(t *testing.T) {
	testParseArray(t, parseIntTests)
}

var operatorPrecedenceTests = []parseTest{
	{"a + + b", "(a + (+b))"},
	{"a + b * c", "(a + (b * c))"},
	{"a << b + c", "(a << (b + c))"},
	{"a < b << c", "(a < (b << c))"},
	{"a == b < c", "(a == (b < c))"},
	{"a & b == c", "(a & (b == c))"},
	{"a ^ b & c", "(a ^ (b & c))"},
	{"a | b ^ c", "(a | (b ^ c))"},
	{"a && b | c", "(a && (b | c))"},
	{"a || b && c", "(a || (b && c))"},
	{"a || b ? c || d : e || f", "((a || b) ? (c || d) : (e || f))"},
}

func TestParseOperatorPrecedence(t *testing.T) {
	testParseArray(t, operatorPrecedenceTests)
}

func testParseArray(t *testing.T, tests []parseTest) {
	for _, test := range tests {
		expressions, err := Parse(test.in, testScope{})
		if err != nil {
			t.Error(err.Error())
			return
		}

		dumps := make([]string, len(expressions))
		for i, e := range expressions {
			dumps[i] = e.Dump()
		}
		got := strings.Join(dumps, ", ")

		if got != test.out {
			t.Error("parse", test.in, "want", test.out, "got", got)
		}
	}
}
