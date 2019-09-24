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
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestValueFromGo(t *testing.T) {
	schema, err := ParseSchema(`
		type Query {
			foo: String
			bar: String
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name         string
		goValue      reflect.Value
		goValueFunc  func(e errorfer) reflect.Value
		typ          *gqlType
		selectionSet *SelectionSet
		want         valueExpectations
	}{
		{
			name:    "String/Empty",
			goValue: reflect.ValueOf(""),
			typ:     stringType,
			want:    valueExpectations{scalar: ""},
		},
		{
			name:    "String/Nonempty",
			goValue: reflect.ValueOf("foo"),
			typ:     stringType,
			want:    valueExpectations{scalar: "foo"},
		},
		{
			name:    "String/Null",
			goValue: reflect.ValueOf(new(*string)).Elem(),
			typ:     stringType,
			want:    valueExpectations{null: true},
		},
		{
			name:    "Boolean/True",
			goValue: reflect.ValueOf(true),
			typ:     booleanType,
			want:    valueExpectations{scalar: "true"},
		},
		{
			name:    "Boolean/False",
			goValue: reflect.ValueOf(false),
			typ:     booleanType,
			want:    valueExpectations{scalar: "false"},
		},
		{
			name:    "Boolean/Null",
			goValue: reflect.ValueOf(new(*bool)).Elem(),
			typ:     booleanType,
			want:    valueExpectations{null: true},
		},
		{
			name:    "Integer/Int32/Zero",
			goValue: reflect.ValueOf(int32(0)),
			typ:     intType,
			want:    valueExpectations{scalar: "0"},
		},
		{
			name:    "Integer/Int32/Positive",
			goValue: reflect.ValueOf(int32(123)),
			typ:     intType,
			want:    valueExpectations{scalar: "123"},
		},
		{
			name:    "Integer/Int32/Negative",
			goValue: reflect.ValueOf(int32(-123)),
			typ:     intType,
			want:    valueExpectations{scalar: "-123"},
		},
		{
			name:    "Integer/Int32/Null",
			goValue: reflect.ValueOf(new(*int32)).Elem(),
			typ:     intType,
			want:    valueExpectations{null: true},
		},
		{
			name:    "Integer/Int/Zero",
			goValue: reflect.ValueOf(int(0)),
			typ:     intType,
			want:    valueExpectations{scalar: "0"},
		},
		{
			name:    "Integer/Int/Positive",
			goValue: reflect.ValueOf(int(123)),
			typ:     intType,
			want:    valueExpectations{scalar: "123"},
		},
		{
			name:    "Integer/Int/Negative",
			goValue: reflect.ValueOf(int(-123)),
			typ:     intType,
			want:    valueExpectations{scalar: "-123"},
		},
		{
			name:    "Integer/Int/Null",
			goValue: reflect.ValueOf(new(*int)).Elem(),
			typ:     intType,
			want:    valueExpectations{null: true},
		},
		{
			name: "Object/StructFields",
			goValue: reflect.ValueOf(&valueQueryStructFields{
				Foo: "xyzzy",
				Bar: "baz",
			}),
			typ: schema.query,
			selectionSet: &SelectionSet{
				fields: []*selectionField{
					{name: "foo", key: "foo"},
					{name: "bar", key: "bar"},
				},
			},
			want: valueExpectations{object: []fieldExpectations{
				{key: "foo", value: valueExpectations{scalar: "xyzzy"}},
				{key: "bar", value: valueExpectations{scalar: "baz"}},
			}},
		},
		{
			name: "Object/PartialStructFields",
			goValue: reflect.ValueOf(&valueQueryStructFields{
				Foo: "xyzzy",
				Bar: "baz",
			}),
			typ: schema.query,
			selectionSet: &SelectionSet{
				fields: []*selectionField{
					{name: "bar", key: "bar"},
				},
			},
			want: valueExpectations{object: []fieldExpectations{
				{key: "bar", value: valueExpectations{scalar: "baz"}},
			}},
		},
		{
			name:    "Object/Method/NoArgs",
			goValue: reflect.ValueOf(new(valueQueryMethodNoArgs)),
			typ:     schema.query,
			selectionSet: &SelectionSet{
				fields: []*selectionField{
					{name: "foo", key: "foo"},
				},
			},
			want: valueExpectations{object: []fieldExpectations{
				{key: "foo", value: valueExpectations{scalar: "xyzzy"}},
			}},
		},
		{
			name: "Object/Method/ContextOnly",
			goValueFunc: func(e errorfer) reflect.Value {
				return reflect.ValueOf(&valueQueryMethodContextOnly{e: e})
			},
			typ: schema.query,
			selectionSet: &SelectionSet{
				fields: []*selectionField{
					{name: "foo", key: "foo"},
				},
			},
			want: valueExpectations{object: []fieldExpectations{
				{key: "foo", value: valueExpectations{scalar: "xyzzy"}},
			}},
		},
		{
			name: "Object/Method/ArgsOnly",
			goValueFunc: func(e errorfer) reflect.Value {
				return reflect.ValueOf(&valueQueryMethodArgsOnly{e: e})
			},
			typ: schema.query,
			selectionSet: &SelectionSet{
				fields: []*selectionField{
					{name: "foo", key: "foo"},
				},
			},
			want: valueExpectations{object: []fieldExpectations{
				{key: "foo", value: valueExpectations{scalar: "xyzzy"}},
			}},
		},
		{
			name: "Object/Alias",
			goValue: reflect.ValueOf(&valueQueryStructFields{
				Foo: "xyzzy",
				Bar: "BORK",
			}),
			typ: schema.query,
			selectionSet: &SelectionSet{
				fields: []*selectionField{
					{name: "foo", key: "magic"},
					{name: "foo", key: "bar"},
				},
			},
			want: valueExpectations{object: []fieldExpectations{
				{key: "magic", value: valueExpectations{scalar: "xyzzy"}},
				{key: "bar", value: valueExpectations{scalar: "xyzzy"}},
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			goValue := test.goValue
			if test.goValueFunc != nil {
				goValue = test.goValueFunc(t)
			}
			got, errs := valueFromGo(context.Background(), goValue, test.typ, test.selectionSet)
			if len(errs) > 0 {
				t.Fatalf("errors: %v", errs)
			}
			test.want.check(t, got)
		})
	}
}

type valueQueryStructFields struct {
	Foo string
	Bar string
}

type valueQueryMethodNoArgs struct{}

func (valueQueryMethodNoArgs) Foo() string {
	return "xyzzy"
}

type valueQueryMethodContextOnly struct {
	e errorfer
}

func (q *valueQueryMethodContextOnly) Foo(ctx context.Context) string {
	if ctx == nil {
		q.e.Errorf("Foo received nil Context")
	}
	return "xyzzy"
}

type valueQueryMethodArgsOnly struct {
	e errorfer
}

func (q *valueQueryMethodArgsOnly) Foo(args map[string]Value) string {
	if len(args) > 0 {
		q.e.Errorf("Foo received non-empty args: %v", args)
	}
	return "xyzzy"
}

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
