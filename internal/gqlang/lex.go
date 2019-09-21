// Copyright 2019 Ross Light
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package gqlang

import (
	"fmt"
	"strings"
)

type lexer struct {
	input string
	pos   Pos
}

func lex(input string) []token {
	l := &lexer{input: input}
	var tokens []token
	for {
		tok := l.next()
		if tok.source == "" {
			// EOF
			break
		}
		tokens = append(tokens, tok)
	}
	return tokens
}

func (l *lexer) next() token {
	l.skipIgnored()
	start := l.pos
	if len(l.input) == 0 {
		return token{start: start}
	}
	for pat, kind := range punctuators {
		if strings.HasPrefix(l.input, pat) {
			return token{
				kind:   kind,
				source: l.consume(len(pat)),
				start:  start,
			}
		}
	}
	switch c := l.input[0]; {
	case isNameChar(c):
		n := 1
		for ; n < len(l.input); n++ {
			if c := l.input[n]; !isNameChar(c) && !isDigit(c) {
				break
			}
		}
		return token{
			kind:   name,
			source: l.consume(n),
			start:  start,
		}
	case c == '"':
		if strings.HasPrefix(l.input, `"""`) {
			return l.blockString()
		}
		return l.simpleString()
	case c == '-' || isDigit(c):
		return l.number()
	default:
		return token{
			kind:   unknown,
			source: l.consume(1),
			start:  start,
		}
	}
}

// simpleString parses a quoted string.
// https://graphql.github.io/graphql-spec/June2018/#sec-String-Value
func (l *lexer) simpleString() token {
	start := l.pos
	for n := 1; n < len(l.input); n++ {
		switch l.input[n] {
		case '\\':
			n++
		case '\n':
			// Stop at end of line.
			return token{
				kind:   stringValue,
				source: l.consume(n),
				start:  start,
			}
		case '"':
			return token{
				kind:   stringValue,
				source: l.consume(n + 1),
				start:  start,
			}
		}
	}
	// Unexpected EOF.
	return token{
		kind:   stringValue,
		source: l.consume(len(l.input)),
		start:  start,
	}
}

// blockString parses a triple-quoted string.
// https://graphql.github.io/graphql-spec/June2018/#sec-String-Value
func (l *lexer) blockString() token {
	const marker = `"""`
	start := l.pos
	for i := len(marker); ; {
		j := strings.Index(l.input[i:], `"""`)
		if j == -1 {
			// Unexpected EOF.
			return token{
				kind:   stringValue,
				source: l.consume(len(l.input)),
				start:  start,
			}
		}
		if l.input[i+j-1] != '\\' {
			return token{
				kind:   stringValue,
				source: l.consume(i + j + len(marker)),
				start:  start,
			}
		}
		// Move past escape.
		i += j + len(marker)
	}
}

// number parses either an integer or a floating-point literal (both start with an integer).
// https://graphql.github.io/graphql-spec/June2018/#sec-Int-Value
// https://graphql.github.io/graphql-spec/June2018/#sec-Float-Value
func (l *lexer) number() token {
	start := l.pos
	n := 0

	// IntegerPart
	if l.input[0] == '-' {
		n++
	}
	if n >= len(l.input) || !isDigit(l.input[n]) {
		return token{
			kind:   unknown,
			source: l.consume(n),
			start:  start,
		}
	}
	n++
	if l.input[n-1] != '0' {
		for n < len(l.input) && isDigit(l.input[n]) {
			n++
		}
	}

	// ( FractionalPart ( ExponentPart ) ? | ExponentPart ) ?
	if n+1 >= len(l.input) {
		// Not enough input? Integer.
		return token{
			kind:   intValue,
			source: l.consume(n),
			start:  start,
		}
	}
	switch l.input[n] {
	case '.':
		if !isDigit(l.input[n+1]) {
			return token{
				kind:   intValue,
				source: l.consume(n),
				start:  start,
			}
		}
		n += 2
		for n < len(l.input) && isDigit(l.input[n]) {
			n++
		}
		// ExponentPart ?
		if n < len(l.input) && (l.input[n] == 'e' || l.input[n] == 'E') {
			if expEnd := l.scanExponentPart(n); expEnd != -1 {
				n = expEnd
			}
		}
		return token{
			kind:   floatValue,
			source: l.consume(n),
			start:  start,
		}
	case 'e', 'E':
		expEnd := l.scanExponentPart(n)
		if expEnd == -1 {
			return token{
				kind:   intValue,
				source: l.consume(n),
				start:  start,
			}
		}
		return token{
			kind:   floatValue,
			source: l.consume(expEnd),
			start:  start,
		}
	default:
		return token{
			kind:   intValue,
			source: l.consume(n),
			start:  start,
		}
	}
}

func (l *lexer) scanExponentPart(start int) int {
	n := 1 // skip past 'e'
	if start+n >= len(l.input) {
		return -1
	}
	if c := l.input[start+n]; c == '+' || c == '-' {
		n++
		if start+n >= len(l.input) {
			return -1
		}
	}
	if !isDigit(l.input[start+n]) {
		return -1
	}
	n++
	for start+n < len(l.input) && isDigit(l.input[start+n]) {
		n++
	}
	return start + n
}

// skipIgnored skips any ignored tokens.
// https://graphql.github.io/graphql-spec/June2018/#sec-Source-Text.Ignored-Tokens
func (l *lexer) skipIgnored() {
	for len(l.input) > 0 {
		switch l.input[0] {
		case ' ', '\t', '\r', '\n', ',':
			l.consume(1)
		case bom[0]:
			if !strings.HasPrefix(l.input, bom) {
				return
			}
			l.consume(len(bom))
		case '#':
			l.consume(1)
			i := strings.IndexAny(l.input, "\n\r")
			if i == -1 {
				// To end of input.
				l.pos += Pos(len(l.input))
				l.input = ""
				return
			}
			l.consume(i + 1)
		default:
			return
		}
	}
}

func (l *lexer) consume(n int) string {
	s := l.input[:n]
	l.input = l.input[n:]
	l.pos += Pos(n)
	return s
}

type token struct {
	kind   tokenKind
	source string
	start  Pos
}

func (tok token) String() string {
	if tok.kind == unknown {
		return "<unknown>"
	}
	return tok.source
}

func (tok token) end() Pos {
	return tok.start + Pos(len(tok.source))
}

// A Pos is a 0-based byte offset in a GraphQL document.
type Pos int

// ToPosition converts a byte position into a line and column number.
func (pos Pos) ToPosition(input string) Position {
	line, col := 1, 1
	for i := 0; i < int(pos); i++ {
		switch input[i] {
		case bom[0]:
			if !strings.HasPrefix(input[i:], bom) {
				col++
				continue
			}
			i += len(bom) - 1
		case '\r':
			if strings.HasPrefix(input[i:], "\r\n") {
				continue
			}
			fallthrough
		case '\n':
			line++
			col = 1
		case '\t':
			const tabWidth = 8
			col++
			for (col-1)%tabWidth != 0 {
				col++
			}
		default:
			col++
		}
	}
	return Position{line, col}
}

// A Position is a line/column pair. Both are 1-based.
// The column is byte-based.
type Position struct {
	Line   int
	Column int
}

// String returns p in the form "line:col".
func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

type tokenKind int

const (
	unknown tokenKind = iota

	// Punctuators
	nonNull  // '!'
	dollar   // '$'
	lparen   // '('
	rparen   // ')'
	ellipsis // '...'
	colon    // ':'
	equals   // '='
	atSign   // '@'
	lbracket // '['
	rbracket // ']'
	lbrace   // '{'
	rbrace   // '}'
	or       // '|'

	name
	intValue
	floatValue
	stringValue
)

var punctuators = map[string]tokenKind{
	"!":   nonNull,
	"$":   dollar,
	"(":   lparen,
	")":   rparen,
	"...": ellipsis,
	":":   colon,
	"=":   equals,
	"@":   atSign,
	"[":   lbracket,
	"]":   rbracket,
	"{":   lbrace,
	"}":   rbrace,
	"|":   or,
}

var punctuatorStrings = map[tokenKind]string{
	nonNull:  "!",
	dollar:   "$",
	lparen:   "(",
	rparen:   ")",
	ellipsis: "...",
	colon:    ":",
	equals:   "=",
	atSign:   "@",
	lbracket: "[",
	rbracket: "]",
	lbrace:   "{",
	rbrace:   "}",
	or:       "|",
}

func (kind tokenKind) String() string {
	switch kind {
	case unknown:
		return "unknown"
	case nonNull:
		return "nonNull"
	case dollar:
		return "dollar"
	case lparen:
		return "lparen"
	case rparen:
		return "rparen"
	case ellipsis:
		return "ellipsis"
	case colon:
		return "colon"
	case equals:
		return "equals"
	case atSign:
		return "atSign"
	case lbracket:
		return "lbracket"
	case rbracket:
		return "rbracket"
	case lbrace:
		return "lbrace"
	case rbrace:
		return "rbrace"
	case or:
		return "or"
	case name:
		return "name"
	case intValue:
		return "intValue"
	case floatValue:
		return "floatValue"
	case stringValue:
		return "stringValue"
	default:
		return fmt.Sprintf("tokenKind(%d)", int(kind))
	}
}

// isNameChar reports whether c could occur at any position in a name.
// https://graphql.github.io/graphql-spec/June2018/#Name
func isNameChar(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || c == '_'
}

func isDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

const bom = "\ufeff"
