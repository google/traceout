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

// This file implements a list of objects that start as tokens but get
// partially replaced with expressions or placeholders as they are parsed
// intermediateList objects support sub-slices, and modifications to the
// sub-slice, including changes in length, are propagated to the parent
// slice.  Changes to the parent slice, including changes caused by another
// sub-slice, will invalidate a sub-slice and cause undefined results if used.

import (
	"strings"
)

type intermediateType int

const (
	intermediateToken intermediateType = iota
	intermediateValueExpression
	intermediatePlaceholder
)

type placeholderType int

const (
	placeholderParen placeholderType = iota
	placeholderCast
)

type intermediate struct {
	typ             intermediateType
	token           token
	Expression      Expression
	placeholderType placeholderType
}

// Slices of an intermediate list that apply operations to the backing it if it exists,
// otherwise propagate the request through their parent
// Only one slice of a list should be used at a time
type intermediateList struct {
	backing      []intermediate
	parent       *intermediateList
	offset, size int
}

func (l *intermediateList) replaceIntermediate(begin, size int, intermediate intermediate) {
	if size < 0 {
		size = l.size - begin
	}
	if begin < 0 || begin+size > l.size || size == 0 {
		panic("invalid arguments to replace")
	}

	if l.backing != nil {
		l.backing[begin] = intermediate
		l.backing = append(l.backing[:begin+1], l.backing[begin+size:]...)
	} else {
		l.parent.replaceIntermediate(l.offset+begin, size, intermediate)
	}
	l.size -= size - 1
}

func (l *intermediateList) replace(begin, size int, val Expression) {
	l.replaceIntermediate(begin, size, intermediate{
		typ:        intermediateValueExpression,
		Expression: val,
	})
}

func (l *intermediateList) replaceWithPlaceholder(begin, size int, val Expression, typ placeholderType) {
	l.replaceIntermediate(begin, size, intermediate{
		typ:             intermediatePlaceholder,
		Expression:      val,
		placeholderType: typ,
	})
}

func (l *intermediateList) get(i int) *intermediate {
	if i > l.size {
		panic("invalid argument to get")
	}
	if l.backing != nil {
		return &l.backing[i]
	} else {
		return l.parent.get(l.offset + i)
	}
}

func (l *intermediateList) len() int {
	return l.size
}

func (l *intermediateList) slice(begin, size int) *intermediateList {
	if size < 0 {
		size = l.size - begin
	}
	if begin < 0 || begin+size > l.size {
		panic("invalid arguments to slice")
	}

	return &intermediateList{
		parent: l,
		offset: begin,
		size:   size,
	}
}

type direction int

const (
	leftToRight direction = iota
	rightToLeft
)

func (l *intermediateList) findIntermediateType(begin int, typ intermediateType,
	dir direction) (index int, intermediate intermediate) {

	if l.backing != nil {
		if dir == leftToRight {
			for index = begin; index < len(l.backing); index++ {
				if l.backing[index].typ == typ {
					return index, l.backing[index]
				}
			}
		} else {
			for index = begin; index >= 0; index-- {
				if l.backing[index].typ == typ {
					return index, l.backing[index]
				}
			}
		}

		return -1, intermediate
	} else {
		index, intermediate = l.parent.findIntermediateType(begin+l.offset, typ, dir)
		if index >= 0 {
			index -= l.offset
		}
		if index >= l.size {
			index = -1
		}
		return index, intermediate
	}

}

func (l *intermediateList) findToken(begin int, typs []tokenType) (index int,
	token token) {

	index = begin
	for {
		var intermediate intermediate
		index, intermediate = l.findIntermediateType(index, intermediateToken, leftToRight)
		if index < 0 {
			return index, token
		}
		t := intermediate.token
		for _, typ := range typs {
			if t.typ == typ {
				return index, t
			}
		}
		index++
	}

	return -1, token
}

func (l *intermediateList) findTokenDir(begin int, typs []tokenType, dir direction) (index int,
	token token) {

	index = begin
	if index == -1 {
		index = l.size - 1
	}
	for {
		var intermediate intermediate
		index, intermediate = l.findIntermediateType(index, intermediateToken, dir)
		if index < 0 {
			return index, token
		}
		t := intermediate.token
		for _, typ := range typs {
			if t.typ == typ {
				return index, t
			}
		}
		if dir == leftToRight {
			index++
		} else {
			index--
		}
	}

	return -1, token
}

func (l *intermediateList) findPlaceholderDir(begin int, dir direction, typ placeholderType) (index int,
	v Expression) {

	index = begin
	if index == -1 {
		index = l.size - 1
	}
	for {
		var intermediate intermediate
		index, intermediate = l.findIntermediateType(index, intermediatePlaceholder, dir)
		if index < 0 {
			return index, nil
		}
		if intermediate.placeholderType == typ {
			return index, intermediate.Expression
		}
		if dir == leftToRight {
			index++
		} else {
			index--
		}
	}

	return -1, nil
}

func (l *intermediateList) placeholder(index int) (Expression, placeholderType) {
	if index < 0 || index >= l.size {
		return nil, -1
	}
	intermediate := l.get(index)
	if intermediate.typ != intermediatePlaceholder {
		return nil, -1
	}
	return intermediate.Expression, intermediate.placeholderType
}

func (l *intermediateList) expression(index int) Expression {
	if index < 0 || index >= l.size {
		return nil
	}
	intermediate := l.get(index)
	if intermediate.typ != intermediateValueExpression {
		return nil
	}
	return intermediate.Expression
}

func (l *intermediateList) token(index int) token {
	if index < 0 || index >= l.size {
		return nullToken
	}
	intermediate := l.get(index)
	if intermediate.typ != intermediateToken {
		return nullToken
	}
	return intermediate.token
}

func newIntermediateList(tokens []token) *intermediateList {
	l := []intermediate{}
	for _, t := range tokens {
		l = append(l, intermediate{
			typ:   intermediateToken,
			token: t,
		})
	}

	return &intermediateList{
		backing: l,
		offset:  0,
		size:    len(l),
	}
}

func (l *intermediateList) dump() string {
	d := []string{}
	for i := 0; i < l.size; i++ {
		intermediate := l.get(i)
		s := ""
		switch intermediate.typ {
		case intermediateToken:
			s = "`" + intermediate.token.val + "`"
		case intermediateValueExpression:
			s = intermediate.Expression.Dump()
		case intermediatePlaceholder:
			s = "<<<(" + intermediate.Expression.Dump() + ")>>>"
		default:
			s = "unknown"
		}
		d = append(d, s)
	}

	return strings.Join(d, " ")
}
