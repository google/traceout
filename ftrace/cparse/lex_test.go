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
	"testing"
)

const (
	schedSwitchFormat = `"prev_comm=%s prev_pid=%d prev_prio=%d prev_state=%s%s ==> next_comm=%s next_pid=%d next_prio=%d", REC->prev_comm, REC->prev_pid, REC->prev_prio, REC->prev_state & (1024-1) ? __print_flags(REC->prev_state & (1024-1), "|", { 1, "S"} , { 2, "D" }, { 4, "T" }, { 8, "t" }, { 16, "Z" }, { 32, "X" }, { 64, "x" }, { 128, "K" }, { 256, "W" }, { 512, "P" }) : "R", REC->prev_state & 1024 ? "+" : "", REC->next_comm, REC->next_pid, REC->next_prio`
)

func TestLexSchedSwitch(t *testing.T) {
	l := NewLexer(schedSwitchFormat)
	for token := l.nextToken(); token.typ != tokenNone; token = l.nextToken() {
		if token.typ == tokenError {
			t.Error("error while lexing sched switch format")
		}
	}
}

type tokenTest struct {
	in  string
	out []tokenType
}

var operatorTests = []tokenTest{
	{"+ 1", []tokenType{tokenPlus, tokenNumber}},
	{"- 1", []tokenType{tokenMinus, tokenNumber}},
	{"! 1", []tokenType{tokenBoolNot, tokenNumber}},
	{"~ 1", []tokenType{tokenNot, tokenNumber}},
	{"1 * 1", []tokenType{tokenNumber, tokenMult, tokenNumber}},
	{"1 / 1", []tokenType{tokenNumber, tokenDiv, tokenNumber}},
	{"1 % 1", []tokenType{tokenNumber, tokenMod, tokenNumber}},
	{"1 + 1", []tokenType{tokenNumber, tokenPlus, tokenNumber}},
	{"1 - 1", []tokenType{tokenNumber, tokenMinus, tokenNumber}},
	{"1 << 1", []tokenType{tokenNumber, tokenLeftShift, tokenNumber}},
	{"1 >> 1", []tokenType{tokenNumber, tokenRightShift, tokenNumber}},
	{"1 < 1", []tokenType{tokenNumber, tokenLess, tokenNumber}},
	{"1 <= 1", []tokenType{tokenNumber, tokenLessEqual, tokenNumber}},
	{"1 > 1", []tokenType{tokenNumber, tokenGreater, tokenNumber}},
	{"1 >= 1", []tokenType{tokenNumber, tokenGreaterEqual, tokenNumber}},
	{"1 == 1", []tokenType{tokenNumber, tokenEqual, tokenNumber}},
	{"1 != 1", []tokenType{tokenNumber, tokenNotEqual, tokenNumber}},
	{"1 & 1", []tokenType{tokenNumber, tokenAnd, tokenNumber}},
	{"1 ^ 1", []tokenType{tokenNumber, tokenXor, tokenNumber}},
	{"1 | 1", []tokenType{tokenNumber, tokenOr, tokenNumber}},
	{"1 && 1", []tokenType{tokenNumber, tokenBoolAnd, tokenNumber}},
	{"1 || 1", []tokenType{tokenNumber, tokenBoolOr, tokenNumber}},
	{"1 ? 1 : 1", []tokenType{tokenNumber, tokenQuestion, tokenNumber, tokenColon, tokenNumber}},
	{"( 1 )", []tokenType{tokenLeftParen, tokenNumber, tokenRightParen}},
	{"1 , 1", []tokenType{tokenNumber, tokenComma, tokenNumber}},
	{"{ 1 }", []tokenType{tokenLeftBracket, tokenNumber, tokenRightBracket}},
	{"+1", []tokenType{tokenPlus, tokenNumber}},
	{"-1", []tokenType{tokenMinus, tokenNumber}},
	{"!1", []tokenType{tokenBoolNot, tokenNumber}},
	{"~1", []tokenType{tokenNot, tokenNumber}},
	{"1*1", []tokenType{tokenNumber, tokenMult, tokenNumber}},
	{"1/1", []tokenType{tokenNumber, tokenDiv, tokenNumber}},
	{"1%1", []tokenType{tokenNumber, tokenMod, tokenNumber}},
	{"1+1", []tokenType{tokenNumber, tokenPlus, tokenNumber}},
	{"1-1", []tokenType{tokenNumber, tokenMinus, tokenNumber}},
	{"1<<1", []tokenType{tokenNumber, tokenLeftShift, tokenNumber}},
	{"1>>1", []tokenType{tokenNumber, tokenRightShift, tokenNumber}},
	{"1<1", []tokenType{tokenNumber, tokenLess, tokenNumber}},
	{"1<=1", []tokenType{tokenNumber, tokenLessEqual, tokenNumber}},
	{"1>1", []tokenType{tokenNumber, tokenGreater, tokenNumber}},
	{"1>=1", []tokenType{tokenNumber, tokenGreaterEqual, tokenNumber}},
	{"1==1", []tokenType{tokenNumber, tokenEqual, tokenNumber}},
	{"1!=1", []tokenType{tokenNumber, tokenNotEqual, tokenNumber}},
	{"1&1", []tokenType{tokenNumber, tokenAnd, tokenNumber}},
	{"1^1", []tokenType{tokenNumber, tokenXor, tokenNumber}},
	{"1|1", []tokenType{tokenNumber, tokenOr, tokenNumber}},
	{"1&&1", []tokenType{tokenNumber, tokenBoolAnd, tokenNumber}},
	{"1||1", []tokenType{tokenNumber, tokenBoolOr, tokenNumber}},
	{"1?1:1", []tokenType{tokenNumber, tokenQuestion, tokenNumber, tokenColon, tokenNumber}},
	{"(1)", []tokenType{tokenLeftParen, tokenNumber, tokenRightParen}},
	{"1,1", []tokenType{tokenNumber, tokenComma, tokenNumber}},
	{"{1}", []tokenType{tokenLeftBracket, tokenNumber, tokenRightBracket}},
	{"0x1", []tokenType{tokenNumber}},
	{"01", []tokenType{tokenNumber}},
	{"0X1", []tokenType{tokenNumber}},
	{"0x1u", []tokenType{tokenNumber}},
	{"0x1U", []tokenType{tokenNumber}},
	{"0x1UL", []tokenType{tokenNumber}},
	{"0x1LL", []tokenType{tokenNumber}},
}

func TestLexOperators(t *testing.T) {
	testArray(t, operatorTests)
}

var symbolTests = []tokenTest{
	{"+a+", []tokenType{tokenPlus, tokenSymbol, tokenPlus}},
	{"+_a+", []tokenType{tokenPlus, tokenSymbol, tokenPlus}},
	{"+abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_+", []tokenType{tokenPlus, tokenSymbol, tokenPlus}},
	{"a(", []tokenType{tokenSymbol, tokenLeftParen}},
}

func TestLexSymbols(t *testing.T) {
	testArray(t, symbolTests)
}

var whitespaceTests = []tokenTest{
	{"1", []tokenType{tokenNumber}},
	{" 1 ", []tokenType{tokenNumber}},
	{"  1  ", []tokenType{tokenNumber}},
	{"\t1\t", []tokenType{tokenNumber}},
	{"\n1\n", []tokenType{tokenNumber}},
}

func TestLexWhitespace(t *testing.T) {
	testArray(t, whitespaceTests)
}

func testArray(t *testing.T, tests []tokenTest) {
	for _, test := range tests {
		l := NewLexer(test.in)
		if tokens, ok := l.expectTokens(test.out); !ok {
			t.Error("lex", test.in, "want", test.out, "got", tokens)
		}
	}
}

func BenchmarkLexSchedSwitch(b *testing.B) {
	for i := 0; i < b.N; i++ {
		l := NewLexer(schedSwitchFormat)
		for t := l.nextToken(); t.typ != tokenNone; t = l.nextToken() {
		}
	}
}
