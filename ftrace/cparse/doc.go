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

/*
Package cparse is a lexer, parser, and evaluator for simple C expressions
intended to support the C printf expressions found in
/sys/kernel/debug/tracing/events/<subsystem>/<event>/format files.  It is
intended only to handle valid C expressions - error checking of invalid
C expressions is not a priority.

Basics

The cparse.Parse function takes a string expression and a cparse.Scope and
produces a slice of Expression objects, which can be evaulated with
Expression.Value and a cparse.Context to produce a cparse.Value.
A cparse.Value can be converted to a go type with AsInt(), AsString(), etc.,
or to an interface{} suitable to pass to printf with AsInterface().  An
Expression can be evaluated multiple times with different contexts.

Not supported (yet?):
Pointers
Arrays
*/

package cparse
