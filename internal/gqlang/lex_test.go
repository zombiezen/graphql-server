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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestLex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []token
	}{
		{
			name:  "Empty",
			input: "",
			want:  []token{},
		},
		{
			name:  "AllIgnored",
			input: "\ufeff, ,\r\n\t# foo bar baz\n",
			want:  []token{},
		},
		{
			name:  "JustComment",
			input: "# foo",
			want:  []token{},
		},
		{
			name:  "HelloWorld",
			input: " hello, \t world!  \n",
			want: []token{
				{kind: name, source: "hello", start: 1},
				{kind: name, source: "world", start: 10},
				{kind: nonNull, source: "!", start: 15},
			},
		},
		{
			name:  "String",
			input: `foo"bar"baz`,
			want: []token{
				{kind: name, source: "foo", start: 0},
				{kind: stringValue, source: `"bar"`, start: 3},
				{kind: name, source: "baz", start: 8},
			},
		},
		{
			name:  "EmptyString",
			input: `""`,
			want: []token{
				{kind: stringValue, source: `""`, start: 0},
			},
		},
		{
			name:  "StringWithEscape",
			input: `"foo\"bar"`,
			want: []token{
				{kind: stringValue, source: `"foo\"bar"`, start: 0},
			},
		},
		{
			name:  "UnterminatedString",
			input: `"foo`,
			want: []token{
				{kind: stringValue, source: `"foo`, start: 0},
			},
		},
		{
			name: "StringTerminatedWithNewline",
			input: `"foo
bar"`,
			want: []token{
				{kind: stringValue, source: `"foo`, start: 0},
				{kind: name, source: "bar", start: 5},
				{kind: stringValue, source: `"`, start: 8},
			},
		},
		{
			name: "BlockString",
			input: `"""
foo"bar"baz
"""`,
			want: []token{
				{kind: stringValue, source: `"""
foo"bar"baz
"""`, start: 0},
			},
		},
		{
			name:  "EmptyBlockString",
			input: `""""""`,
			want: []token{
				{kind: stringValue, source: `""""""`, start: 0},
			},
		},
		{
			name: "BlockStringWithEscape",
			input: `"""
foo\"""bar
"""`,
			want: []token{
				{kind: stringValue, source: `"""
foo\"""bar
"""`, start: 0},
			},
		},
		{
			name:  "UnterminatedBlockString",
			input: `"""foo`,
			want: []token{
				{kind: stringValue, source: `"""foo`, start: 0},
			},
		},
		{
			name:  "PositiveInteger",
			input: "42",
			want: []token{
				{kind: intValue, source: "42", start: 0},
			},
		},
		{
			name:  "PositiveIntegerFollowedByName",
			input: "42abc",
			want: []token{
				{kind: intValue, source: "42", start: 0},
				{kind: name, source: "abc", start: 2},
			},
		},
		{
			name:  "NegativeInteger",
			input: "-42",
			want: []token{
				{kind: intValue, source: "-42", start: 0},
			},
		},
		{
			name:  "MinusSign",
			input: "-",
			want: []token{
				{kind: unknown, source: "-", start: 0},
			},
		},
		{
			name:  "MinusSignWithName",
			input: "-abc",
			want: []token{
				{kind: unknown, source: "-", start: 0},
				{kind: name, source: "abc", start: 1},
			},
		},
		{
			name:  "Zero",
			input: "0",
			want: []token{
				{kind: intValue, source: "0", start: 0},
			},
		},
		{
			name:  "TwoZeroes",
			input: "00",
			want: []token{
				{kind: intValue, source: "0", start: 0},
				{kind: intValue, source: "0", start: 1},
			},
		},
		{
			name:  "NegativeZero",
			input: "-0",
			want: []token{
				{kind: intValue, source: "-0", start: 0},
			},
		},
		{
			name:  "ZeroOne",
			input: "01",
			want: []token{
				{kind: intValue, source: "0", start: 0},
				{kind: intValue, source: "1", start: 1},
			},
		},
		{
			name:  "IntegerWithE",
			input: "42e",
			want: []token{
				{kind: intValue, source: "42", start: 0},
				{kind: name, source: "e", start: 2},
			},
		},
		{
			name:  "IntegerWithEB",
			input: "42eb",
			want: []token{
				{kind: intValue, source: "42", start: 0},
				{kind: name, source: "eb", start: 2},
			},
		},
		{
			name:  "FloatWithFraction",
			input: "1.0",
			want: []token{
				{kind: floatValue, source: "1.0", start: 0},
			},
		},
		{
			name:  "FloatWithExponent",
			input: "1e50",
			want: []token{
				{kind: floatValue, source: "1e50", start: 0},
			},
		},
		{
			name:  "FloatWithPositiveExponent",
			input: "1e+50",
			want: []token{
				{kind: floatValue, source: "1e+50", start: 0},
			},
		},
		{
			name:  "FloatWithNegativeExponent",
			input: "1e-50",
			want: []token{
				{kind: floatValue, source: "1e-50", start: 0},
			},
		},
		{
			name:  "FloatWithFractionAndExponent",
			input: "6.0221413e23",
			want: []token{
				{kind: floatValue, source: "6.0221413e23", start: 0},
			},
		},
		{
			name:  "FloatMissingFractionDigits",
			input: "1.",
			want: []token{
				{kind: intValue, source: "1", start: 0},
				{kind: unknown, source: ".", start: 1},
			},
		},
		{
			name:  "IntegersWithEllipsis",
			input: "1...5",
			want: []token{
				{kind: intValue, source: "1", start: 0},
				{kind: ellipsis, source: "...", start: 1},
				{kind: intValue, source: "5", start: 4},
			},
		},
		{
			name:  "BasicOperation",
			input: "query MyQuery { user(id: 4) { firstName, lastName } }\n",
			want: []token{
				{kind: name, source: "query", start: 0},
				{kind: name, source: "MyQuery", start: 6},
				{kind: lbrace, source: "{", start: 14},
				{kind: name, source: "user", start: 16},
				{kind: lparen, source: "(", start: 20},
				{kind: name, source: "id", start: 21},
				{kind: colon, source: ":", start: 23},
				{kind: intValue, source: "4", start: 25},
				{kind: rparen, source: ")", start: 26},
				{kind: lbrace, source: "{", start: 28},
				{kind: name, source: "firstName", start: 30},
				{kind: name, source: "lastName", start: 41},
				{kind: rbrace, source: "}", start: 50},
				{kind: rbrace, source: "}", start: 52},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := lex(test.input)
			diff := cmp.Diff(test.want, got,
				cmpopts.EquateEmpty(),
				cmp.AllowUnexported(token{}))
			if diff != "" {
				t.Errorf("-want +got:\n%s", diff)
			}
		})
	}
}

func TestToPosition(t *testing.T) {
	tests := []struct {
		input string
		pos   Pos
		want  Position
	}{
		{"", 0, Position{1, 1}},
		{"foo", 0, Position{1, 1}},
		{"foo", 1, Position{1, 2}},
		{"foo", 3, Position{1, 4}},
		{"foo\n", 3, Position{1, 4}},
		{"foo\nbar", 3, Position{1, 4}},
		{"foo\nbar", 4, Position{2, 1}},
		{"foo\r\nbar", 3, Position{1, 4}},
		{"foo\r\nbar", 4, Position{1, 4}},
		{"foo\r\nbar", 5, Position{2, 1}},
		{"\ufefffoo", 0, Position{1, 1}},
		{"\ufefffoo", 1, Position{1, 1}},
		{"\ufefffoo", 2, Position{1, 1}},
		{"\ufefffoo", 3, Position{1, 1}},
		{"\ufefffoo", 4, Position{1, 2}},
		{"\t", 0, Position{1, 1}},
		{"\t", 1, Position{1, 9}},
		{"f\too", 0, Position{1, 1}},
		{"f\too", 2, Position{1, 9}},
		{"1234567\t", 0, Position{1, 1}},
		{"1234567\t", 8, Position{1, 9}},
		{"12345678\t", 0, Position{1, 1}},
		{"12345678\t", 9, Position{1, 17}},
	}
	for _, test := range tests {
		if got := test.pos.ToPosition(test.input); got != test.want {
			t.Errorf("posString(%q, %d) = %v; want %v", test.input, test.pos, got, test.want)
		}
	}
}
