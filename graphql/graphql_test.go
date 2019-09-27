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
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestExecute(t *testing.T) {
	const schemaSource = `
		type Query {
			myString: String
			myBoolean: Boolean
			myInt: Int
			myInt32: Int

			noArgsMethod: String
			contextOnlyMethod: String
			argsOnlyMethod: String
			contextAndArgsMethod: String
		}

	`
	tests := []struct {
		name        string
		queryObject func(e errorfer) interface{}
		query       string
		want        []fieldExpectations
	}{
		{
			name: "String/Empty",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyString: newString("")}
			},
			query: `{ myString }`,
			want: []fieldExpectations{
				{key: "myString", value: valueExpectations{scalar: ""}},
			},
		},
		{
			name: "String/Nonempty",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyString: newString("foo")}
			},
			query: `{ myString }`,
			want: []fieldExpectations{
				{key: "myString", value: valueExpectations{scalar: "foo"}},
			},
		},
		{
			name: "String/Null",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyString: nil}
			},
			query: `{ myString }`,
			want: []fieldExpectations{
				{key: "myString", value: valueExpectations{null: true}},
			},
		},
		{
			name: "Boolean/True",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyBoolean: newBool(true)}
			},
			query: `{ myBoolean }`,
			want: []fieldExpectations{
				{key: "myBoolean", value: valueExpectations{scalar: "true"}},
			},
		},
		{
			name: "Boolean/False",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyBoolean: newBool(false)}
			},
			query: `{ myBoolean }`,
			want: []fieldExpectations{
				{key: "myBoolean", value: valueExpectations{scalar: "false"}},
			},
		},
		{
			name: "Boolean/Null",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyBoolean: nil}
			},
			query: `{ myBoolean }`,
			want: []fieldExpectations{
				{key: "myBoolean", value: valueExpectations{null: true}},
			},
		},
		{
			name: "Integer/Int32/Zero",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt32: newInt32(0)}
			},
			query: `{ myInt32 }`,
			want: []fieldExpectations{
				{key: "myInt32", value: valueExpectations{scalar: "0"}},
			},
		},
		{
			name: "Integer/Int32/Positive",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt32: newInt32(123)}
			},
			query: `{ myInt32 }`,
			want: []fieldExpectations{
				{key: "myInt32", value: valueExpectations{scalar: "123"}},
			},
		},
		{
			name: "Integer/Int32/Negative",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt32: newInt32(-123)}
			},
			query: `{ myInt32 }`,
			want: []fieldExpectations{
				{key: "myInt32", value: valueExpectations{scalar: "-123"}},
			},
		},
		{
			name: "Integer/Int32/Null",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt32: nil}
			},
			query: `{ myInt32 }`,
			want: []fieldExpectations{
				{key: "myInt32", value: valueExpectations{null: true}},
			},
		},
		{
			name: "Integer/Int/Zero",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt: newInt(0)}
			},
			query: `{ myInt }`,
			want: []fieldExpectations{
				{key: "myInt", value: valueExpectations{scalar: "0"}},
			},
		},
		{
			name: "Integer/Int/Positive",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt: newInt(123)}
			},
			query: `{ myInt }`,
			want: []fieldExpectations{
				{key: "myInt", value: valueExpectations{scalar: "123"}},
			},
		},
		{
			name: "Integer/Int/Negative",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt: newInt(-123)}
			},
			query: `{ myInt }`,
			want: []fieldExpectations{
				{key: "myInt", value: valueExpectations{scalar: "-123"}},
			},
		},
		{
			name: "Integer/Int/Null",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt: nil}
			},
			query: `{ myInt }`,
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
			query: `{
				myInt
				myString
			}`,
			want: []fieldExpectations{
				{key: "myInt", value: valueExpectations{scalar: "42"}},
				{key: "myString", value: valueExpectations{scalar: "hello"}},
			},
		},
		{
			name: "Object/Method/NoArgs",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			query: `{ noArgsMethod }`,
			want: []fieldExpectations{
				{key: "noArgsMethod", value: valueExpectations{scalar: "xyzzy"}},
			},
		},
		{
			name: "Object/Method/ContextOnly",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			query: `{ contextOnlyMethod }`,
			want: []fieldExpectations{
				{key: "contextOnlyMethod", value: valueExpectations{scalar: "xyzzy"}},
			},
		},
		{
			name: "Object/Method/ArgsOnly",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			query: `{ argsOnlyMethod }`,
			want: []fieldExpectations{
				{key: "argsOnlyMethod", value: valueExpectations{scalar: "xyzzy"}},
			},
		},
		{
			name: "Object/Method/ContextAndArgs",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{e: e}
			},
			query: `{ contextAndArgsMethod }`,
			want: []fieldExpectations{
				{key: "contextAndArgsMethod", value: valueExpectations{scalar: "xyzzy"}},
			},
		},
		{
			name: "Object/Alias",
			queryObject: func(e errorfer) interface{} {
				return &testQueryStruct{MyInt32: newInt32(42)}
			},
			query: `{ magic: myInt32, myInt: myInt32 }`,
			want: []fieldExpectations{
				{key: "magic", value: valueExpectations{scalar: "42"}},
				{key: "myInt", value: valueExpectations{scalar: "42"}},
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
			resp := srv.Execute(ctx, Request{Query: test.query})
			if len(resp.Errors) > 0 {
				t.Fatal("Errors:", resp.Errors)
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
	if len(args) > 0 {
		q.e.Errorf("Foo received non-empty args: %v", args)
	}
	return "xyzzy"
}

func (q *testQueryStruct) ContextAndArgsMethod(ctx context.Context, args map[string]Value) string {
	if len(args) > 0 {
		q.e.Errorf("Foo received non-empty args: %v", args)
	}
	return "xyzzy"
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
		if v.Len() != len(expect.object) {
			var gotKeys, wantKeys []string
			for i := 0; i < v.Len(); i++ {
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
