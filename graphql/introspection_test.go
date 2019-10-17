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
	tests := []struct {
		name        string
		schema      string
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
	}

	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schema, err := ParseSchema(test.schema)
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
