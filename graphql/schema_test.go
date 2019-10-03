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
	"testing"
)

func TestParseSchema(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr bool
	}{
		{
			name:    "Empty",
			source:  "",
			wantErr: true,
		},
		{
			name:    "EmptyQuery",
			source:  "type Query {}",
			wantErr: true,
		},
		{
			name:    "SingleStringField",
			source:  "type Query { foo: String }",
			wantErr: false,
		},
		{
			name:    "ScalarType",
			source:  "type Query { foo: Bar }\nscalar Bar",
			wantErr: false,
		},
		{
			name:    "DuplicateTypeName",
			source:  "type Query { foo: String }\nscalar Bar\nscalar Bar",
			wantErr: true,
		},
		{
			name:    "UnknownType",
			source:  "type Query { foo: Bar }",
			wantErr: true,
		},
		{
			name:    "OperationsNotAllowed",
			source:  "type Query { foo: String }\nquery { foo }",
			wantErr: true,
		},
		{
			name:    "ReservedFieldName",
			source:  "type Query { __foo: String }",
			wantErr: true,
		},
		{
			name:    "ReservedTypeName",
			source:  "type Query { foo: String }\nscalar __Foo\n",
			wantErr: true,
		},
		{
			name:    "ScalarQuery",
			source:  "scalar Query",
			wantErr: true,
		},
		{
			name:    "BuiltinConflict",
			source:  "type Query { foo: String }\nscalar String",
			wantErr: true,
		},
		{
			name:    "DuplicateFieldName",
			source:  "type Query { foo: String, foo: String }",
			wantErr: true,
		},
		{
			name:    "Arguments",
			source:  "type Query { foo(bar: Boolean!): String }",
			wantErr: false,
		},
		{
			name:    "Arguments/UnknownType",
			source:  "type Query { foo(bar: Bar): String }",
			wantErr: true,
		},
		{
			name:    "Arguments/DuplicateNames",
			source:  "type Query { foo(bar: Boolean!, bar: Boolean!): String }",
			wantErr: true,
		},
		{
			name:    "Arguments/ReservedName",
			source:  "type Query { foo(__bar: Boolean!): String }",
			wantErr: true,
		},
		{
			name:    "Arguments/OutputType",
			source:  "type Query { foo(bar: Bar): String }\ntype Bar { xyzzy: Boolean! }",
			wantErr: true,
		},
		{
			name:    "Arguments/DefaultValue",
			source:  "type Query { foo(bar: Boolean! = true): String }",
			wantErr: false,
		},
		{
			name:    "Arguments/DefaultValue/InvalidType",
			source:  "type Query { foo(bar: Boolean! = 123): String }",
			wantErr: true,
		},
		{
			name:    "Arguments/DefaultValue/NullForNullable",
			source:  "type Query { foo(bar: Boolean = null): String }",
			wantErr: false,
		},
		{
			name:    "Arguments/DefaultValue/NullForNonNullable",
			source:  "type Query { foo(bar: Boolean! = null): String }",
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseSchema(test.source)
			if err != nil {
				t.Logf("Error: %v", err)
				if !test.wantErr {
					t.Fail()
				}
			} else if test.wantErr {
				t.Error("ParseSchema did not return error")
			}
		})
	}
}
