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
)

func TestValueFromGo(t *testing.T) {
	schema, err := ParseSchema(`type Query { foo: String }`)
	if err != nil {
		t.Fatal(err)
	}
	_ = schema
	tests := []struct {
		name         string
		goValue      reflect.Value
		typ          *gqlType
		selectionSet *SelectionSet

		wantNull   bool
		wantScalar string
	}{
		{
			name:       "String/Empty",
			goValue:    reflect.ValueOf(""),
			typ:        stringType,
			wantScalar: "",
		},
		{
			name:       "String/Nonempty",
			goValue:    reflect.ValueOf("foo"),
			typ:        stringType,
			wantScalar: "foo",
		},
		{
			name:     "String/Null",
			goValue:  reflect.ValueOf(new(*string)).Elem(),
			typ:      stringType,
			wantNull: true,
		},
		{
			name:       "Boolean/True",
			goValue:    reflect.ValueOf(true),
			typ:        booleanType,
			wantScalar: "true",
		},
		{
			name:       "Boolean/False",
			goValue:    reflect.ValueOf(false),
			typ:        booleanType,
			wantScalar: "false",
		},
		{
			name:     "Boolean/Null",
			goValue:  reflect.ValueOf(new(*bool)).Elem(),
			typ:      booleanType,
			wantNull: true,
		},
		{
			name:       "Integer/Int32/Zero",
			goValue:    reflect.ValueOf(int32(0)),
			typ:        intType,
			wantScalar: "0",
		},
		{
			name:       "Integer/Int32/Positive",
			goValue:    reflect.ValueOf(int32(123)),
			typ:        intType,
			wantScalar: "123",
		},
		{
			name:       "Integer/Int32/Negative",
			goValue:    reflect.ValueOf(int32(-123)),
			typ:        intType,
			wantScalar: "-123",
		},
		{
			name:     "Integer/Int32/Null",
			goValue:  reflect.ValueOf(new(*int32)).Elem(),
			typ:      intType,
			wantNull: true,
		},
		{
			name:       "Integer/Int/Zero",
			goValue:    reflect.ValueOf(int(0)),
			typ:        intType,
			wantScalar: "0",
		},
		{
			name:       "Integer/Int/Positive",
			goValue:    reflect.ValueOf(int(123)),
			typ:        intType,
			wantScalar: "123",
		},
		{
			name:       "Integer/Int/Negative",
			goValue:    reflect.ValueOf(int(-123)),
			typ:        intType,
			wantScalar: "-123",
		},
		{
			name:     "Integer/Int/Null",
			goValue:  reflect.ValueOf(new(*int)).Elem(),
			typ:      intType,
			wantNull: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, errs := valueFromGo(context.Background(), test.goValue, test.typ, test.selectionSet)
			if len(errs) > 0 {
				t.Fatalf("errors: %+v", errs)
			}
			if gotNull := got.IsNull(); gotNull != test.wantNull {
				t.Errorf("v.IsNull() = %t; want %t", gotNull, test.wantNull)
			}
			if gotScalar := got.Scalar(); gotScalar != test.wantScalar {
				t.Errorf("v.Scalar() = %q; want %q", gotScalar, test.wantScalar)
			}
		})
	}
}
