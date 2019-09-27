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

func TestScalarValue(t *testing.T) {
	tests := []struct {
		val      ScalarValue
		want     string
		wantNull bool
	}{
		{
			val:      ScalarValue{Type: NullScalar, Raw: "null"},
			wantNull: true,
		},
		{
			val:  ScalarValue{Type: BooleanScalar, Raw: "false"},
			want: "false",
		},
		{
			val:  ScalarValue{Type: BooleanScalar, Raw: "true"},
			want: "true",
		},
		{
			val:  ScalarValue{Type: IntScalar, Raw: "123"},
			want: "123",
		},
		{
			val:  ScalarValue{Type: StringScalar, Raw: `""`},
			want: "",
		},
		{
			val:  ScalarValue{Type: StringScalar, Raw: `"abcdef"`},
			want: "abcdef",
		},
		{
			val:  ScalarValue{Type: StringScalar, Raw: `"ab\u1234cd"`},
			want: "ab\u1234cd",
		},
		{
			val:  ScalarValue{Type: StringScalar, Raw: `"\tHello\"\\\r\n\b\f"`},
			want: "\tHello\"\\\r\n\b\f",
		},
		{
			val:  ScalarValue{Type: StringScalar, Raw: `""""""`},
			want: "",
		},
		{
			val:  ScalarValue{Type: StringScalar, Raw: `"""Hello World!"""`},
			want: "Hello World!",
		},
		{
			val:  ScalarValue{Type: StringScalar, Raw: `"""    """`},
			want: "",
		},
		{
			val:  ScalarValue{Type: StringScalar, Raw: `"""foo\"""bar"""`},
			want: `foo"""bar`,
		},
		{
			val: ScalarValue{Type: StringScalar, Raw: `"""
				Hello
					,
				World
			"""`},
			want: "Hello\n\t,\nWorld",
		},
		{
			val:  ScalarValue{Type: StringScalar, Raw: "\"\"\"\r\n\tHello\r\tWorld\"\"\""},
			want: "Hello\nWorld",
		},
	}
	for _, test := range tests {
		got, ok := test.val.Value()
		if got != test.want || ok != !test.wantNull {
			t.Errorf("ScalarValue{Type: %v, Raw: %q}.Value() = %q, %t; want %q, %t", test.val.Type, test.val.Raw, got, ok, test.want, !test.wantNull)
		}
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		s    string
		want []string
	}{
		{"", []string{}},
		{"foo", []string{"foo"}},
		{"foo\n", []string{"foo"}},
		{"foo\nbar", []string{"foo", "bar"}},
		{"foo\nbar\n", []string{"foo", "bar"}},
		{"foo\r\nbar\r\n", []string{"foo", "bar"}},
		{"foo\rbar\r", []string{"foo", "bar"}},
	}
	for _, test := range tests {
		got := splitLines(test.s)
		if diff := cmp.Diff(test.want, got, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("splitLines(%q) (-want +got):\n%s", test.s, diff)
		}
	}
}
