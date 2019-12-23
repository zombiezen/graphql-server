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
			value: testObjectValue().ValueFor("myInt"),
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

func testStringValue(s string) Value {
	schema, err := ParseSchema(`
		type Query {
			myString: String!
		}
	`, nil)
	if err != nil {
		panic(err)
	}
	queryObject := &testQueryStruct{
		MyString: NullString{S: s, Valid: true},
	}
	srv, err := NewServer(schema, queryObject, nil)
	if err != nil {
		panic(err)
	}
	response := srv.Execute(context.Background(), Request{
		Query: `{ myString }`,
	})
	if len(response.Errors) > 0 {
		panic(response.Errors[0])
	}
	return response.Data.ValueFor("myString")
}

func testStringListValue(list []string) Value {
	type query struct {
		MyStringList []string
	}

	schema, err := ParseSchema(`
		type Query {
			myStringList: [String!]!
		}
	`, nil)
	if err != nil {
		panic(err)
	}
	srv, err := NewServer(schema, &query{list}, nil)
	if err != nil {
		panic(err)
	}
	response := srv.Execute(context.Background(), Request{
		Query: `{ myStringList }`,
	})
	if len(response.Errors) > 0 {
		panic(response.Errors[0])
	}
	return response.Data.ValueFor("myStringList")
}

func testInputObjectValue(input Input) Value {
	schema, err := ParseSchema(`
		type Query {
			myString: String!
		}

		type Mutation {
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
	queryObject := &testQueryStruct{
		MyString: NullString{S: "foo", Valid: true},
	}
	mutationObject := new(testInputObjectCapture)
	srv, err := NewServer(schema, queryObject, mutationObject)
	if err != nil {
		panic(err)
	}
	response := srv.Execute(context.Background(), Request{
		Query: `mutation ($obj: Input!) { capture(obj: $obj) }`,
		Variables: map[string]Input{
			"obj": input,
		},
	})
	if len(response.Errors) > 0 {
		panic(response.Errors[0])
	}
	return mutationObject.obj
}

type testInputObjectCapture struct {
	obj Value
}

func (capture *testInputObjectCapture) Capture(args map[string]Value) bool {
	capture.obj = args["obj"]
	return true
}
