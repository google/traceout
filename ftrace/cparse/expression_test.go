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

// Tests string expressions that return a true or false value.

import (
	"testing"
)

var expressionFalseTests = []string{
	"0==1",
	"1==0",
	"0!=0",
	"1!=1",
	"0>0",
	"0>1",
	"0>=1",
	"1<0",
	"0<0",
	"1<=0",

	"-1>0",
	"0<-1",
	"-1>=0",
	"0<=-1",
}

var expressionTrueTests = []string{
	"0==0",
	"1==1",
	"0!=1",
	"1!=0",
	"0<1",
	"0<=0",
	"0<=1",
	"1>0",
	"0>=0",
	"1>=0",

	"0u<1u",
	"0u<=0u",
	"0u<=1u",
	"1u>0u",
	"0u>=0u",
	"1u>=0u",

	"-1<0",
	"0>-1",
	"-1<=0",
	"0>=-1",

	"1+1==2",
	"1-1==0",
	"1*1==1",
	"2*3==6",

	"8/4==2",
	"8%4==0",
	"8%3==2",
	"8/-3==-2",
	"8%-3==2",
	"-8/3==-2",
	"-8%3==-2",
	"-8/-3==2",
	"-8%-3==-2",

	"(1&3)==1",
	"(1&2)==0",
	"(1|3)==3",
	"(1|2)==3",
	"(1^2)==3",
	"(1^3)==2",

	"(1&&1)==1",
	"(0&&1)==0",
	"(0&&0)==0",
	"(1||1)==1",
	"(0||1)==1",
	"(0||0)==0",

	"1<<2==4",
	"1<<0==1",
	"4>>2==1",
	"4>>3==0",
	"0xffffffffu<<1==0xfffffffeu",

	"1?2:3==2",
	"0?2:3==3",

	"-(1)==-1",
	"+(1)==1",
	"!0",
	"!1==0",
	"-1u==0xffffffff",
	"+1u==1",
	"~0==0xffffffff",
	"~0u==0xffffffff",
	"~0ll==0xffffffffffffffffll",
	"~0ull==0xffffffffffffffffull",
}

func TestExpressions(t *testing.T) {
	testExpressions(t, expressionFalseTests, false)
	testExpressions(t, expressionTrueTests, true)
}

func testExpressions(t *testing.T, tests []string, expect bool) {
	for _, test := range tests {
		expressions, err := Parse(test, testScope{})
		if err != nil {
			t.Error("failed to parse \"" + test + "\": " + err.Error())
			continue
		}

		if len(expressions) != 1 {
			t.Error("failed to parse \"" + test + "\": got more than one expression")
			continue
		}

		got := expressions[0].Value(nil)

		if !got.IsInt() {
			t.Error("expected bool from \"" + test + "\", got " + got.Dump())
			continue
		}

		if got.AsBool() != expect {
			t.Errorf("\"%s\" (%s) expected %v got %v", test, expressions[0].Dump(), expect, got.AsBool())
			continue
		}
	}
}

func TestEqualityOperators(t *testing.T) {

}
