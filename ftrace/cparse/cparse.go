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

// Lex, parse, and print format messages for ftrace events, given a specification like:
// print fmt: "prev_comm=%s prev_pid=%d prev_prio=%d prev_state=%s%s ==> next_comm=%s
// next_pid=%d next_prio=%d", REC->prev_comm, REC->prev_pid, REC->prev_prio,
// REC->prev_state & (1024-1) ? __print_flags(REC->prev_state & (1024-1), "|",
// { 1, "S"} , { 2, "D" }, { 4, "T" }, { 8, "t" }, { 16, "Z" }, { 32, "X" }, { 64, "x" },
// { 128, "K" }, { 256, "W" }, { 512, "P" }) : "R", REC->prev_state & 1024 ? "+" : "",
// REC->next_comm, REC->next_pid, REC->next_prio

import ()

// Expression is a parsed expression.  Use Value(ctx) to evaluate it in a given
// context.
type Expression interface {
	// Evaluate the Expression in a given context.  Can be called multiple times
	// with different contexts.  If IsConstant() returns true, can be called with
	// nil context.
	Value(ctx EvalContext) Value
	// Return a string representation of the result of parsing the expression for
	// debugging.
	Dump() string
	// Returns true if the expression is constant (no function calls, all referenced
	// variables are ConstantVariables).  If true, Value() will always return the
	// same value for any context, including nil.
	IsConstant() bool
}

// EvalContext is a context in which to evaluate an Expression.  It is not used
// directly by cparse, but is passed to the Get methods of Function and Variable
// objects to get their value in the current context.
type EvalContext interface{}

// Scope is the scope in which to parse and Expression.  Unknown symbols are
// passed to the methods of the Scope object to get a Variable or Function
// object, which is used during evaluation to get the value in the evaluation
// context.
type Scope interface {
	GetVariable(name string) Variable
	GetFunction(name string) Function
	GetType(name string) string
}

// A Function object is a handle to call a function when an Expression is being
// evaluated
type Function interface {
	// Called during Expression.Value(context) with the evaluation context and
	// the values of the arguments, and returns the value of the result
	Get(ctx EvalContext, args []Value) Value
}

// A Function object is a handle to get the value of a variable when an Expression
//is being evaluated
type Variable interface {
	// Called during Expression.Value(context) with the evaluation context returns
	// the value of the variable in the evaluation context
	Get(ctx EvalContext) Value
}

// Parse takes a string representing comma separated C expressions and a Scope
// object, and returns a slice of Expression objects.
func Parse(input string, scope Scope) ([]Expression, error) {
	l := NewLexer(input)
	p := NewParser(l, scope)

	e, err := p.parse()
	if err != nil {
		return nil, err
	}

	expressions := []Expression{}
	if l, ok := e.(listExpression); ok {
		for _, exp := range l.vals {
			expressions = append(expressions, exp)
		}
	} else {
		expressions = append(expressions, e)
	}

	return expressions, nil
}

type constantVariable struct {
	value Value
}

func (v constantVariable) Get(ctx EvalContext) Value {
	return v.value
}

// NewConstantVariable is a helper for use inside Scope.GetVariable if the value
// of the variable never changes.  It allows the parser to optimize the expression
// by collapsing operators on constant values at parse time instead of evaluating
// them at evaluation time.
func NewConstantVariable(value Value) Variable {
	return constantVariable{
		value: value,
	}
}

// CallFunction returns an Expression that evaluates to the the value of the
// function called with the given argument expression.
func CallFunction(function Function, name string, args []Expression) Expression {
	return newFunctionExpression(function, name, args)
}

// CastExpression returns an Expression that evaluates to the the value of the
// given expression cast to the given integer type.
func CastExpression(val Expression, size int, signed bool) Expression {
	return newCastExpression(newTypeExpression(intType{size, signed}).(typeExpression), val)
}
