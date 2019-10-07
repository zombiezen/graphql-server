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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/xerrors"
)

func TestExecute(t *testing.T) {
	const schemaSource = `
		type Query {
			myString: String
			myBoolean: Boolean
			myInt: Int
			myInt32: Int

			niladicNoArgsMethod: String
			niladicContextOnlyMethod: String
			niladicArgsOnlyMethod: String
			niladicContextAndArgsMethod: String

			noArgsMethod(echo: String): String
			contextOnlyMethod(echo: String): String
			argsOnlyMethod(echo: String): String
			contextAndArgsMethod(echo: String): String
			argWithDefault(echo: String = "xyzzy"): String
			requiredArg(echo: String!): String!
			requiredArgWithDefault(echo: String! = "xyzzy"): String!

			nilErrorMethod: String
			errorMethod: String

			listArgument(truths: [Boolean]): String
			inputObjectArgument(complex: Complex): String
		}

		input Complex {
			foo: String
		}
	`
	tests := []struct {
		name        string
		queryObject func(e errorfer) interface{}
		request     Request
		want        []fieldExpectations
		wantErrors  []*ResponseError
	}{
		{
			name: "String/Empty",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyString: newString("")}
			},
			request: Request{Query: `{ myString }`},
			want: []fieldExpectations{
				{key: "myString", value: valueExpectations{scalar: ""}},
			},
		},
		{
			name: "String/Nonempty",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyString: newString("foo")}
			},
			request: Request{Query: `{ myString }`},
			want: []fieldExpectations{
				{key: "myString", value: valueExpectations{scalar: "foo"}},
			},
		},
		{
			name: "String/Null",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyString: nil}
			},
			request: Request{Query: `{ myString }`},
			want: []fieldExpectations{
				{key: "myString", value: valueExpectations{null: true}},
			},
		},
		{
			name: "Boolean/True",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyBoolean: newBool(true)}
			},
			request: Request{Query: `{ myBoolean }`},
			want: []fieldExpectations{
				{key: "myBoolean", value: valueExpectations{scalar: "true"}},
			},
		},
		{
			name: "Boolean/False",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyBoolean: newBool(false)}
			},
			request: Request{Query: `{ myBoolean }`},
			want: []fieldExpectations{
				{key: "myBoolean", value: valueExpectations{scalar: "false"}},
			},
		},
		{
			name: "Boolean/Null",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyBoolean: nil}
			},
			request: Request{Query: `{ myBoolean }`},
			want: []fieldExpectations{
				{key: "myBoolean", value: valueExpectations{null: true}},
			},
		},
		{
			name: "Integer/Int32/Zero",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt32: newInt32(0)}
			},
			request: Request{Query: `{ myInt32 }`},
			want: []fieldExpectations{
				{key: "myInt32", value: valueExpectations{scalar: "0"}},
			},
		},
		{
			name: "Integer/Int32/Positive",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt32: newInt32(123)}
			},
			request: Request{Query: `{ myInt32 }`},
			want: []fieldExpectations{
				{key: "myInt32", value: valueExpectations{scalar: "123"}},
			},
		},
		{
			name: "Integer/Int32/Negative",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt32: newInt32(-123)}
			},
			request: Request{Query: `{ myInt32 }`},
			want: []fieldExpectations{
				{key: "myInt32", value: valueExpectations{scalar: "-123"}},
			},
		},
		{
			name: "Integer/Int32/Null",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt32: nil}
			},
			request: Request{Query: `{ myInt32 }`},
			want: []fieldExpectations{
				{key: "myInt32", value: valueExpectations{null: true}},
			},
		},
		{
			name: "Integer/Int/Zero",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt: newInt(0)}
			},
			request: Request{Query: `{ myInt }`},
			want: []fieldExpectations{
				{key: "myInt", value: valueExpectations{scalar: "0"}},
			},
		},
		{
			name: "Integer/Int/Positive",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt: newInt(123)}
			},
			request: Request{Query: `{ myInt }`},
			want: []fieldExpectations{
				{key: "myInt", value: valueExpectations{scalar: "123"}},
			},
		},
		{
			name: "Integer/Int/Negative",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt: newInt(-123)}
			},
			request: Request{Query: `{ myInt }`},
			want: []fieldExpectations{
				{key: "myInt", value: valueExpectations{scalar: "-123"}},
			},
		},
		{
			name: "Integer/Int/Null",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt: nil}
			},
			request: Request{Query: `{ myInt }`},
			want: []fieldExpectations{
				{key: "myInt", value: valueExpectations{null: true}},
			},
		},
		{
			name: "Object/MultipleStructFields",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{
					MyInt:    newInt(42),
					MyString: newString("hello"),
				}
			},
			request: Request{Query: `{
				myInt
				myString
			}`},
			want: []fieldExpectations{
				{key: "myInt", value: valueExpectations{scalar: "42"}},
				{key: "myString", value: valueExpectations{scalar: "hello"}},
			},
		},
		{
			name: "Object/NiladicMethod/NoArgs",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{Query: `{ niladicNoArgsMethod }`},
			want: []fieldExpectations{
				{key: "niladicNoArgsMethod", value: valueExpectations{scalar: "xyzzy"}},
			},
		},
		{
			name: "Object/NiladicMethod/ContextOnly",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{Query: `{ niladicContextOnlyMethod }`},
			want: []fieldExpectations{
				{key: "niladicContextOnlyMethod", value: valueExpectations{scalar: "xyzzy"}},
			},
		},
		{
			name: "Object/NiladicMethod/ArgsOnly",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{Query: `{ niladicArgsOnlyMethod }`},
			want: []fieldExpectations{
				{key: "niladicArgsOnlyMethod", value: valueExpectations{scalar: "xyzzy"}},
			},
		},
		{
			name: "Object/NiladicMethod/ContextAndArgs",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{Query: `{ niladicContextAndArgsMethod }`},
			want: []fieldExpectations{
				{key: "niladicContextAndArgsMethod", value: valueExpectations{scalar: "xyzzy"}},
			},
		},
		{
			name: "Object/Method/NoArgs",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{Query: `{ noArgsMethod }`},
			want: []fieldExpectations{
				{key: "noArgsMethod", value: valueExpectations{scalar: "xyzzy"}},
			},
		},
		{
			name: "Object/Method/ContextOnly",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{Query: `{ contextOnlyMethod }`},
			want: []fieldExpectations{
				{key: "contextOnlyMethod", value: valueExpectations{scalar: "xyzzy"}},
			},
		},
		{
			name: "Object/Method/ArgsOnly/Null",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{Query: `{ argsOnlyMethod }`},
			want: []fieldExpectations{
				{key: "argsOnlyMethod", value: valueExpectations{scalar: "xyzzy"}},
			},
		},
		{
			name: "Object/Method/ArgsOnly/Value",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{Query: `{ argsOnlyMethod(echo: "foo") }`},
			want: []fieldExpectations{
				{key: "argsOnlyMethod", value: valueExpectations{scalar: "fooxyzzy"}},
			},
		},
		{
			name: "Object/Method/ArgsOnly/Variable",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{
				Query: `query($stringArg: String) {
					argsOnlyMethod(echo: $stringArg)
				}`,
				Variables: map[string]Input{
					"stringArg": ScalarInput("foo"),
				},
			},
			want: []fieldExpectations{
				{key: "argsOnlyMethod", value: valueExpectations{scalar: "fooxyzzy"}},
			},
		},
		{
			name: "Object/Method/ArgsOnly/VariableWithDefault/Unspecified",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{
				Query: `query($stringArg: String = "foo") {
					argsOnlyMethod(echo: $stringArg)
				}`,
				Variables: map[string]Input{},
			},
			want: []fieldExpectations{
				{key: "argsOnlyMethod", value: valueExpectations{scalar: "fooxyzzy"}},
			},
		},
		{
			name: "Object/Method/ArgsOnly/VariableWithDefault/Overridden",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{
				Query: `query($stringArg: String = "foo") {
					argsOnlyMethod(echo: $stringArg)
				}`,
				Variables: map[string]Input{
					"stringArg": ScalarInput("bar"),
				},
			},
			want: []fieldExpectations{
				{key: "argsOnlyMethod", value: valueExpectations{scalar: "barxyzzy"}},
			},
		},
		{
			name: "Object/Method/ArgsOnly/VariableWithDefault/Null",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{
				Query: `query($stringArg: String = "foo") {
					argsOnlyMethod(echo: $stringArg)
				}`,
				Variables: map[string]Input{
					"stringArg": {},
				},
			},
			want: []fieldExpectations{
				{key: "argsOnlyMethod", value: valueExpectations{scalar: "xyzzy"}},
			},
		},
		{
			name: "Object/Method/ContextAndArgs/Null",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{Query: `{ contextAndArgsMethod }`},
			want: []fieldExpectations{
				{key: "contextAndArgsMethod", value: valueExpectations{scalar: "xyzzy"}},
			},
		},
		{
			name: "Object/Method/ContextAndArgs/Value",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{Query: `{ contextAndArgsMethod(echo: "foo") }`},
			want: []fieldExpectations{
				{key: "contextAndArgsMethod", value: valueExpectations{scalar: "fooxyzzy"}},
			},
		},
		{
			name: "Object/Method/Error",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{Query: `{ errorMethod }`},
			want: []fieldExpectations{
				{key: "errorMethod", value: valueExpectations{null: true}},
			},
			wantErrors: []*ResponseError{
				{
					Locations: []Location{{1, 3}},
					Path:      []PathSegment{{Field: "errorMethod"}},
				},
			},
		},
		{
			name: "Object/Method/Error/Alias",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{Query: `{ myAlias: errorMethod }`},
			want: []fieldExpectations{
				{key: "myAlias", value: valueExpectations{null: true}},
			},
			wantErrors: []*ResponseError{
				{
					Locations: []Location{{1, 3}},
					Path:      []PathSegment{{Field: "myAlias"}},
				},
			},
		},
		{
			name: "Object/Method/Error/PartialObject",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt: newInt(42)}
			},
			request: Request{Query: `{
				error1: errorMethod
				myInt
				error2: errorMethod
			}`},
			want: []fieldExpectations{
				{key: "error1", value: valueExpectations{null: true}},
				{key: "myInt", value: valueExpectations{scalar: "42"}},
				{key: "error2", value: valueExpectations{null: true}},
			},
			wantErrors: []*ResponseError{
				{
					Locations: []Location{{2, 33}},
					Path:      []PathSegment{{Field: "error1"}},
				},
				{
					Locations: []Location{{4, 33}},
					Path:      []PathSegment{{Field: "error2"}},
				},
			},
		},
		{
			name: "Object/Method/Error/Nil",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{Query: `{ nilErrorMethod }`},
			want: []fieldExpectations{
				{key: "nilErrorMethod", value: valueExpectations{scalar: "xyzzy"}},
			},
		},
		{
			name: "Object/Alias",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt32: newInt32(42)}
			},
			request: Request{Query: `{ magic: myInt32, myInt: myInt32 }`},
			want: []fieldExpectations{
				{key: "magic", value: valueExpectations{scalar: "42"}},
				{key: "myInt", value: valueExpectations{scalar: "42"}},
			},
		},
		{
			name: "Object/ArgumentWithDefault/Omitted",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{Query: `{ argWithDefault }`},
			want: []fieldExpectations{
				{key: "argWithDefault", value: valueExpectations{scalar: "xyzzyxyzzy"}},
			},
		},
		{
			name: "Object/ArgumentWithDefault/Specified",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{Query: `{ argWithDefault(echo: "foo") }`},
			want: []fieldExpectations{
				{key: "argWithDefault", value: valueExpectations{scalar: "foofoo"}},
			},
		},
		{
			name: "Object/ArgumentWithDefault/Null",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{Query: `{ argWithDefault(echo: null) }`},
			want: []fieldExpectations{
				{key: "argWithDefault", value: valueExpectations{scalar: ""}},
			},
		},
		{
			name: "Object/ArgumentWithDefault/UnspecifiedVariable",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{
				Query: `query ($stringArg: String) {
					argWithDefault(echo: $stringArg)
				}`,
				Variables: map[string]Input{},
			},
			want: []fieldExpectations{
				{key: "argWithDefault", value: valueExpectations{scalar: "xyzzyxyzzy"}},
			},
		},
		{
			name: "Object/RequiredArgument/VariableDefault",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{
				Query: `query($stringArg: String = "bork") {
					requiredArg(echo: $stringArg)
				}`,
				Variables: map[string]Input{},
			},
			want: []fieldExpectations{
				{key: "requiredArg", value: valueExpectations{scalar: "borkbork"}},
			},
		},
		{
			name: "Object/RequiredArgument/VariableNull",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{
				Query: `query($stringArg: String = "bork") {
					requiredArg(echo: $stringArg)
				}`,
				Variables: map[string]Input{"stringArg": {}},
			},
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{2, 59},
					},
					Path: []PathSegment{
						{Field: "requiredArg"},
					},
				},
			},
		},
		{
			name: "Object/RequiredArgumentWithDefault/Variable",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{
				Query: `query ($stringArg: String) {
					requiredArgWithDefault(echo: $stringArg)
				}`,
				Variables: map[string]Input{"stringArg": ScalarInput("foo")},
			},
			want: []fieldExpectations{
				{key: "requiredArgWithDefault", value: valueExpectations{scalar: "foofoo"}},
			},
		},
		{
			name: "Object/RequiredArgumentWithDefault/Unspecified",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{Query: `{ requiredArgWithDefault }`},
			want: []fieldExpectations{
				{key: "requiredArgWithDefault", value: valueExpectations{scalar: "xyzzyxyzzy"}},
			},
		},
		{
			name: "Object/RequiredArgumentWithDefault/UnspecifiedVariable",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{
				Query: `query ($stringArg: String) {
					requiredArgWithDefault(echo: $stringArg)
				}`,
				Variables: map[string]Input{},
			},
			want: []fieldExpectations{
				{key: "requiredArgWithDefault", value: valueExpectations{scalar: "xyzzyxyzzy"}},
			},
		},
		{
			name: "Object/RequiredArgumentWithDefault/NullVariable",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			request: Request{
				Query: `query ($stringArg: String) {
					requiredArgWithDefault(echo: $stringArg)
				}`,
				Variables: map[string]Input{"stringArg": Input{}},
			},
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{2, 70},
					},
					Path: []PathSegment{
						{Field: "requiredArgWithDefault"},
					},
				},
			},
		},
		{
			name: "Object/ListArgument",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{
				Query: `{
					listArgument(truths: [true, false, true])
				}`,
			},
			want: []fieldExpectations{
				{key: "listArgument", value: valueExpectations{scalar: "101"}},
			},
		},
		{
			name: "Object/ListArgument/Null",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{
				Query: `{
					listArgument(truths: null)
				}`,
			},
			want: []fieldExpectations{
				{key: "listArgument", value: valueExpectations{scalar: ""}},
			},
		},
		{
			name: "Object/ListArgument/Scalar",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{
				Query: `{
					listArgument(truths: true)
				}`,
			},
			want: []fieldExpectations{
				{key: "listArgument", value: valueExpectations{scalar: "1"}},
			},
		},
		{
			name: "Object/ListArgument/Variable",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{
				Query: `query($myList: [Boolean]) {
					listArgument(truths: $myList)
				}`,
				Variables: map[string]Input{
					"myList": ListInput([]Input{
						ScalarInput("true"),
						ScalarInput("false"),
						ScalarInput("true"),
					}),
				},
			},
			want: []fieldExpectations{
				{key: "listArgument", value: valueExpectations{scalar: "101"}},
			},
		},
		{
			name: "Object/ListArgument/Variable/Null",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{
				Query: `query($myList: [Boolean]) {
					listArgument(truths: $myList)
				}`,
				Variables: map[string]Input{
					"myList": {},
				},
			},
			want: []fieldExpectations{
				{key: "listArgument", value: valueExpectations{scalar: ""}},
			},
		},
		{
			name: "Object/ListArgument/Variable/Scalar",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{
				Query: `query($myList: [Boolean]) {
					listArgument(truths: $myList)
				}`,
				Variables: map[string]Input{
					"myList": ScalarInput("true"),
				},
			},
			want: []fieldExpectations{
				{key: "listArgument", value: valueExpectations{scalar: "1"}},
			},
		},
		{
			name: "Object/InputObjectArgument",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{Query: `{ inputObjectArgument(complex: { foo: "xyzzy" }) }`},
			want: []fieldExpectations{
				{key: "inputObjectArgument", value: valueExpectations{scalar: "xyzzy"}},
			},
		},
		{
			name: "Object/InputObjectArgument/Null",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{Query: `{ inputObjectArgument(complex: null) }`},
			want: []fieldExpectations{
				{key: "inputObjectArgument", value: valueExpectations{scalar: "<null input>"}},
			},
		},
	}

	ctx := context.Background()
	schema, err := ParseSchema(schemaSource)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			srv, err := NewServer(schema, test.queryObject(t), nil)
			if err != nil {
				t.Fatal(err)
			}
			resp := srv.Execute(ctx, test.request)
			for _, e := range resp.Errors {
				t.Logf("Error: %s", e.Message)
			}
			if diff := compareErrors(test.wantErrors, resp.Errors); diff != "" {
				t.Errorf("errors (-want +got):\n%s", diff)
			}
			if len(test.want) == 0 && resp.Data.IsNull() {
				return
			}
			expect := &valueExpectations{object: test.want}
			expect.check(t, resp.Data)
		})
	}
}

type testQueryStruct struct {
	MyString  *string
	MyBoolean *bool
	MyInt     *int
	MyInt32   *int32

	e errorfer
}

func (q *testQueryStruct) NiladicNoArgsMethod() string {
	return "xyzzy"
}

func (q *testQueryStruct) NiladicContextOnlyMethod(ctx context.Context) string {
	if ctx == nil {
		q.e.Errorf("Foo received nil Context")
	}
	return "xyzzy"
}

func (q *testQueryStruct) NiladicArgsOnlyMethod(args map[string]Value) string {
	if len(args) > 0 {
		q.e.Errorf("Foo received non-empty args: %v", args)
	}
	return "xyzzy"
}

func (q *testQueryStruct) NiladicContextAndArgsMethod(ctx context.Context, args map[string]Value) string {
	if ctx == nil {
		q.e.Errorf("Foo received nil Context")
	}
	if len(args) > 0 {
		q.e.Errorf("Foo received non-empty args: %v", args)
	}
	return "xyzzy"
}

func (q *testQueryStruct) NoArgsMethod() string {
	return "xyzzy"
}

func (q *testQueryStruct) ContextOnlyMethod(ctx context.Context) string {
	if ctx == nil {
		q.e.Errorf("Foo received nil Context")
	}
	return "xyzzy"
}

func (q *testQueryStruct) ArgsOnlyMethod(args map[string]Value) string {
	if len(args) != 1 {
		q.e.Errorf("Foo received args: %v", args)
	} else {
		for key := range args {
			if key != "echo" {
				q.e.Errorf("Foo received args: %v", args)
			}
		}
	}
	return args["echo"].Scalar() + "xyzzy"
}

func (q *testQueryStruct) ContextAndArgsMethod(ctx context.Context, args map[string]Value) string {
	if ctx == nil {
		q.e.Errorf("Foo received nil Context")
	}
	if len(args) != 1 {
		q.e.Errorf("Foo received args: %v", args)
	} else {
		for key := range args {
			if key != "echo" {
				q.e.Errorf("Foo received args: %v", args)
			}
		}
	}
	return args["echo"].Scalar() + "xyzzy"
}

func (q *testQueryStruct) ArgWithDefault(args map[string]Value) string {
	echo := args["echo"].Scalar()
	return echo + echo
}

func (q *testQueryStruct) RequiredArg(args map[string]Value) string {
	if args["echo"].IsNull() {
		q.e.Errorf("echo is null")
	}
	echo := args["echo"].Scalar()
	return echo + echo
}

func (q *testQueryStruct) RequiredArgWithDefault(args map[string]Value) string {
	if args["echo"].IsNull() {
		q.e.Errorf("echo is null")
	}
	echo := args["echo"].Scalar()
	return echo + echo
}

func (q *testQueryStruct) NilErrorMethod() (string, error) {
	return "xyzzy", nil
}

func (q *testQueryStruct) ErrorMethod() (string, error) {
	return "xyzzy", xerrors.New("I have failed")
}

func (q *testQueryStruct) ListArgument(args map[string]Value) string {
	truths := args["truths"]
	sb := new(strings.Builder)
	for i := 0; i < truths.Len(); i++ {
		t := truths.At(i)
		if t.IsNull() {
			sb.WriteByte('N')
		} else if t.Boolean() {
			sb.WriteByte('1')
		} else {
			sb.WriteByte('0')
		}
	}
	return sb.String()
}

func (q *testQueryStruct) InputObjectArgument(args map[string]Value) string {
	complex := args["complex"]
	if complex.IsNull() {
		return "<null input>"
	}
	return complex.ValueFor("foo").Scalar()
}

func newString(s string) *string { return &s }
func newBool(b bool) *bool       { return &b }
func newInt(i int) *int          { return &i }
func newInt32(i int32) *int32    { return &i }

type valueExpectations struct {
	null   bool
	scalar string
	object []fieldExpectations
}

type fieldExpectations struct {
	key   string
	value valueExpectations
}

func (expect *valueExpectations) check(e errorfer, v Value) {
	if gotNull := v.IsNull(); gotNull != expect.null {
		e.Errorf("v.IsNull() = %t; want %t", gotNull, expect.null)
	}
	if gotScalar := v.Scalar(); gotScalar != expect.scalar {
		e.Errorf("v.Scalar() = %q; want %q", gotScalar, expect.scalar)
	}
	if len(expect.object) > 0 {
		if v.IsNull() {
			return
		}
		if v.NumFields() != len(expect.object) {
			var gotKeys, wantKeys []string
			for i := 0; i < v.NumFields(); i++ {
				gotKeys = append(gotKeys, v.Field(i).Key)
			}
			for _, f := range expect.object {
				wantKeys = append(wantKeys, f.key)
			}
			diff := cmp.Diff(wantKeys, gotKeys,
				cmpopts.SortSlices(func(a, b string) bool { return a < b }))
			e.Errorf("v fields (-want +got):\n%s", diff)
			return
		}
		for i, wantField := range expect.object {
			gotField := v.Field(i)
			if gotField.Key != wantField.key {
				e.Errorf("fields[%d].key = %q; want %q", i, gotField.Key, wantField.key)
			}
			// TODO(maybe): Prepend info about which field failed on error.
			wantField.value.check(e, gotField.Value)
		}
	}
}

type errorfer interface {
	Errorf(format string, arguments ...interface{})
}
