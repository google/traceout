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
	"unicode"
)

type token struct {
	typ tokenType
	pos int
	val string
}

var nullToken = token{
	typ: tokenNone,
}

type tokenType int

type ascii int8

const eof = -1

const (
	tokenNone tokenType = iota
	tokenError

	tokenString
	tokenNumber

	tokenSymbol

	tokenPlus
	tokenMinus
	tokenBoolNot
	tokenNot

	tokenMult
	tokenDiv
	tokenMod

	//tokenPlus binary version
	//tokenMinus binary version

	tokenLeftShift
	tokenRightShift

	tokenLess
	tokenLessEqual
	tokenGreater
	tokenGreaterEqual

	tokenEqual
	tokenNotEqual

	tokenAnd
	tokenXor
	tokenOr

	tokenBoolAnd
	tokenBoolOr

	tokenQuestion
	tokenColon

	tokenLeftParen
	tokenRightParen
	tokenComma
	tokenLeftBracket
	tokenRightBracket
)

var stringToToken = map[string]tokenType{
	"+": tokenPlus,
	"-": tokenMinus,
	"!": tokenBoolNot,
	"~": tokenNot,

	"*": tokenMult,
	"/": tokenDiv,
	"%": tokenMod,

	"<<": tokenLeftShift,
	">>": tokenRightShift,

	"<":  tokenLess,
	"<=": tokenLessEqual,
	">":  tokenGreater,
	">=": tokenGreaterEqual,

	"==": tokenEqual,
	"!=": tokenNotEqual,

	"&": tokenAnd,
	"^": tokenXor,
	"|": tokenOr,

	"&&": tokenBoolAnd,
	"||": tokenBoolOr,

	"?": tokenQuestion,
	":": tokenColon,

	"(": tokenLeftParen,
	")": tokenRightParen,
	",": tokenComma,
	"{": tokenLeftBracket,
	"}": tokenRightBracket,
}

type lexer struct {
	input  string     // starting input string
	tokens chan token // channel of output tokens
	state  stateFn    // current parsing function
	pos    int        // current input position
	start  int        // input position of the beginning of the current token
}

func NewLexer(input string) *lexer {
	l := &lexer{
		input:  input,
		tokens: make(chan token),
	}
	go l.run()
	return l
}

func (l *lexer) nextToken() token {
	return <-l.tokens
}

func (l *lexer) allTokens() []token {
	tokens := []token{}
	for t := range l.tokens {
		tokens = append(tokens, t)
	}
	return tokens
}

// helper for state transtions that also trims whitespace
func (l *lexer) nextState() stateFn {
	l.trimLeft()
	return l.state(l)
}

func (l *lexer) run() {
	for l.state = lexNone; l.state != nil; {
		l.state = l.nextState()
	}
	close(l.tokens)
}

func (l *lexer) next() ascii {
	c := l.peek()
	l.pos++
	return c
}

func (l *lexer) backup() {
	l.pos--
}

func (l *lexer) peek() ascii {
	if l.pos >= len(l.input) {
		return eof
	}
	return ascii(l.input[l.pos])
}

func (l *lexer) trimLeft() {
	for isSpace(l.peek()) {
		l.next()
	}
	l.start = l.pos
	return
}

func (l *lexer) emit(t tokenType) {
	l.tokens <- token{
		typ: t,
		pos: l.start,
		val: l.input[l.start:l.pos],
	}
}

type stateFn func(*lexer) stateFn

// lexNone scans for any valid token, but will never emit a token
func lexNone(l *lexer) stateFn {
	c := l.peek()
	switch {
	case c == eof:
		/* TODO */
		return nil
	case c == '"':
		return lexString
	case isNumber(c):
		return lexNumber
	case isSymbolStartValid(c):
		return lexSymbol
	default:
		return lexPunctuation
	}
}

// parse a string starting with a quote at the current position
func lexString(l *lexer) stateFn {
	l.next()
	for {
		switch l.next() {
		case '\\':
			if c := l.next(); c != eof {
				// ignore character following backslash
				break
			}
			fallthrough
		case eof:
			return l.error("unterminated string")
		case '"':
			l.emit(tokenString)
			return lexNone
		}
	}
}

// TODO: array subscripts
func lexSymbol(l *lexer) stateFn {
	for {
		switch c := l.next(); {
		case isSymbolValid(c):
			continue
		case c == '-':
			if l.peek() == '>' {
				l.next()
				continue
			}
			fallthrough
		default:
			l.backup()
			l.emit(tokenSymbol)
			return lexNone
		}
	}
}

func lexNumber(l *lexer) stateFn {
	for {
		switch c := l.peek(); {
		case isSymbolValid(c):
			l.next()
			continue
		default:
			l.emit(tokenNumber)
			return lexNone
		}
	}
}

func lexPunctuation(l *lexer) stateFn {
	s := string(l.next())
	t, _ := stringToToken[s]
	for {
		ns := s + string(l.peek())
		nt, ok := stringToToken[ns]
		if !ok {
			break
		}
		t = nt
		s = ns
		l.next()
	}

	if t == tokenNone {
		return l.error("unknown token '" + s + "'")
	}

	l.emit(t)
	return lexNone
}

func (l *lexer) error(e string) stateFn {
	l.tokens <- token{
		typ: tokenError,
		pos: l.pos,
		val: fmt.Sprintf("error %s at %d\n", e, l.pos),
	}
	return nil
}

func (l *lexer) expectTokens(tokens []tokenType) (ret []tokenType, ok bool) {
	ok = true
	for t := range l.tokens {
		ret = append(ret, t.typ)
		if len(tokens) == 0 {
			ok = false
		} else {
			if tokens[0] != t.typ {
				ok = false
			}
			tokens = tokens[1:]
		}
	}
	return
}

func isSpace(c ascii) bool {
	return unicode.IsSpace(rune(c))
}

func isAlpha(c ascii) bool {
	return unicode.IsLetter(rune(c))
}

func isNumber(c ascii) bool {
	return unicode.IsNumber(rune(c))
}

func isSymbolStartValid(c ascii) bool {
	return isAlpha(c) || c == '_'
}

func isSymbolValid(c ascii) bool {
	return isSymbolStartValid(c) || isNumber(c)
}
