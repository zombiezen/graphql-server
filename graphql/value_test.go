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

package graphql

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestValue_String(t *testing.T) {
	tests := []struct {
		name  string
		value Value
		want  string
	}{
		{
			name:  "Null",
			value: Value{},
			want:  "null",
		},
		{
			name:  "Int",
			value: testIntValue(42),
			want:  "42",
		},
		{
			name:  "String/Empty",
			value: testStringValue(""),
			want:  `""`,
		},
		{
			name:  "String/Printable",
			value: testStringValue("Hello, World!"),
			want:  `"Hello, World!"`,
		},
		{
			name:  "String/EscapeCharacters",
			value: testStringValue("\tHello, World!\r\n"),
			want:  `"\tHello, World!\r\n"`,
		},
		{
			name:  "String/UnicodeEscapes",
			value: testStringValue("\x00\x7f"),
			want:  `"\u0000\u007f"`,
		},
		{
			// TODO(someday): This behavior is not fully specified.
			// See https://github.com/graphql/graphql-spec/issues/214
			name:  "String/OutsideBMP",
			value: testStringValue("\U00010000"),
			want:  `"\ud800\udc00"`,
		},
		{
			name: "InputObject",
			value: testInputObjectValue(InputObject(map[string]Input{
				"aaa": ScalarInput("123"),
				"zzz": ScalarInput("456"),
			})),
			want: "{\n\taaa: 123,\n\tzzz: \"456\"\n}",
		},
		{
			name:  "List/Empty",
			value: testStringListValue(nil),
			want:  "[]",
		},
		{
			name:  "List/SingleElement",
			value: testStringListValue([]string{"foo"}),
			want:  `["foo"]`,
		},
		{
			name:  "List/MultipleElements",
			value: testStringListValue([]string{"foo", "bar"}),
			want:  `["foo", "bar"]`,
		},
		{
			name:  "Object",
			value: testObjectValue(),
			want:  "{\n\tmyInt: 42,\n\tmyString: \"xyzzy\"\n}",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.value.String()
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("value.String() (-want +got):\n%s", diff)
			}
		})
	}
}

func testValue(val interface{}, typ string) Value {
	schema, err := ParseSchema(`
		type Query {
			myValue: `+typ+`
		}

		enum MyEnum {
			FOO
			BAR
			BAZ
		}
	`, nil)
	if err != nil {
		panic(err)
	}
	resolver := fieldResolverFunc(func(_ context.Context, _ FieldRequest) (interface{}, error) {
		return val, nil
	})
	srv, err := NewServer(schema, resolver, nil)
	if err != nil {
		panic(err)
	}
	response := srv.Execute(context.Background(), Request{
		Query: `{ myValue }`,
	})
	if len(response.Errors) > 0 {
		panic(response.Errors[0])
	}
	return response.Data.ValueFor("myValue")
}

func testStringValue(s string) Value {
	return testValue(s, "String!")
}

func testIntValue(i int32) Value {
	return testValue(i, "Int!")
}

func testStringListValue(list []string) Value {
	return testValue(list, "[String!]!")
}

func testInputObjectValue(input Input) Value {
	schema, err := ParseSchema(`
		type Query {
			capture(obj: Input!): Boolean!
		}

		input Input {
			aaa: Int!
			zzz: String!
		}
	`, nil)
	if err != nil {
		panic(err)
	}
	var obj Value
	resolver := fieldResolverFunc(func(_ context.Context, req FieldRequest) (interface{}, error) {
		obj = req.Args["obj"]
		return true, nil
	})
	srv, err := NewServer(schema, resolver, nil)
	if err != nil {
		panic(err)
	}
	response := srv.Execute(context.Background(), Request{
		Query: `query ($obj: Input!) { capture(obj: $obj) }`,
		Variables: map[string]Input{
			"obj": input,
		},
	})
	if len(response.Errors) > 0 {
		panic(response.Errors[0])
	}
	return obj
}
