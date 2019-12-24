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
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestValue_Convert(t *testing.T) {
	t.Parallel()

	type inputObject struct {
		AAA int
		ZZZ string
	}
	type object struct {
		MyString string
		MyInt    int
	}

	tests := []struct {
		name    string
		value   Value
		dst     interface{}
		want    interface{}
		wantErr bool
	}{
		{
			name:  "Null/String",
			value: Value{},
			dst:   newString("abc"),
			want:  "",
		},
		{
			name:  "Null/Int",
			value: Value{},
			dst:   newInt(123),
			want:  0,
		},
		{
			name:  "Boolean/True",
			value: testValue(true, "Boolean!"),
			dst:   new(bool),
			want:  true,
		},
		{
			name:  "Boolean/False",
			value: testValue(false, "Boolean!"),
			dst:   new(bool),
			want:  false,
		},
		{
			name:  "Int/ToString",
			value: testIntValue(42),
			dst:   new(string),
			want:  "42",
		},
		{
			name:  "Int/ToInt",
			value: testIntValue(42),
			dst:   new(int),
			want:  42,
		},
		{
			name:  "String",
			value: testStringValue("foo"),
			dst:   new(string),
			want:  "foo",
		},
		{
			name:    "String/ToInt/Bad",
			value:   testStringValue("foo"),
			dst:     new(int),
			wantErr: true,
		},
		{
			name:  "String/ToInt/Good",
			value: testStringValue("123"),
			dst:   new(int),
			want:  123,
		},
		{
			name:  "Enum/ToString",
			value: testValue("FOO", "MyEnum!"),
			dst:   new(string),
			want:  "FOO",
		},
		{
			name:    "Enum/ToInt",
			value:   testValue("FOO", "MyEnum!"),
			dst:     new(int),
			wantErr: true,
		},
		{
			name:  "StringList",
			value: testStringListValue([]string{"foo", "bar"}),
			dst:   new([]string),
			want:  []string{"foo", "bar"},
		},
		{
			name: "InputObject/ToValue",
			value: testInputObjectValue(InputObject(map[string]Input{
				"aaa": ScalarInput("123"),
				"zzz": ScalarInput("456"),
			})),
			dst: new(inputObject),
			want: inputObject{
				AAA: 123,
				ZZZ: "456",
			},
		},
		{
			name: "InputObject/ToPointer",
			value: testInputObjectValue(InputObject(map[string]Input{
				"aaa": ScalarInput("123"),
				"zzz": ScalarInput("456"),
			})),
			dst: new(*inputObject),
			want: &inputObject{
				AAA: 123,
				ZZZ: "456",
			},
		},
		{
			name: "InputObject/ToMap",
			value: testInputObjectValue(InputObject(map[string]Input{
				"aaa": ScalarInput("123"),
				"zzz": ScalarInput("456"),
			})),
			dst: new(map[string]Value),
			want: map[string]Value{
				"aaa": testValue(123, "Int!"),
				"zzz": testValue("456", "String!"),
			},
		},
		{
			name:  "Object/ToValue",
			value: testObjectValue(),
			dst:   new(object),
			want: object{
				MyString: "xyzzy",
				MyInt:    42,
			},
		},
		{
			name:  "Object/ToPointer",
			value: testObjectValue(),
			dst:   new(*object),
			want: &object{
				MyString: "xyzzy",
				MyInt:    42,
			},
		},
		{
			name:  "Object/ToMap",
			value: testObjectValue(),
			dst:   new(map[string]Value),
			want: map[string]Value{
				"myString": testValue("xyzzy", "String"),
				"myInt":    testValue(42, "Int"),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.value.Convert(test.dst)
			if err != nil {
				t.Logf("Error: %v", err)
				if !test.wantErr {
					t.Fail()
				}
				return
			}
			if test.wantErr {
				t.Fatal("Convert did not return error")
			}
			got := reflect.ValueOf(test.dst).Elem().Interface()
			diff := cmp.Diff(test.want, got,
				cmp.AllowUnexported(inputObject{}, Value{}),
				cmp.Comparer(func(t1, t2 *gqlType) bool {
					return t1 == t2
				}))
			if diff != "" {
				t.Errorf("-want +got:\n%s", diff)
			}
		})
	}
}
