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
)

func TestIntrospection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		schema      string
		options     *SchemaOptions
		hasMutation bool
		request     Request
		want        []fieldExpectations
	}{
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-00283
			name: "Type/User",
			schema: `
				type Query {
					foo: String
				}

				type User {
					id: String
					name: String
					birthday: Date
				}

				scalar Date
			`,
			request: Request{Query: `{
				__type(name: "User") {
					name
					fields {
						name
						type {
							name
						}
					}
				}
			}`},
			want: []fieldExpectations{
				{key: "__type", value: valueExpectations{object: []fieldExpectations{
					{key: "name", value: valueExpectations{scalar: "User"}},
					{key: "fields", value: valueExpectations{list: []valueExpectations{
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "id"}},
							{key: "type", value: valueExpectations{object: []fieldExpectations{
								{key: "name", value: valueExpectations{scalar: "String"}},
							}}},
						}},
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "name"}},
							{key: "type", value: valueExpectations{object: []fieldExpectations{
								{key: "name", value: valueExpectations{scalar: "String"}},
							}}},
						}},
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "birthday"}},
							{key: "type", value: valueExpectations{object: []fieldExpectations{
								{key: "name", value: valueExpectations{scalar: "Date"}},
							}}},
						}},
					}}},
				}}},
			},
		},
		{
			name: "Type/AllFields",
			schema: `
				"""
				The root query type.
				"""
				type Query {
					"""
					A very important field
					that I read from.
					"""
					foo: String
				}
			`,
			request: Request{
				Query: `{
					__type(name: "Query") {
						kind
						name
						description
						fields {
							name
							description
							args {
								name
							}
							type {
								kind
								name
							}
							isDeprecated
							deprecationReason
						}
						interfaces {
							name
						}
					}
				}`,
			},
			want: []fieldExpectations{
				{key: "__type", value: valueExpectations{object: []fieldExpectations{
					{key: "kind", value: valueExpectations{scalar: "OBJECT"}},
					{key: "name", value: valueExpectations{scalar: "Query"}},
					{key: "description", value: valueExpectations{scalar: "The root query type."}},
					{key: "fields", value: valueExpectations{list: []valueExpectations{
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "foo"}},
							{key: "description", value: valueExpectations{scalar: "A very important field\nthat I read from."}},
							{key: "args", value: valueExpectations{list: []valueExpectations{}}},
							{key: "type", value: valueExpectations{object: []fieldExpectations{
								{key: "kind", value: valueExpectations{scalar: "SCALAR"}},
								{key: "name", value: valueExpectations{scalar: "String"}},
							}}},
							{key: "isDeprecated", value: valueExpectations{scalar: "false"}},
							{key: "deprecationReason", value: valueExpectations{null: true}},
						}},
					}}},
					{key: "interfaces", value: valueExpectations{list: []valueExpectations{}}},
				}}},
			},
		},
		{
			name: "Type/IgnoreDescriptions",
			schema: `
				"""Hello, World!"""
				type Query {
					foo: String!
				}
			`,
			options: &SchemaOptions{
				IgnoreDescriptions: true,
			},
			request: Request{
				Query: `{
					__type(name: "Query") {
						description
					}
				}`,
			},
			want: []fieldExpectations{
				{key: "__type", value: valueExpectations{object: []fieldExpectations{
					{key: "description", value: valueExpectations{null: true}},
				}}},
			},
		},
		{
			name: "Schema/QueryAndMutation",
			schema: `
				type Query {
					foo: String
				}

				type Mutation {
					foo: String
				}

				scalar Foo
			`,
			hasMutation: true,
			request: Request{
				Query: `{
					__schema {
						queryType {
							name
						}
						mutationType {
							name
						}
						subscriptionType {
							name
						}
						types {
							name
						}
						directives {
							name
							locations
							args {
								name
								type {
									kind
									name
								}
							}
						}
					}
				}`,
			},
			want: []fieldExpectations{
				{key: "__schema", value: valueExpectations{object: []fieldExpectations{
					{key: "queryType", value: valueExpectations{object: []fieldExpectations{
						{key: "name", value: valueExpectations{scalar: "Query"}},
					}}},
					{key: "mutationType", value: valueExpectations{object: []fieldExpectations{
						{key: "name", value: valueExpectations{scalar: "Mutation"}},
					}}},
					{key: "subscriptionType", value: valueExpectations{null: true}},
					{key: "types", value: valueExpectations{list: []valueExpectations{
						// TODO(someday): Order here doesn't matter, but at the moment,
						// the implementation will always return this order.
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "Boolean"}},
						}},
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "Float"}},
						}},
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "Int"}},
						}},
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "String"}},
						}},
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "ID"}},
						}},
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "__Schema"}},
						}},
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "__Type"}},
						}},
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "__Field"}},
						}},
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "__InputValue"}},
						}},
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "__EnumValue"}},
						}},
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "__TypeKind"}},
						}},
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "__Directive"}},
						}},
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "__DirectiveLocation"}},
						}},
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "Query"}},
						}},
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "Mutation"}},
						}},
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "Foo"}},
						}},
					}}},
					{key: "directives", value: valueExpectations{list: []valueExpectations{
						// TODO(someday): Order here doesn't matter, but at the moment,
						// the implementation will always return this order.
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "deprecated"}},
							{key: "locations", value: valueExpectations{list: []valueExpectations{
								{scalar: "FIELD_DEFINITION"},
								{scalar: "ENUM_VALUE"},
							}}},
							{key: "args", value: valueExpectations{list: []valueExpectations{
								{object: []fieldExpectations{
									{key: "name", value: valueExpectations{scalar: "reason"}},
									{key: "type", value: valueExpectations{object: []fieldExpectations{
										{key: "kind", value: valueExpectations{scalar: "SCALAR"}},
										{key: "name", value: valueExpectations{scalar: "String"}},
									}}},
								}},
							}}},
						}},
					}}},
				}}},
			},
		},
		{
			name: "Typename",
			schema: `
				type Query {
					myObject: MyType!
				}

				type MyType {
					bar: String
				}
			`,
			request: Request{
				Query: `{
					__typename
					myObject { __typename }
				}`,
			},
			want: []fieldExpectations{
				{key: "__typename", value: valueExpectations{scalar: "Query"}},
				{key: "myObject", value: valueExpectations{object: []fieldExpectations{
					{key: "__typename", value: valueExpectations{scalar: "MyType"}},
				}}},
			},
		},
		{
			name: "Deprecated/Field/DefaultReason",
			schema: `
				type Query {
					foo: String @deprecated
				}
			`,
			request: Request{
				Query: `{
					__type(name: "Query") {
						fields(includeDeprecated: true) {
							name
							isDeprecated
							deprecationReason
						}
					}
				}`,
			},
			want: []fieldExpectations{
				{key: "__type", value: valueExpectations{object: []fieldExpectations{
					{key: "fields", value: valueExpectations{list: []valueExpectations{
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "foo"}},
							{key: "isDeprecated", value: valueExpectations{scalar: "true"}},
							{key: "deprecationReason", value: valueExpectations{scalar: "No longer supported"}},
						}},
					}}},
				}}},
			},
		},
		{
			name: "Deprecated/Field/CustomReason",
			schema: `
				type Query {
					foo: String @deprecated(reason: "bar")
				}
			`,
			request: Request{
				Query: `{
					__type(name: "Query") {
						fields(includeDeprecated: true) {
							name
							isDeprecated
							deprecationReason
						}
					}
				}`,
			},
			want: []fieldExpectations{
				{key: "__type", value: valueExpectations{object: []fieldExpectations{
					{key: "fields", value: valueExpectations{list: []valueExpectations{
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "foo"}},
							{key: "isDeprecated", value: valueExpectations{scalar: "true"}},
							{key: "deprecationReason", value: valueExpectations{scalar: "bar"}},
						}},
					}}},
				}}},
			},
		},
		{
			name: "Deprecated/Field/HideFromList",
			schema: `
				type Query {
					foo: String @deprecated
				}
			`,
			request: Request{
				Query: `{
					__type(name: "Query") {
						fields {
							name
						}
					}
				}`,
			},
			want: []fieldExpectations{
				{key: "__type", value: valueExpectations{object: []fieldExpectations{
					{key: "fields", value: valueExpectations{list: []valueExpectations{}}},
				}}},
			},
		},
		{
			name: "Deprecated/Enum/DefaultReason",
			schema: `
				type Query {
					foo: String
				}

				enum MyEnum {
					FOO @deprecated
				}
			`,
			request: Request{
				Query: `{
					__type(name: "MyEnum") {
						enumValues(includeDeprecated: true) {
							name
							isDeprecated
							deprecationReason
						}
					}
				}`,
			},
			want: []fieldExpectations{
				{key: "__type", value: valueExpectations{object: []fieldExpectations{
					{key: "enumValues", value: valueExpectations{list: []valueExpectations{
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "FOO"}},
							{key: "isDeprecated", value: valueExpectations{scalar: "true"}},
							{key: "deprecationReason", value: valueExpectations{scalar: "No longer supported"}},
						}},
					}}},
				}}},
			},
		},
		{
			name: "Deprecated/Enum/CustomReason",
			schema: `
				type Query {
					foo: String
				}

				enum MyEnum {
					FOO @deprecated(reason: "bar")
				}
			`,
			request: Request{
				Query: `{
					__type(name: "MyEnum") {
						enumValues(includeDeprecated: true) {
							name
							isDeprecated
							deprecationReason
						}
					}
				}`,
			},
			want: []fieldExpectations{
				{key: "__type", value: valueExpectations{object: []fieldExpectations{
					{key: "enumValues", value: valueExpectations{list: []valueExpectations{
						{object: []fieldExpectations{
							{key: "name", value: valueExpectations{scalar: "FOO"}},
							{key: "isDeprecated", value: valueExpectations{scalar: "true"}},
							{key: "deprecationReason", value: valueExpectations{scalar: "bar"}},
						}},
					}}},
				}}},
			},
		},
		{
			name: "Deprecated/Enum/HideFromList",
			schema: `
				type Query {
					foo: String
				}

				enum MyEnum {
					FOO @deprecated
				}
			`,
			request: Request{
				Query: `{
					__type(name: "MyEnum") {
						enumValues {
							name
						}
					}
				}`,
			},
			want: []fieldExpectations{
				{key: "__type", value: valueExpectations{object: []fieldExpectations{
					{key: "enumValues", value: valueExpectations{list: []valueExpectations{}}},
				}}},
			},
		},
	}

	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schema, err := ParseSchema(test.schema, test.options)
			if err != nil {
				t.Fatal(err)
			}
			query := &introspectionQuery{
				Foo: "foo",
				MyObject: &introspectionMyType{
					Bar: "bar",
				},
			}
			var mutation interface{}
			if test.hasMutation {
				mutation = query
			}
			srv, err := NewServer(schema, query, mutation)
			if err != nil {
				t.Fatal(err)
			}
			resp := srv.Execute(ctx, test.request)
			for _, e := range resp.Errors {
				t.Errorf("Error: %s", e.Message)
			}
			expect := &valueExpectations{object: test.want}
			expect.check(t, resp.Data)
		})
	}
}

type introspectionQuery struct {
	Foo      string
	MyObject *introspectionMyType
}

type introspectionMyType struct {
	Bar string
}
