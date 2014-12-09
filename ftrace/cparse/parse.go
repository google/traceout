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
	"fmt"
	"strings"
)

type parser struct {
	lex    *lexer
	tokens []token
	scope  Scope
}

func NewParser(lex *lexer, scope Scope) *parser {
	return &parser{
		lex:   lex,
		scope: scope,
	}
}

func (p *parser) parse() (Expression, error) {
	tokens := p.lex.allTokens()
	return p.parseExpression(tokens)
}

func (p *parser) parseExpression(tokens []token) (e Expression, err error) {
	// create intermediate list
	l := newIntermediateList(tokens)

	// Call parseSubExpression
	size, err := p.parseSubExpression(l, tokenNone)
	if err != nil {
		return nil, err
	}

	if size != 1 {
		return nil, nil
	}
	return l.expression(0), nil
}

// Replaces a series of tokens and expressions at the beginning of an intermediateList with
// a single valueExpression
func (p *parser) parseSubExpression(l *intermediateList, endToken tokenType) (int, error) {
	// find first ( or { or endToken, call parseSubExpression if necessary, repeat
	for {
		i, t := l.findToken(0, []tokenType{tokenLeftBracket, tokenLeftParen, endToken})
		if t.typ == tokenNone {
			break
		}

		if t.typ == endToken {
			l = l.slice(0, i)
			break
		}

		subL := l.slice(i+1, -1)
		subEndToken := tokenNone
		switch t.typ {
		case tokenLeftParen:
			subEndToken = tokenRightParen
		case tokenLeftBracket:
			subEndToken = tokenRightBracket
		default:
			panic("bad start token " + t.val)
		}
		subSize, err := p.parseSubExpression(subL, subEndToken)
		if err != nil {
			return -1, err
		}

		// subsize should be 0 or 1
		// i points to the start token, i+subSize+1 points to the end token
		if l.token(i+subSize+1).typ != subEndToken {
			return -1, fmt.Errorf("missing closing token for " + t.val)
		}

		e := l.expression(i + 1)
		if t.typ == tokenLeftParen {
			if _, ok := e.(typeExpression); ok {
				// a type expression inside parenthesis must be a cast, but there is no way to know
				// what the cast applies to until later, so keep it as a placeholder for now
				l.replaceWithPlaceholder(i, 3, e, placeholderCast)
			} else if ft := l.token(i - 1); ft.typ == tokenSymbol {
				var args []Expression
				if subSize == 0 {
					args = []Expression{}
				} else if l, ok := e.(listExpression); ok {
					args = l.vals
				} else {
					args = []Expression{e}
				}
				function := p.scope.GetFunction(ft.val)
				l.replace(i-1, subSize+3, newFunctionExpression(function, ft.val, args))
			} else if subSize == 1 {
				l.replace(i, subSize+2, e)
			} else {
				return -1, fmt.Errorf("empty parens without function call?")
			}
		} else {
			l.replace(i, subSize+2, newStructExpression(e))
		}
	}

	// replace all literal tokens with constantExpressions
	for {
		i, t := l.findToken(0, []tokenType{tokenNumber, tokenString})
		if i < 0 {
			break
		}
		l.replace(i, 1, newConstantExpressionFromString(t.val))
	}

	// replace all symbol tokens variableExpression or typeExpression
	// TODO: array subscripts and TODO: postfix increments
	for {
		i, t := l.findToken(0, []tokenType{tokenSymbol})
		if i < 0 {
			break
		}

		typeKeywords := []string(nil)
		for c := 0; ; c++ {
			t := l.token(i + c)
			if t.typ == tokenSymbol && isTypeKeyword(t.val) {
				typeKeywords = append(typeKeywords, t.val)
			} else {
				break
			}
		}
		tokensUsed := len(typeKeywords)

		// Try a type defined by the scope
		if len(typeKeywords) == 0 {
			scopeType := p.scope.GetType(t.val)
			if scopeType != "" {
				typeKeywords = strings.Split(scopeType, " ")
				tokensUsed = 1
			}
		}

		if len(typeKeywords) > 0 {
			t, err := keywordsToIntType(typeKeywords)
			if err != nil {
				return -1, err
			}
			l.replace(i, tokensUsed, newTypeExpression(t))
		} else {
			v := p.scope.GetVariable(t.val)
			ve := newVariableExpression(v, t.val)
			if c, ok := v.(constantVariable); ok {
				l.replace(i, 1, newConstantExpression(ve, c.value))
			} else {
				l.replace(i, 1, ve)
			}
		}
	}

	// handle unary operators, casts, and TODO: prefix increments
	// also flattens any paren expressions it finds that are not casts
	i := -1
	for {
		var t token
		i, t = l.findTokenDir(i, unaryOperators, rightToLeft)
		j, e := l.findPlaceholderDir(i, rightToLeft, placeholderCast)
		if i < 0 && j < 0 {
			break
		}

		if j > i {
			// Rightmost operator at this precedence level is a cast
			i = j
			after := l.expression(i + 1)
			if after == nil {
				return -1, fmt.Errorf("expected expression to the right of cast (%s)", e.Dump())
			}
			l.replace(i, 2, newCastExpression(e.(typeExpression), after))
			continue
		}

		after := l.expression(i + 1)
		if after == nil {
			return -1, fmt.Errorf("expected expression to the right of %s", t.val)
		}

		// special case for unary operators + and -
		// + and - are binary operators if the token to the left is a value (a symbol, a literal,
		// or an expression), unary otherwise.  All other unary operators are invalid if the token
		// to the left is a value, so just reject unary operators with values to the left and binary
		// + and - will be handled by a later pass.
		if t.typ == tokenPlus || t.typ == tokenMinus {
			before := l.expression(i - 1)
			if before != nil {
				i--
				continue
			}
		}

		e = newOperatorExpression(t, []Expression{after})
		l.replace(i, 2, e)
	}

	// handle binary operators
	for _, opLevel := range binaryOperatorPrecdence {
		for {
			i, t := l.findTokenDir(0, opLevel.typs, opLevel.dir)
			if i < 0 {
				break
			}

			before := l.expression(i - 1)
			after := l.expression(i + 1)
			if before == nil {
				return -1, fmt.Errorf("expected expression to the left of %s", t.val)
			}
			if after == nil {
				return -1, fmt.Errorf("expected expression to the right of %s", t.val)
			}

			e := newOperatorExpression(t, []Expression{before, after})
			l.replace(i-1, 3, e)
		}
	}

	// handle trinary operators
	for {
		i, t := l.findTokenDir(-1, []tokenType{tokenQuestion}, rightToLeft)
		if i < 0 {
			break
		}

		left := l.expression(i - 1)
		middle := l.expression(i + 1)
		if left == nil {
			return -1, fmt.Errorf("expected expression before '?'")
		}
		if middle == nil {
			return -1, fmt.Errorf("expected expression after '?'")
		}

		// We can cheat here, as ?: is the lowest priority operator the only valid
		// intermediate list here is {expression, '?', expression, ':', expression}
		// and the ':' operator can only be at i+2
		if l.token(i+2).typ != tokenColon {
			return -1, fmt.Errorf("expected ':' after '?'")
		}
		right := l.expression(i + 3)
		if right == nil {
			return -1, fmt.Errorf("expected expression after ':'")
		}

		e := newOperatorExpression(t, []Expression{left, middle, right})
		l.replace(i-1, 5, e)
	}

	// handle commas
	for {
		i, t := l.findTokenDir(0, []tokenType{tokenComma}, leftToRight)
		if i < 0 {
			break
		}
		before := l.expression(i - 1)
		after := l.expression(i + 1)
		if before == nil {
			return -1, fmt.Errorf("expected expression to the left of %s", t.val)
		}
		if after == nil {
			return -1, fmt.Errorf("expected expression to the right of %s", t.val)
		}

		e := newListExpression(before, after)
		l.replace(i-1, 3, e)
	}

	// sanity check for single expression
	if l.len() > 1 {
		return -1, fmt.Errorf("failed to parse expression %s", l.dump())
	}

	return l.len(), nil
}

var unaryOperators = []tokenType{tokenPlus, tokenMinus, tokenNot, tokenBoolNot}

var binaryOperatorPrecdence = []struct {
	typs     []tokenType
	dir      direction
	operands int
}{
	{[]tokenType{tokenMult, tokenDiv, tokenMod}, leftToRight, 2},
	{[]tokenType{tokenPlus, tokenMinus}, leftToRight, 2},
	{[]tokenType{tokenLeftShift, tokenRightShift}, leftToRight, 2},
	{[]tokenType{tokenLess, tokenLessEqual, tokenGreater, tokenGreaterEqual}, leftToRight, 2},
	{[]tokenType{tokenEqual, tokenNotEqual}, leftToRight, 2},
	{[]tokenType{tokenAnd}, leftToRight, 2},
	{[]tokenType{tokenXor}, leftToRight, 2},
	{[]tokenType{tokenOr}, leftToRight, 2},
	{[]tokenType{tokenBoolAnd}, leftToRight, 2},
	{[]tokenType{tokenBoolOr}, leftToRight, 2},
}

type testScope struct{}
type testVariable struct{}
type testFunction struct{}

func (testScope) GetVariable(name string) Variable {
	return testVariable{}
}

func (testScope) GetFunction(name string) Function {
	return testFunction{}
}

func (testScope) GetType(name string) string {
	if name == "t" {
		return "int"
	} else {
		return ""
	}
}

func (testFunction) Get(ctx EvalContext, args []Value) Value {
	return NewValueInt(uint64(len(args)), 4, true)
}

func (testVariable) Get(ctx EvalContext) Value {
	return NewValueInt(1, 4, true)
}
