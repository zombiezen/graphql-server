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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/xerrors"
)

func TestExecuteQuery(t *testing.T) {
	t.Parallel()
	const schemaSource = `
		type Query {
			myString: String
			myNonNullString: String!
			myBoolean: Boolean
			myInt: Int
			myInt32: Int
			myIntId: ID
			myInt64Id: ID
			myStringId: ID
			myList: [Int!]!
			myObjectList: [Dog!]!
			myErrorList: [String]
			myNonNullErrorList: [String!]
			myDog: Dog
			myDirection: Direction

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
			enumArg(direction: Direction!): String!
			convertedArgsMethod(echo: String): String!
			convertedPointerArgsMethod(echo: String): String!

			nilErrorMethod: String
			errorMethod: String

			idArgument(id: ID): String
			listArgument(truths: [Boolean]): String
			inputObjectArgument(complex: Complex): String
		}

		type Dog {
			name: String!
			barkVolume: Int
		}

		enum Direction {
			NORTH
			SOUTH
			EAST
			WEST
		}

		input Complex {
			foo: String
		}
	`
	tests := []struct {
		name          string
		queryObject   func(e errorfer) interface{}
		request       Request
		want          []fieldExpectations
		wantInitError bool
		wantErrors    []*ResponseError
	}{
		{
			name: "Nil",
			queryObject: func(e errorfer) interface{} {
				return nil
			},
			wantInitError: true,
		},
		{
			name: "String/Empty",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyString: NullString{S: "", Valid: true}}
			},
			request: Request{Query: `{ myString }`},
			want: []fieldExpectations{
				{key: "myString", value: valueExpectations{scalar: ""}},
			},
		},
		{
			name: "String/Nonempty",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyString: NullString{S: "foo", Valid: true}}
			},
			request: Request{Query: `{ myString }`},
			want: []fieldExpectations{
				{key: "myString", value: valueExpectations{scalar: "foo"}},
			},
		},
		{
			name: "String/Null",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyString: NullString{}}
			},
			request: Request{Query: `{ myString }`},
			want: []fieldExpectations{
				{key: "myString", value: valueExpectations{null: true}},
			},
		},
		{
			name: "String/NullInNonNull",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyNonNullString: NullString{}}
			},
			request: Request{Query: `{ myNonNullString }`},
			want: []fieldExpectations{
				{key: "myNonNullString", value: valueExpectations{null: true}},
			},
			wantErrors: []*ResponseError{
				{
					Locations: []Location{{1, 3}},
					Path: []PathSegment{
						{Field: "myNonNullString"},
					},
				},
			},
		},
		{
			name: "Boolean/True",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyBoolean: NullBoolean{Bool: true, Valid: true}}
			},
			request: Request{Query: `{ myBoolean }`},
			want: []fieldExpectations{
				{key: "myBoolean", value: valueExpectations{scalar: "true"}},
			},
		},
		{
			name: "Boolean/False",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyBoolean: NullBoolean{Bool: false, Valid: true}}
			},
			request: Request{Query: `{ myBoolean }`},
			want: []fieldExpectations{
				{key: "myBoolean", value: valueExpectations{scalar: "false"}},
			},
		},
		{
			name: "Boolean/Null",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyBoolean: NullBoolean{}}
			},
			request: Request{Query: `{ myBoolean }`},
			want: []fieldExpectations{
				{key: "myBoolean", value: valueExpectations{null: true}},
			},
		},
		{
			name: "Integer/Int32/Zero",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt32: NullInt{Int: 0, Valid: true}}
			},
			request: Request{Query: `{ myInt32 }`},
			want: []fieldExpectations{
				{key: "myInt32", value: valueExpectations{scalar: "0"}},
			},
		},
		{
			name: "Integer/Int32/Positive",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt32: NullInt{Int: 123, Valid: true}}
			},
			request: Request{Query: `{ myInt32 }`},
			want: []fieldExpectations{
				{key: "myInt32", value: valueExpectations{scalar: "123"}},
			},
		},
		{
			name: "Integer/Int32/Negative",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt32: NullInt{Int: -123, Valid: true}}
			},
			request: Request{Query: `{ myInt32 }`},
			want: []fieldExpectations{
				{key: "myInt32", value: valueExpectations{scalar: "-123"}},
			},
		},
		{
			name: "Integer/Int32/Null",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt32: NullInt{}}
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
			name: "ID/Result/Int",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyIntID: NullInt{Int: 42, Valid: true}}
			},
			request: Request{Query: `{ myIntId }`},
			want: []fieldExpectations{
				{key: "myIntId", value: valueExpectations{scalar: "42"}},
			},
		},
		{
			name: "ID/Result/Int64",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt64ID: newInt64(42)}
			},
			request: Request{Query: `{ myInt64Id }`},
			want: []fieldExpectations{
				{key: "myInt64Id", value: valueExpectations{scalar: "42"}},
			},
		},
		{
			name: "ID/Result/String",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyStringID: NullString{S: "aardvark", Valid: true}}
			},
			request: Request{Query: `{ myStringId }`},
			want: []fieldExpectations{
				{key: "myStringId", value: valueExpectations{scalar: "aardvark"}},
			},
		},
		{
			name: "ID/Input/Literal/Int",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{Query: `{ idArgument(id: 123) }`},
			want: []fieldExpectations{
				{key: "idArgument", value: valueExpectations{scalar: "123"}},
			},
		},
		{
			name: "ID/Input/Literal/String",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{Query: `{ idArgument(id: "aardvark") }`},
			want: []fieldExpectations{
				{key: "idArgument", value: valueExpectations{scalar: "aardvark"}},
			},
		},
		{
			name: "ID/Input/Literal/Float",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{Query: `{ idArgument(id: 123.0) }`},
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{1, 18},
					},
					Path: []PathSegment{
						{Field: "idArgument"},
					},
				},
			},
		},
		{
			name: "ID/Input/Variable/Int",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{
				Query:     `query($id: ID!) { idArgument(id: $id) }`,
				Variables: map[string]Input{"id": ScalarInput("123")},
			},
			want: []fieldExpectations{
				{key: "idArgument", value: valueExpectations{scalar: "123"}},
			},
		},
		{
			name: "ID/Input/Variable/String",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{
				Query:     `query($id: ID!) { idArgument(id: $id) }`,
				Variables: map[string]Input{"id": ScalarInput("aardvark")},
			},
			want: []fieldExpectations{
				{key: "idArgument", value: valueExpectations{scalar: "aardvark"}},
			},
		},
		{
			name: "ID/Input/Variable/Float",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{
				Query:     `query($id: ID!) { idArgument(id: $id) }`,
				Variables: map[string]Input{"id": ScalarInput("123.0")},
			},
			want: []fieldExpectations{
				{key: "idArgument", value: valueExpectations{scalar: "123.0"}},
			},
		},
		{
			name: "Enum/Valid",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyDirection: NullString{S: "NORTH", Valid: true}}
			},
			request: Request{Query: `{ myDirection }`},
			want: []fieldExpectations{
				{key: "myDirection", value: valueExpectations{scalar: "NORTH"}},
			},
		},
		{
			name: "Enum/Invalid",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyDirection: NullString{S: "WEAST", Valid: true}}
			},
			request: Request{Query: `{ myDirection }`},
			want: []fieldExpectations{
				{key: "myDirection", value: valueExpectations{null: true}},
			},
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{1, 3},
					},
					Path: []PathSegment{
						{Field: "myDirection"},
					},
				},
			},
		},
		{
			name: "Enum/Argument/Valid",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{
				Query: `query($d: Direction!) { enumArg(direction: $d) }`,
				Variables: map[string]Input{
					"d": ScalarInput("NORTH"),
				},
			},
			want: []fieldExpectations{
				{key: "enumArg", value: valueExpectations{scalar: "NORTH"}},
			},
		},
		{
			name: "Enum/Argument/Invalid",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{
				Query: `query($d: Direction!) { enumArg(direction: $d) }`,
				Variables: map[string]Input{
					"d": ScalarInput("WEAST"),
				},
			},
			wantErrors: []*ResponseError{
				{},
			},
		},
		{
			name: "ConvertedArgsMethod/Value",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{
				Query: `{ convertedArgsMethod(echo: "ohai") }`,
			},
			want: []fieldExpectations{
				{key: "convertedArgsMethod", value: valueExpectations{scalar: "ohaiohai"}},
			},
		},
		{
			name: "ConvertedArgsMethod/Pointer",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{
				Query: `{ convertedPointerArgsMethod(echo: "ohai") }`,
			},
			want: []fieldExpectations{
				{key: "convertedPointerArgsMethod", value: valueExpectations{scalar: "ohaiohai"}},
			},
		},
		{
			name: "List/Nonempty",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyList: []int32{123, 456, 789}}
			},
			request: Request{Query: `{ myList }`},
			want: []fieldExpectations{
				{key: "myList", value: valueExpectations{list: []valueExpectations{
					{scalar: "123"},
					{scalar: "456"},
					{scalar: "789"},
				}}},
			},
		},
		{
			name: "List/Empty",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyList: []int32{}}
			},
			request: Request{Query: `{ myList }`},
			want: []fieldExpectations{
				{key: "myList", value: valueExpectations{list: []valueExpectations{}}},
			},
		},
		{
			name: "List/Nil",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyList: nil}
			},
			request: Request{Query: `{ myList }`},
			want: []fieldExpectations{
				{key: "myList", value: valueExpectations{list: []valueExpectations{}}},
			},
		},
		{
			name: "List/Objects",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyObjectList: []*testDogStruct{
					{Name: "Fido"},
					{Name: "Rover"},
				}}
			},
			request: Request{Query: `{ myObjectList { name } }`},
			want: []fieldExpectations{
				{key: "myObjectList", value: valueExpectations{list: []valueExpectations{
					{object: []fieldExpectations{
						{key: "name", value: valueExpectations{scalar: "Fido"}},
					}},
					{object: []fieldExpectations{
						{key: "name", value: valueExpectations{scalar: "Rover"}},
					}},
				}}},
			},
		},
		{
			name: "List/Errors/NullableElements",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyErrorList: []testErrorScalar{true, false, true}}
			},
			request: Request{Query: `{ myErrorList }`},
			want: []fieldExpectations{
				{key: "myErrorList", value: valueExpectations{list: []valueExpectations{
					{scalar: "ok"},
					{null: true},
					{scalar: "ok"},
				}}},
			},
			wantErrors: []*ResponseError{
				{
					Locations: []Location{{1, 3}},
					Path: []PathSegment{
						{Field: "myErrorList"},
						{ListIndex: 1},
					},
				},
			},
		},
		{
			name: "List/Errors/NonNullableElements",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyNonNullErrorList: []testErrorScalar{true, false, true}}
			},
			request: Request{Query: `{ myNonNullErrorList }`},
			want: []fieldExpectations{
				{key: "myNonNullErrorList", value: valueExpectations{null: true}},
			},
			wantErrors: []*ResponseError{
				{
					Locations: []Location{{1, 3}},
					Path: []PathSegment{
						{Field: "myNonNullErrorList"},
						{ListIndex: 1},
					},
				},
			},
		},
		{
			name: "Object/MultipleStructFields",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{
					MyInt:    newInt(42),
					MyString: NullString{S: "hello", Valid: true},
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
			name: "Object/MergeFields",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{
					MyDog: &testDogStruct{
						Name:       "Fido",
						BarkVolume: NullInt{Int: 11, Valid: true},
					},
				}
			},
			request: Request{Query: `{
				myDog { name }
				myDog { barkVolume }
			}`},
			want: []fieldExpectations{
				{key: "myDog", value: valueExpectations{object: []fieldExpectations{
					{key: "name", value: valueExpectations{scalar: "Fido"}},
					{key: "barkVolume", value: valueExpectations{scalar: "11"}},
				}}},
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
				return &testQueryStruct{MyInt32: NullInt{Int: 42, Valid: true}}
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
		{
			name: "Object/InputObjectArgument/Variable",
			queryObject: func(e errorfer) interface{} {
				return new(testQueryStruct)
			},
			request: Request{
				Query: `query($complex: Complex!) {
					inputObjectArgument(complex: $complex)
				}`,
				Variables: map[string]Input{
					"complex": InputObject(map[string]Input{
						"foo": ScalarInput("xyzzy"),
					}),
				},
			},
			want: []fieldExpectations{
				{key: "inputObjectArgument", value: valueExpectations{scalar: "xyzzy"}},
			},
		},
		{
			name: "Fragment/Inline",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{
					MyInt32:  NullInt{Int: 42, Valid: true},
					MyString: NullString{S: "Hello", Valid: true},
				}
			},
			request: Request{Query: `{ myInt32, ... on Query { myString } }`},
			want: []fieldExpectations{
				{key: "myInt32", value: valueExpectations{scalar: "42"}},
				{key: "myString", value: valueExpectations{scalar: "Hello"}},
			},
		},
		{
			name: "Fragment/Named",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{
					MyInt32:  NullInt{Int: 42, Valid: true},
					MyString: NullString{S: "Hello", Valid: true},
				}
			},
			request: Request{Query: `
				{ myInt32, ...queryString }
				fragment queryString on Query { myString }
			`},
			want: []fieldExpectations{
				{key: "myInt32", value: valueExpectations{scalar: "42"}},
				{key: "myString", value: valueExpectations{scalar: "Hello"}},
			},
		},
		{
			name: "Function/NoArgs",
			queryObject: func(e errorfer) interface{} {
				return func() *testQueryStruct {
					return &testQueryStruct{
						MyString: NullString{S: "Hello", Valid: true},
					}
				}
			},
			request: Request{Query: `{ myString }`},
			want: []fieldExpectations{
				{key: "myString", value: valueExpectations{scalar: "Hello"}},
			},
		},
		{
			name: "Function/AllArgs",
			queryObject: func(e errorfer) interface{} {
				return func(ctx context.Context, sel *SelectionSet) *testQueryStruct {
					if !sel.Has("myString") {
						t.Error("sel.Has(\"myString\") = false; want true")
					}
					return &testQueryStruct{
						MyString: NullString{S: "Hello", Valid: true},
					}
				}
			},
			request: Request{Query: `{ myString }`},
			want: []fieldExpectations{
				{key: "myString", value: valueExpectations{scalar: "Hello"}},
			},
		},
		{
			name: "Function/ErrorReturn/Nil",
			queryObject: func(e errorfer) interface{} {
				return func() (*testQueryStruct, error) {
					return &testQueryStruct{
						MyString: NullString{S: "Hello", Valid: true},
					}, nil
				}
			},
			request: Request{Query: `{ myString }`},
			want: []fieldExpectations{
				{key: "myString", value: valueExpectations{scalar: "Hello"}},
			},
		},
		{
			name: "Function/ErrorReturn/Error",
			queryObject: func(e errorfer) interface{} {
				return func() (*testQueryStruct, error) {
					return &testQueryStruct{
						MyString: NullString{S: "Hello", Valid: true},
					}, xerrors.New("fail!")
				}
			},
			request: Request{Query: `{ myString }`},
			wantErrors: []*ResponseError{
				new(ResponseError),
			},
		},
	}

	ctx := context.Background()
	schema, err := ParseSchema(schemaSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			srv, err := NewServer(schema, test.queryObject(t), nil)
			if err != nil {
				t.Logf("NewServer: %v", err)
				if !test.wantInitError {
					t.Fail()
				}
				return
			}
			if test.wantInitError {
				t.Fatal("NewServer did not return error")
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
	MyString           NullString
	MyNonNullString    NullString
	MyBoolean          NullBoolean
	MyInt              *int
	MyInt32            NullInt
	MyIntID            NullInt
	MyInt64ID          *int64
	MyStringID         NullString
	MyList             []int32
	MyObjectList       []*testDogStruct
	MyErrorList        []testErrorScalar
	MyNonNullErrorList []testErrorScalar
	MyDog              *testDogStruct
	MyDirection        NullString

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

func (q *testQueryStruct) EnumArg(args map[string]Value) string {
	return args["direction"].Scalar()
}

type convertedArgs struct {
	Echo string
}

func (q *testQueryStruct) ConvertedArgsMethod(args convertedArgs) string {
	return args.Echo + args.Echo
}

func (q *testQueryStruct) ConvertedPointerArgsMethod(args *convertedArgs) string {
	return args.Echo + args.Echo
}

func (q *testQueryStruct) NilErrorMethod() (string, error) {
	return "xyzzy", nil
}

func (q *testQueryStruct) ErrorMethod() (string, error) {
	return "xyzzy", xerrors.New("I have failed")
}

func (q *testQueryStruct) IDArgument(args map[string]Value) string {
	return args["id"].Scalar()
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

type testDogStruct struct {
	Name       string
	BarkVolume NullInt
}

type testErrorScalar bool

func (s testErrorScalar) MarshalText() ([]byte, error) {
	if !s {
		return nil, xerrors.New("flail")
	}
	return []byte("ok"), nil
}

func TestExecuteMutate(t *testing.T) {
	t.Parallel()
	const schemaSource = `
		type Query {
			foo: Boolean
		}

		type Mutation {
			structField: Boolean!
			method: Boolean!
			errorMethod: Boolean!
		}
	`
	tests := []struct {
		name           string
		mutationObject interface{}
		request        Request
		want           []fieldExpectations
		wantInitError  bool
		wantErrors     []*ResponseError
	}{
		{
			name:           "Nil",
			mutationObject: nil,
			wantInitError:  true,
		},
		{
			name: "StructField",
			mutationObject: &testMutationStruct{
				StructField: true,
			},
			request: Request{Query: `mutation { structField }`},
			want: []fieldExpectations{
				{key: "structField", value: valueExpectations{scalar: "true"}},
			},
		},
		{
			name:           "Method/Success",
			mutationObject: new(testMutationStruct),
			request:        Request{Query: `mutation { method }`},
			want: []fieldExpectations{
				{key: "method", value: valueExpectations{scalar: "true"}},
			},
		},
		{
			name:           "Method/Error",
			mutationObject: new(testMutationStruct),
			request:        Request{Query: `mutation { errorMethod }`},
			want: []fieldExpectations{
				{key: "errorMethod", value: valueExpectations{null: true}},
			},
			wantErrors: []*ResponseError{
				{
					Locations: []Location{{1, 12}},
					Path:      []PathSegment{{Field: "errorMethod"}},
				},
			},
		},
		{
			name: "Function/NoArgs",
			mutationObject: func() *testMutationStruct {
				return &testMutationStruct{
					StructField: true,
				}
			},
			request: Request{Query: `mutation { structField }`},
			want: []fieldExpectations{
				{key: "structField", value: valueExpectations{scalar: "true"}},
			},
		},
		{
			name: "Function/AllArgs",
			mutationObject: func(ctx context.Context, sel *SelectionSet) *testMutationStruct {
				if !sel.Has("structField") {
					t.Error("sel.Has(\"structField\") = false; want true")
				}
				return &testMutationStruct{
					StructField: true,
				}
			},
			request: Request{Query: `mutation { structField }`},
			want: []fieldExpectations{
				{key: "structField", value: valueExpectations{scalar: "true"}},
			},
		},
		{
			name: "Function/ErrorReturn/Nil",
			mutationObject: func() (*testMutationStruct, error) {
				return &testMutationStruct{
					StructField: true,
				}, nil
			},
			request: Request{Query: `mutation { structField }`},
			want: []fieldExpectations{
				{key: "structField", value: valueExpectations{scalar: "true"}},
			},
		},
		{
			name: "Function/ErrorReturn/Error",
			mutationObject: func() (*testMutationStruct, error) {
				return &testMutationStruct{
					StructField: true,
				}, xerrors.New("fail!")
			},
			request: Request{Query: `mutation { structField }`},
			wantErrors: []*ResponseError{
				new(ResponseError),
			},
		},
	}

	ctx := context.Background()
	schema, err := ParseSchema(schemaSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	queryObject := fieldResolverFunc(func(ctx context.Context, req FieldRequest) (interface{}, error) {
		return nil, xerrors.New("stub")
	})
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			srv, err := NewServer(schema, queryObject, test.mutationObject)
			if err != nil {
				t.Logf("NewServer: %v", err)
				if !test.wantInitError {
					t.Fail()
				}
				return
			}
			if test.wantInitError {
				t.Fatal("NewServer did not return error")
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

type testMutationStruct struct {
	StructField bool
}

func (testMutationStruct) Method() bool {
	return true
}

func (testMutationStruct) ErrorMethod() (bool, error) {
	return false, xerrors.New("failure")
}

func TestFieldResolver(t *testing.T) {
	t.Parallel()

	schema, err := ParseSchema(`
		type Query {
			foo(arg: String!): String!
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	resolver := fieldResolverFunc(func(ctx context.Context, req FieldRequest) (interface{}, error) {
		called = true
		if want := "foo"; req.Name != want {
			t.Errorf("req.Name = %q; want %q", req.Name, want)
		}
		if got, want := req.Args["arg"].Scalar(), "bar"; got != want {
			t.Errorf("req.Args[\"arg\"].Scalar() = %q; want %q", got, want)
		}
		return "baz", nil
	})
	srv, err := NewServer(schema, resolver, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	resp := srv.Execute(ctx, Request{
		Query: `{ foo(arg: "bar") }`,
	})
	for _, e := range resp.Errors {
		t.Errorf("Error: %s", e.Message)
	}
	if !called {
		t.Error("ResolveField method not called")
	}
	if got, want := resp.Data.ValueFor("foo").Scalar(), "baz"; got != want {
		t.Errorf("data.foo = %q; want %q", got, want)
	}
}

type fieldResolverFunc func(context.Context, FieldRequest) (interface{}, error)

func (f fieldResolverFunc) ResolveField(ctx context.Context, req FieldRequest) (interface{}, error) {
	return f(ctx, req)
}

func (f fieldResolverFunc) Foo() (string, error) {
	return "WRONG", xerrors.New("this method should never be called")
}

func TestOperationFinisher(t *testing.T) {
	t.Parallel()

	schema, err := ParseSchema(`
		type Query {
			foo: String!
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	t.Run("NoErrors", func(t *testing.T) {
		query := &testOperationFinisher{
			foo: "bar",
		}
		server, err := NewServer(schema, query, nil)
		if err != nil {
			t.Fatal(err)
		}
		response := server.Execute(ctx, Request{Query: "{ foo }"})
		if query.called != 1 {
			t.Errorf("FinishOperation called %d times; want 1", query.called)
		} else {
			if query.hasErrors {
				t.Error("FinishOperation reported HasErrors")
			}
		}
		for _, err := range response.Errors {
			t.Errorf("Response error: %v", err)
		}
		if got, want := response.Data.ValueFor("foo").Scalar(), "bar"; got != want {
			t.Errorf("foo = %q; want %q", got, want)
		}
	})

	t.Run("ReturnError", func(t *testing.T) {
		query := &testOperationFinisher{
			foo:         "bar",
			finishError: xerrors.New("BORK"),
		}
		server, err := NewServer(schema, query, nil)
		if err != nil {
			t.Fatal(err)
		}
		response := server.Execute(ctx, Request{Query: "{ foo }"})
		if query.called != 1 {
			t.Errorf("FinishOperation called %d times; want 1", query.called)
		} else {
			if query.hasErrors {
				t.Error("FinishOperation reported HasErrors")
			}
		}
		if len(response.Errors) != 1 {
			t.Errorf("Response returned %d errors; want 1", len(response.Errors))
		} else {
			err := response.Errors[0]
			t.Logf("Response error message: %s", err.Message)
			if want := "BORK"; !strings.Contains(err.Message, want) {
				t.Errorf("Response error message does not contain %q", want)
			}
			if len(err.Locations) > 0 || len(err.Path) > 0 {
				t.Error("Response error had locations and/or a path")
			}
		}
		if got, want := response.Data.ValueFor("foo").Scalar(), "bar"; got != want {
			t.Errorf("foo = %q; want %q", got, want)
		}
	})

	t.Run("FieldError", func(t *testing.T) {
		query := &testOperationFinisher{
			fooError: xerrors.New("can't fetch"),
		}
		server, err := NewServer(schema, query, nil)
		if err != nil {
			t.Fatal(err)
		}
		response := server.Execute(ctx, Request{Query: "{ foo }"})
		if query.called != 1 {
			t.Errorf("FinishOperation called %d times; want 1", query.called)
		} else {
			if !query.hasErrors {
				t.Error("FinishOperation did not report HasErrors")
			}
		}
		if len(response.Errors) != 1 {
			t.Errorf("Response returned %d errors; want 1", len(response.Errors))
		} else {
			err := response.Errors[0]
			t.Logf("Response error message: %s", err.Message)
			wantPath := []PathSegment{
				{Field: "foo"},
			}
			if diff := cmp.Diff(wantPath, err.Path); diff != "" {
				t.Errorf("Response error path (-want +got):\n%s", diff)
			}
		}
	})
}

type testOperationFinisher struct {
	foo      string
	fooError error

	called      int
	hasErrors   bool
	finishError error
}

func (f *testOperationFinisher) Foo() (string, error) {
	return f.foo, f.fooError
}

func (f *testOperationFinisher) FinishOperation(ctx context.Context, details *OperationDetails) error {
	f.called++
	f.hasErrors = details.HasErrors
	return f.finishError
}

func TestUnion(t *testing.T) {
	t.Parallel()

	schema, err := ParseSchema(`
		type Query {
			fooOrBar: FooOrBar
			fooOrFoo: FooOrFoo
		}

		union FooOrBar = UnionFoo | Bar

		type UnionFoo {
			foo: String
		}

		type Bar {
			bar: String
		}

		union FooOrFoo = UnionFoo | MyFoo

		type MyFoo {
			foo(arg: String): String
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	const fooOrBarQuery = `{ fooOrBar { __typename, ... on UnionFoo { foo }, ... on Bar { bar } } }`

	t.Run("StaticTypeName", func(t *testing.T) {
		q := &unionQuery{fooOrBar: &UnionFoo{Foo: "xyzzy"}}
		srv, err := NewServer(schema, q, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp := srv.Execute(ctx, Request{Query: fooOrBarQuery})
		if len(resp.Errors) > 0 {
			t.Fatal(resp.Errors)
		}

		q.mu.Lock()
		set := q.set
		q.mu.Unlock()
		if !set.Has("foo") {
			t.Error(`set.Has("foo") = false; want true`)
		}
		if !set.Has("bar") {
			t.Error(`set.Has("bar") = false; want true`)
		}

		(&valueExpectations{
			object: []fieldExpectations{
				{key: "__typename", value: valueExpectations{scalar: "UnionFoo"}},
				{key: "foo", value: valueExpectations{scalar: "xyzzy"}},
			},
		}).check(t, resp.Data.ValueFor("fooOrBar"))
	})

	t.Run("DynamicTypeName", func(t *testing.T) {
		q := &unionQuery{
			fooOrBar: &DynamicUnion{
				typename: "Bar",
				Foo:      "BORK",
				Bar:      "xyzzy",
			},
		}
		srv, err := NewServer(schema, q, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp := srv.Execute(ctx, Request{Query: fooOrBarQuery})
		if len(resp.Errors) > 0 {
			t.Fatal(resp.Errors)
		}

		q.mu.Lock()
		set := q.set
		q.mu.Unlock()
		if !set.Has("foo") {
			t.Error(`set.Has("foo") = false; want true`)
		}
		if !set.Has("bar") {
			t.Error(`set.Has("bar") = false; want true`)
		}

		(&valueExpectations{
			object: []fieldExpectations{
				{key: "__typename", value: valueExpectations{scalar: "Bar"}},
				{key: "bar", value: valueExpectations{scalar: "xyzzy"}},
			},
		}).check(t, resp.Data.ValueFor("fooOrBar"))
	})

	t.Run("OverlappingFieldNames", func(t *testing.T) {
		const fooOrFooQuery = `{ fooOrFoo {` +
			`__typename, ` +
			`... on UnionFoo { foo }, ` +
			`... on MyFoo { foo(arg: "xyzzy") }, ` +
			`} }`
		t.Run("WithoutArg", func(t *testing.T) {
			q := &unionQuery{
				fooOrFoo: UnionFoo{Foo: "static"},
			}
			srv, err := NewServer(schema, q, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp := srv.Execute(ctx, Request{Query: fooOrFooQuery})
			if len(resp.Errors) > 0 {
				t.Fatal(resp.Errors)
			}

			q.mu.Lock()
			set := q.set
			q.mu.Unlock()
			if !set.Has("foo") {
				t.Error(`set.Has("foo") = false; want true`)
			}

			(&valueExpectations{
				object: []fieldExpectations{
					{key: "__typename", value: valueExpectations{scalar: "UnionFoo"}},
					{key: "foo", value: valueExpectations{scalar: "static"}},
				},
			}).check(t, resp.Data.ValueFor("fooOrFoo"))
		})

		t.Run("WithArg", func(t *testing.T) {
			q := &unionQuery{
				fooOrFoo: myFoo{},
			}
			srv, err := NewServer(schema, q, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp := srv.Execute(ctx, Request{Query: fooOrFooQuery})
			if len(resp.Errors) > 0 {
				t.Fatal(resp.Errors)
			}

			q.mu.Lock()
			set := q.set
			q.mu.Unlock()
			if !set.Has("foo") {
				t.Error(`set.Has("foo") = false; want true`)
			}

			(&valueExpectations{
				object: []fieldExpectations{
					{key: "__typename", value: valueExpectations{scalar: "MyFoo"}},
					{key: "foo", value: valueExpectations{scalar: "xyzzy"}},
				},
			}).check(t, resp.Data.ValueFor("fooOrFoo"))
		})

		t.Run("Typename", func(t *testing.T) {
			q := &unionQuery{
				fooOrFoo: UnionFoo{Foo: "static"},
			}
			srv, err := NewServer(schema, q, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp := srv.Execute(ctx, Request{Query: `{ fooOrFoo {` +
				`... on MyFoo { __typename, foo(arg: "xyzzy") }, ` +
				`... on UnionFoo { foo }, ` +
				`__typename, ` +
				`} }`})
			if len(resp.Errors) > 0 {
				t.Fatal(resp.Errors)
			}

			q.mu.Lock()
			set := q.set
			q.mu.Unlock()
			if !set.Has("__typename") {
				t.Error(`set.Has("__typename") = false; want true`)
			}
			if !set.Has("foo") {
				t.Error(`set.Has("foo") = false; want true`)
			}

			(&valueExpectations{
				object: []fieldExpectations{
					{key: "__typename", value: valueExpectations{scalar: "UnionFoo"}},
					{key: "foo", value: valueExpectations{scalar: "static"}},
				},
			}).check(t, resp.Data.ValueFor("fooOrFoo"))
		})
	})
}

type unionQuery struct {
	fooOrBar interface{}
	fooOrFoo interface{}

	mu  sync.Mutex
	set *SelectionSet
}

func (q *unionQuery) FooOrBar(set *SelectionSet) interface{} {
	q.mu.Lock()
	q.set = set
	q.mu.Unlock()
	return q.fooOrBar
}

func (q *unionQuery) FooOrFoo(set *SelectionSet) interface{} {
	q.mu.Lock()
	q.set = set
	q.mu.Unlock()
	return q.fooOrFoo
}

type UnionFoo struct {
	Foo string
}

type DynamicUnion struct {
	typename string

	Foo string
	Bar string
}

func (u *DynamicUnion) GraphQLType() string {
	return u.typename
}

type myFoo struct{}

func (myFoo) GraphQLType() string {
	return "MyFoo"
}

type myFooArgs struct {
	Arg string
}

func (myFoo) Foo(args myFooArgs) string {
	return args.Arg
}

func newString(s string) *string { return &s }
func newBool(b bool) *bool       { return &b }
func newInt(i int) *int          { return &i }
func newInt32(i int32) *int32    { return &i }
func newInt64(i int64) *int64    { return &i }

type valueExpectations struct {
	null   bool
	scalar string
	list   []valueExpectations
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
	if expect.list != nil {
		if v.IsNull() {
			return
		}
		if v.Len() != len(expect.list) {
			e.Errorf("len(v) == %d; want %d", v.Len(), len(expect.list))
		}
		for i := 0; i < v.Len() && i < len(expect.list); i++ {
			p := prefixErrorfer{
				prefix:   fmt.Sprintf("list[%d]: ", i),
				errorfer: e,
			}
			expect.list[i].check(p, v.At(i))
		}
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
				continue
			}
			p := prefixErrorfer{
				prefix:   fmt.Sprintf("field %s: ", gotField.Key),
				errorfer: e,
			}
			wantField.value.check(p, gotField.Value)
		}
	}
}

type errorfer interface {
	Errorf(format string, arguments ...interface{})
}

type prefixErrorfer struct {
	prefix   string
	errorfer errorfer
}

func (p prefixErrorfer) Errorf(format string, arguments ...interface{}) {
	inner := fmt.Sprintf(format, arguments...)
	p.errorfer.Errorf("%s", p.prefix+inner)
}

func TestResponseMarshalJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		v    Response
		want []json.Token
	}{
		{
			name: "DataOnly",
			v: Response{
				Data: testObjectValue(),
			},
			want: []json.Token{
				json.Delim('{'),
				"data",
				json.Delim('{'),
				"myInt", json.Number("42"),
				"myString", "xyzzy",
				json.Delim('}'),
				json.Delim('}'),
			},
		},
		{
			name: "ErrorsOnly",
			v: Response{
				Errors: []*ResponseError{
					{
						Message: "Failure",
						Locations: []Location{
							{Line: 12, Column: 34},
						},
						Path: []PathSegment{
							{Field: "myList"},
							{ListIndex: 33},
							{Field: "id"},
						},
					},
				},
			},
			want: []json.Token{
				json.Delim('{'),
				"errors",
				json.Delim('['),
				json.Delim('{'),
				"message", "Failure",

				"locations",
				json.Delim('['),
				json.Delim('{'),
				"line", json.Number("12"),
				"column", json.Number("34"),
				json.Delim('}'),
				json.Delim(']'),

				"path",
				json.Delim('['),
				"myList",
				json.Number("33"),
				"id",
				json.Delim(']'),

				json.Delim('}'),
				json.Delim(']'),
				json.Delim('}'),
			},
		},
		{
			name: "ErrorJustMessage",
			v: Response{
				Errors: []*ResponseError{
					{
						Message: "Failure",
					},
				},
			},
			want: []json.Token{
				json.Delim('{'),
				"errors",
				json.Delim('['),
				json.Delim('{'),
				"message", "Failure",
				json.Delim('}'),
				json.Delim(']'),
				json.Delim('}'),
			},
		},
		{
			name: "DataAndErrors",
			v: Response{
				Data: testObjectValue(),
				Errors: []*ResponseError{
					{
						Message: "Failure",
					},
				},
			},
			want: []json.Token{
				json.Delim('{'),

				// Errors should come first, as per recommendation in
				// https://graphql.github.io/graphql-spec/June2018/#sec-Response-Format
				"errors",
				json.Delim('['),
				json.Delim('{'),
				"message", "Failure",
				json.Delim('}'),
				json.Delim(']'),

				"data",
				json.Delim('{'),
				"myInt", json.Number("42"),
				"myString", "xyzzy",
				json.Delim('}'),

				json.Delim('}'),
			},
		},
		{
			name: "IDs",
			v: Response{
				Data: testIDObjectValue(),
			},
			want: []json.Token{
				json.Delim('{'),
				"data",
				json.Delim('{'),
				"myStringId", "xyzzy",
				"myInt64Id", "42",
				json.Delim('}'),
				json.Delim('}'),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(test.v)
			if err != nil {
				t.Fatal("Marshal:", err)
			}
			var got []json.Token
			dec := json.NewDecoder(bytes.NewReader(data))
			dec.UseNumber()
			for {
				tok, err := dec.Token()
				if xerrors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal("Token:", err)
				}
				got = append(got, tok)
			}
			diff := cmp.Diff(test.want, got, cmpopts.EquateEmpty())
			if diff != "" {
				t.Errorf("JSON tokens (-want +got):\n%s", diff)
			}
		})
	}
}

func testObjectValue() Value {
	schema, err := ParseSchema(`
		type Query {
			myString: String
			myInt: Int
		}
	`, nil)
	if err != nil {
		panic(err)
	}
	queryObject := &testQueryStruct{
		MyString: NullString{S: "xyzzy", Valid: true},
		MyInt:    newInt(42),
	}
	srv, err := NewServer(schema, queryObject, nil)
	if err != nil {
		panic(err)
	}
	response := srv.Execute(context.Background(), Request{
		Query: `{ myInt, myString }`,
	})
	if len(response.Errors) > 0 {
		panic(response.Errors[0])
	}
	return response.Data
}

func testIDObjectValue() Value {
	schema, err := ParseSchema(`
		type Query {
			myStringId: ID
			myInt64Id: ID
		}
	`, nil)
	if err != nil {
		panic(err)
	}
	queryObject := &testQueryStruct{
		MyStringID: NullString{S: "xyzzy", Valid: true},
		MyInt64ID:  newInt64(42),
	}
	srv, err := NewServer(schema, queryObject, nil)
	if err != nil {
		panic(err)
	}
	response := srv.Execute(context.Background(), Request{
		Query: `{ myStringId, myInt64Id }`,
	})
	if len(response.Errors) > 0 {
		panic(response.Errors[0])
	}
	return response.Data
}
