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
	"strconv"
	"strings"
)

type expressionBase struct {
	constant bool
}

func (e expressionBase) IsConstant() bool {
	return e.constant
}

//
// Operators, unary, binary or trinary
//

type operatorExpression struct {
	expressionBase
	operator token
	args     []Expression
}

func newOperatorExpression(operator token, args []Expression) (e Expression) {
	e = operatorExpression{
		operator: operator,
		args:     args,
	}

	if listIsConstants(args) {
		e = toConstant(e)
	}

	return
}

func (e operatorExpression) Value(ctx EvalContext) Value {
	var v1, v2, v3 Value

	if len(e.args) >= 1 {
		v1 = e.args[0].Value(ctx)
		if v1.IsError() {
			return v1
		}
	}

	if len(e.args) >= 2 {
		v2 = e.args[1].Value(ctx)
		if v2.IsError() {
			return v2
		}
	}

	if len(e.args) >= 3 {
		v3 = e.args[2].Value(ctx)
		if v3.IsError() {
			return v3
		}
	}

	// Operand checking
	switch e.operator.typ {
	case tokenNot, tokenBoolNot:
		if len(e.args) != 1 {
			return NewValueError("wrong number of args to " + e.operator.val)
		}
		if !v1.IsInt() {
			return NewValueError("expected integer as left operand to " + e.operator.val)
		}
	case tokenPlus, tokenMinus:
		if len(e.args) == 1 {
			if !v1.IsInt() {
				return NewValueError("expected integer as left operand to " + e.operator.val)
			}
			break
		} else if len(e.args) != 2 {
			return NewValueError("wrong number of args to " + e.operator.val)
		}
		// binary version of + or -
		fallthrough
	case tokenMult, tokenDiv, tokenMod,
		tokenLeftShift, tokenRightShift,
		tokenLess, tokenLessEqual,
		tokenGreater, tokenGreaterEqual,
		tokenEqual, tokenNotEqual,
		tokenAnd, tokenXor, tokenOr,
		tokenBoolAnd, tokenBoolOr:
		if len(e.args) != 2 {
			return NewValueError("wrong number of args to " + e.operator.val)
		}
		if !v1.IsInt() {
			return NewValueError("expected integer as left operand to " + e.operator.val)
		}
		if !v2.IsInt() {
			return NewValueError("expected integer as right operand to " + e.operator.val)
		}
	case tokenQuestion:
		if len(e.args) != 3 {
			return NewValueError("wrong number of args to " + e.operator.val)
		}
		if !v1.IsInt() {
			return NewValueError("expected integer as operand to " + e.operator.val)
		}
	default:
		return NewValueError("unknown operator " + e.operator.val)
	}

	// Operand type conversion
	switch e.operator.typ {
	case tokenNot, tokenBoolNot:
		v1.intType = intPromote(v1.intType)
	case tokenPlus, tokenMinus:
		v1.intType = intPromote(v1.intType)
		if len(e.args) == 1 {
			break
		}
		// binary version of + or -
		fallthrough
	case tokenMult, tokenDiv, tokenMod,
		tokenLess, tokenLessEqual,
		tokenGreater, tokenGreaterEqual,
		tokenEqual, tokenNotEqual,
		tokenAnd, tokenXor, tokenOr,
		tokenBoolAnd, tokenBoolOr:
		v1.intType, v2.intType = intBalance(intPromote(v1.intType), intPromote(v2.intType))
	case tokenLeftShift, tokenRightShift:
		v1.intType, v2.intType = intPromote(v1.intType), intPromote(v2.intType)
	case tokenQuestion:
		v2.intType, v3.intType = intBalance(intPromote(v2.intType), intPromote(v3.intType))
	default:
		return NewValueError("unknown operator " + e.operator.val)
	}

	// Operand evaluation
	switch e.operator.typ {
	case tokenPlus:
		if len(e.args) == 1 {
			return v1
		}
		return newValueIntLike(v1, v1.AsUint64()+v2.AsUint64())
	case tokenMinus:
		if len(e.args) == 1 {
			return newValueIntLike(v1, -v1.AsUint64())
		}
		return newValueIntLike(v1, v1.AsUint64()-v2.AsUint64())
	case tokenNot:
		return newValueIntLike(v1, ^v1.AsUint64())
	case tokenBoolNot:
		return NewValueBool(!v1.AsBool())
	case tokenMult:
		return newValueIntLike(v1, v1.AsUint64()*v2.AsUint64())
	case tokenDiv:
		invert := false
		if v1.intType.signed && v1.AsInt() < 0 {
			invert = !invert
			v1.intVal = uint64(-v1.AsInt())
		}
		if v2.intType.signed && v2.AsInt() < 0 {
			invert = !invert
			v2.intVal = uint64(-v2.AsInt())
		}
		result := v1.AsUint64() / v2.AsUint64()
		if invert {
			result = uint64(-int64(result))
		}
		return newValueIntLike(v1, result)
	case tokenMod:
		invert := false
		if v1.intType.signed && v1.AsInt() < 0 {
			invert = true
			v1.intVal = uint64(-v1.AsInt())
		}
		if v2.intType.signed && v2.AsInt() < 0 {
			v2.intVal = uint64(-v2.AsInt())
		}
		result := v1.AsUint64() % v2.AsUint64()
		if invert {
			result = uint64(-int64(result))
		}
		return newValueIntLike(v1, result)
	case tokenLeftShift:
		return newValueIntLike(v1, v1.AsUint64()<<v2.AsUint64())
	case tokenRightShift:
		return newValueIntLike(v1, v1.AsUint64()>>v2.AsUint64())
	case tokenLess:
		if v1.intType.signed {
			return NewValueBool(v1.AsInt() < v2.AsInt())
		} else {
			return NewValueBool(v1.AsUint64() < v2.AsUint64())
		}
	case tokenLessEqual:
		if v1.intType.signed {
			return NewValueBool(v1.AsInt() <= v2.AsInt())
		} else {
			return NewValueBool(v1.AsUint64() <= v2.AsUint64())
		}
	case tokenGreater:
		if v1.intType.signed {
			return NewValueBool(v1.AsInt() > v2.AsInt())
		} else {
			return NewValueBool(v1.AsUint64() > v2.AsUint64())
		}
	case tokenGreaterEqual:
		if v1.intType.signed {
			return NewValueBool(v1.AsInt() >= v2.AsInt())
		} else {
			return NewValueBool(v1.AsUint64() >= v2.AsUint64())
		}
	case tokenEqual:
		return NewValueBool(v1.AsUint64() == v2.AsUint64())
	case tokenNotEqual:
		return NewValueBool(v1.AsUint64() != v2.AsUint64())
	case tokenAnd:
		return newValueIntLike(v1, v1.AsUint64()&v2.AsUint64())
	case tokenXor:
		return newValueIntLike(v1, v1.AsUint64()^v2.AsUint64())
	case tokenOr:
		return newValueIntLike(v1, v1.AsUint64()|v2.AsUint64())
	case tokenBoolAnd:
		return NewValueBool(v1.AsBool() && v2.AsBool())
	case tokenBoolOr:
		return NewValueBool(v1.AsBool() || v2.AsBool())
	case tokenQuestion:
		if v1.AsBool() {
			return v2
		} else {
			return v3
		}
	default:
		return NewValueError("unknown operator " + e.operator.val)
	}
}

func (e operatorExpression) Dump() string {
	switch len(e.args) {
	case 1:
		return "(" + e.operator.val + e.args[0].Dump() + ")"
	case 2:
		return "(" + e.args[0].Dump() + " " + e.operator.val + " " + e.args[1].Dump() + ")"

	case 3:
		return "(" + e.args[0].Dump() + " ? " + e.args[1].Dump() + " : " + e.args[2].Dump() + ")"
	default:
		panic("bad operator")
	}
}

//
// Lists
//

type listExpression struct {
	expressionBase
	vals []Expression
}

func newListExpression(left, right Expression) (e Expression) {
	vals := []Expression{}
	if l, ok := left.(listExpression); ok {
		vals = append(vals, l.vals...)
	} else {
		vals = append(vals, left)
	}

	if l, ok := right.(listExpression); ok {
		vals = append(vals, l.vals...)
	} else {
		vals = append(vals, right)
	}

	e = listExpression{
		vals: vals,
	}

	if listIsConstants(vals) {
		e = toConstant(e)
	}

	return
}

func (e listExpression) Value(ctx EvalContext) Value {
	ret := make([]Value, len(e.vals))
	for i, v := range e.vals {
		ret[i] = v.Value(ctx)
	}
	return NewValueList(ret)
}

func (e listExpression) Dump() string {
	s := make([]string, len(e.vals))
	for i := range e.vals {
		s[i] = e.vals[i].Dump()
	}
	return "{" + strings.Join(s, ", ") + "}"
}

//
// Structs
//
type structExpression struct {
	expressionBase
	exp Expression
}

func newStructExpression(exp Expression) (e Expression) {
	return structExpression{
		exp: exp,
	}
}

func (e structExpression) Value(ctx EvalContext) Value {
	return e.exp.Value(ctx)
}

func (e structExpression) Dump() string {
	return "{" + e.exp.Dump() + "}"
}

//
// Literals, strings or integers, or constant expressions of literals
//

type constantExpression struct {
	expressionBase
	exp Expression
	val Value
}

// TODO: get int/long size from scope
func newConstantExpressionFromString(s string) Expression {
	var val Value
	if s[0] == '"' {
		val = NewValueString(s[1 : len(s)-1])
	} else {
		s := strings.ToLower(s)
		n := strings.TrimRight(s, "ul")
		suffix := s[len(n):]
		size := 4
		signed := true
		switch suffix {
		case "":
		case "u":
			signed = false
		case "l":
			size = 8
		case "lu", "ul":
			size = 8
			signed = false
		case "ll":
			size = 8
		case "llu", "ull":
			size = 8
			signed = false
		default:
			return newConstantExpression(nil, NewValueError("invalid integer suffix "+suffix))
		}
		i, err := strconv.ParseUint(n, 0, size*8)
		if err != nil {
			val = NewValueError("invalid integer constant: " + err.Error())
		} else {
			val = NewValueInt(uint64(i), size, signed)
		}
	}

	return newConstantExpression(nil, val)
}

func newConstantExpression(exp Expression, val Value) Expression {
	return constantExpression{
		val: val,
		exp: exp,
		expressionBase: expressionBase{
			constant: true,
		},
	}
}

func (e constantExpression) Value(ctx EvalContext) Value {
	return e.val
}

func (e constantExpression) Dump() string {
	if e.exp != nil {
		return e.exp.Dump()
	} else {
		return e.val.Dump()
	}
}

func toConstant(e Expression) Expression {
	return newConstantExpression(e, e.Value(nil))
}

func listIsConstants(l []Expression) bool {
	for _, a := range l {
		if !a.IsConstant() {
			return false
		}
	}
	return true

}

//
// Variables (event fields)
//
type variableExpression struct {
	expressionBase
	variable Variable
	name     string
}

func newVariableExpression(variable Variable, name string) Expression {
	return variableExpression{
		name:     name,
		variable: variable,
	}
}

func (e variableExpression) Value(ctx EvalContext) Value {
	if e.variable == nil {
		return NewValueError("unknown variable " + e.name)
	}
	return e.variable.Get(ctx)
}

func (e variableExpression) Dump() string {
	return e.name
}

//
// Functions
//

type functionExpression struct {
	expressionBase
	function Function
	name     string
	args     []Expression
}

func newFunctionExpression(function Function, name string, args []Expression) Expression {
	return functionExpression{
		name:     name,
		args:     args,
		function: function,
	}
}

func (e functionExpression) Value(ctx EvalContext) Value {
	argValues := make([]Value, len(e.args))
	for i, a := range e.args {
		v := a.Value(ctx)
		if v.IsError() {
			return v
		}
		argValues[i] = v
	}
	if e.function == nil {
		return NewValueError("unknown kernel function " + e.name)
	}
	return e.function.Get(ctx, argValues)
}

func (e functionExpression) Dump() string {
	args := make([]string, len(e.args))
	for i, a := range e.args {
		args[i] = a.Dump()
	}
	return e.name + "(" + strings.Join(args, ", ") + ")"
}

//
// Types
//
type typeExpression struct {
	expressionBase
	intType intType
}

func newTypeExpression(intType intType) Expression {
	return typeExpression{
		intType: intType,
	}
}

func (e typeExpression) Value(ctx EvalContext) Value {
	return NewValueError("type expression has no value")
}

func (e typeExpression) Dump() string {
	return e.intType.dump()
}

//
// Casts
//
type castExpression struct {
	expressionBase
	intType intType
	val     Expression
}

func newCastExpression(t typeExpression, val Expression) (e Expression) {
	e = castExpression{
		intType: t.intType,
		val:     val,
	}

	if val.IsConstant() {
		e = toConstant(e)
	}

	return e
}

func (e castExpression) Value(ctx EvalContext) Value {
	if e.val == nil {
		return NewValueError("cast expression evaluated without a value")
	}
	val := e.val.Value(ctx)
	if !val.IsInt() {
		return NewValueError("cast applied to non-integer " + val.Dump())
	}
	return newValueIntCast(val, e.intType)
}

func (e castExpression) Dump() string {
	v := "MISSING CAST VALUE"
	if e.val != nil {
		v = e.val.Dump()
	}
	return "(" + e.intType.dump() + ")" + v
}
