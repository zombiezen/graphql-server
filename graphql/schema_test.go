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
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-36555
			name: "EnumType",
			source: `
				type Query { adjacent(direction: Direction! = NORTH): ID }

				enum Direction {
					NORTH
					EAST
					SOUTH
					WEST
				}
			`,
			wantErr: false,
		},
		{
			name: "EnumType/WrongName",
			source: `
				type Query { adjacent(direction: Direction! = WEAST): ID }

				enum Direction {
					NORTH
					EAST
					SOUTH
					WEST
				}
			`,
			wantErr: true,
		},
		{
			name: "EnumType/DuplicateName",
			source: `
				type Query { adjacent(direction: Direction!): ID }

				enum Direction {
					NORTH
					NORTH
					EAST
					SOUTH
					WEST
				}
			`,
			wantErr: true,
		},
		{
			name: "EnumType/ReservedName",
			source: `
				type Query { adjacent(direction: Direction!): ID }

				enum Direction {
					__NORTH
					EAST
					SOUTH
					WEST
				}
			`,
			wantErr: true,
		},
		{
			name: "InputObject",
			source: `
				type Query {
					magnitude(vec: Point2D): Float
				}

				input Point2D {
					x: Float
					y: Float
				}
			`,
			wantErr: false,
		},
		{
			name: "InputObject/ReservedFieldName",
			source: `
				type Query {
					magnitude(vec: Point2D): Float
				}

				input Point2D {
					__x: Float
				}
			`,
			wantErr: true,
		},
		{
			name: "InputObject/DuplicateFieldNames",
			source: `
				type Query {
					magnitude(vec: Point2D): Float
				}

				input Point2D {
					x: Float
					x: Float
				}
			`,
			wantErr: true,
		},
		{
			name: "InputObject/CantUseAsFieldType",
			source: `
				type Query {
					thePoint: Point2D
				}

				input Point2D {
					x: Float
					y: Float
				}
			`,
			wantErr: true,
		},
		{
			name: "InputObject/CantUseOutputTypeInInputFields",
			source: `
				type Query {
					foo: Foo
				}

				type Foo {
					s: String
				}

				input Bar {
					x: Foo
				}
			`,
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseSchema(test.source, nil)
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
