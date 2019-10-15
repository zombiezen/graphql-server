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
	t.Skip("Introspection not implemented")

	tests := []struct {
		name    string
		schema  string
		request Request
		want    []fieldExpectations
	}{
		{
			name: "Basic/Type",
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
							args
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
	}

	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schema, err := ParseSchema(test.schema)
			if err != nil {
				t.Fatal(err)
			}
			srv, err := NewServer(schema, new(introspectionQuery), nil)
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
	Foo string
}