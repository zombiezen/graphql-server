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

package gqlang

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

type posSet map[Pos]struct{}

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     *Document
		wantErrs posSet
	}{
		{
			name:  "Empty",
			input: "",
			want:  &Document{},
		},
		{
			name:  "OverviewQuery",
			input: "query MyQuery { user(id: 4) { firstName, lastName } }\n",
			want: &Document{
				Definitions: []*Definition{
					{Operation: &Operation{
						Start: 0,
						Type:  Query,
						Name:  &Name{Value: "MyQuery", Start: 6},
						SelectionSet: &SelectionSet{
							LBrace: 14,
							RBrace: 52,
							Sel: []*Selection{
								{Field: &Field{
									Name: &Name{Value: "user", Start: 16},
									Arguments: &Arguments{
										LParen: 20,
										RParen: 26,
										Args: []*Argument{
											{
												Name:  &Name{Value: "id", Start: 21},
												Colon: 23,
												Value: &InputValue{Scalar: &ScalarValue{
													Start: 25,
													Type:  IntScalar,
													Raw:   "4",
												}},
											},
										},
									},
									SelectionSet: &SelectionSet{
										LBrace: 28,
										RBrace: 50,
										Sel: []*Selection{
											{Field: &Field{
												Name: &Name{Value: "firstName", Start: 30},
											}},
											{Field: &Field{
												Name: &Name{Value: "lastName", Start: 41},
											}},
										},
									},
								}},
							},
						},
					}},
				},
			},
		},
		{
			name:  "Shorthand",
			input: " { field } ",
			want: &Document{
				Definitions: []*Definition{
					{Operation: &Operation{
						Start: 1,
						Type:  Query,
						SelectionSet: &SelectionSet{
							LBrace: 1,
							RBrace: 9,
							Sel: []*Selection{
								{Field: &Field{
									Name: &Name{Value: "field", Start: 3},
								}},
							},
						},
					}},
				},
			},
		},
		{
			name:  "MissingClosingBrace",
			input: " { field  ",
			want: &Document{
				Definitions: []*Definition{
					{Operation: &Operation{
						Start: 1,
						Type:  Query,
						SelectionSet: &SelectionSet{
							LBrace: 1,
							RBrace: -1,
							Sel: []*Selection{
								{Field: &Field{
									Name: &Name{Value: "field", Start: 3},
								}},
							},
						},
					}},
				},
			},
			wantErrs: posSet{
				10: {},
			},
		},
		{
			name:  "EmptyOperation",
			input: " { } ",
			want: &Document{
				Definitions: []*Definition{
					{Operation: &Operation{
						Start: 1,
						Type:  Query,
						SelectionSet: &SelectionSet{
							LBrace: 1,
							RBrace: 3,
						},
					}},
				},
			},
			wantErrs: posSet{
				3: {},
			},
		},
		{
			name:  "EmptyArgs",
			input: " { foo() } ",
			want: &Document{
				Definitions: []*Definition{
					{Operation: &Operation{
						Start: 1,
						Type:  Query,
						SelectionSet: &SelectionSet{
							LBrace: 1,
							RBrace: 9,
							Sel: []*Selection{
								{Field: &Field{
									Name: &Name{Value: "foo", Start: 3},
									Arguments: &Arguments{
										LParen: 6,
										RParen: 7,
										Args:   nil,
									},
								}},
							},
						},
					}},
				},
			},
			wantErrs: posSet{
				7: {},
			},
		},
		{
			name:  "Variables",
			input: "query getProfile($devicePicSize: Int) { user { profilePic(size: $devicePicSize) } }",
			want: &Document{
				Definitions: []*Definition{
					{Operation: &Operation{
						Start: 0,
						Type:  Query,
						Name:  &Name{Value: "getProfile", Start: 6},
						VariableDefinitions: &VariableDefinitions{
							LParen: 16,
							RParen: 36,
							Defs: []*VariableDefinition{
								{
									Var: &Variable{
										Dollar: 17,
										Name:   &Name{Value: "devicePicSize", Start: 18},
									},
									Colon: 31,
									Type: &TypeRef{
										Named: &Name{Value: "Int", Start: 33},
									},
								},
							},
						},
						SelectionSet: &SelectionSet{
							LBrace: 38,
							RBrace: 82,
							Sel: []*Selection{
								{Field: &Field{
									Name: &Name{Value: "user", Start: 40},
									SelectionSet: &SelectionSet{
										LBrace: 45,
										RBrace: 80,
										Sel: []*Selection{
											{Field: &Field{
												Name: &Name{Value: "profilePic", Start: 47},
												Arguments: &Arguments{
													LParen: 57,
													RParen: 78,
													Args: []*Argument{
														{
															Name:  &Name{Value: "size", Start: 58},
															Colon: 62,
															Value: &InputValue{VariableRef: &Variable{
																Dollar: 64,
																Name:   &Name{Value: "devicePicSize", Start: 65},
															}},
														},
													},
												},
											}},
										},
									},
								}},
							},
						},
					}},
				},
			},
		},
		{
			name: "ScalarType",
			input: `"""
An RFC 3339 datetime
"""
scalar DateTime
`,
			want: &Document{
				Definitions: []*Definition{
					{Type: &TypeDefinition{Scalar: &ScalarTypeDefinition{
						Description: &Description{
							Start: 0,
							Raw:   "\"\"\"\nAn RFC 3339 datetime\n\"\"\"",
						},
						Keyword: 29,
						Name:    &Name{Value: "DateTime", Start: 36},
					}}},
				},
			},
		},
		{
			name: "ObjectType",
			input: `"A single task in a project"
type Item {
	id: ID!
	completedAt: DateTime
}
`,
			want: &Document{
				Definitions: []*Definition{
					{Type: &TypeDefinition{Object: &ObjectTypeDefinition{
						Description: &Description{
							Start: 0,
							Raw:   `"A single task in a project"`,
						},
						Keyword: 29,
						Name:    &Name{Value: "Item", Start: 34},
						Fields: &FieldsDefinition{
							LBrace: 39,
							RBrace: 73,
							Defs: []*FieldDefinition{
								{
									Name:  &Name{Value: "id", Start: 42},
									Colon: 44,
									Type: &TypeRef{
										NonNull: &NonNullType{
											Named: &Name{Value: "ID", Start: 46},
											Pos:   48,
										},
									},
								},
								{
									Name:  &Name{Value: "completedAt", Start: 51},
									Colon: 62,
									Type: &TypeRef{
										Named: &Name{Value: "DateTime", Start: 64},
									},
								},
							},
						},
					}}},
				},
			},
		},
		{
			name: "ObjectTypeWithFieldArgs",
			input: `type Query {
	project(id: ID!): Project
}
`,
			want: &Document{
				Definitions: []*Definition{
					{Type: &TypeDefinition{Object: &ObjectTypeDefinition{
						Keyword: 0,
						Name:    &Name{Value: "Query", Start: 5},
						Fields: &FieldsDefinition{
							LBrace: 11,
							Defs: []*FieldDefinition{
								{
									Name: &Name{Value: "project", Start: 14},
									Args: &ArgumentsDefinition{
										LParen: 21,
										Args: []*InputValueDefinition{
											{
												Name:  &Name{Value: "id", Start: 22},
												Colon: 24,
												Type: &TypeRef{NonNull: &NonNullType{
													Named: &Name{Value: "ID", Start: 26},
													Pos:   28,
												}},
											},
										},
										RParen: 29,
									},
									Colon: 30,
									Type: &TypeRef{
										Named: &Name{Value: "Project", Start: 32},
									},
								},
							},
							RBrace: 40,
						},
					}}},
				},
			},
		},
		{
			name: "InputObjectType",
			input: `"Parameters for creating an item"
input ItemInput {
	projectId: ID
	text: String!
}
`,
			want: &Document{
				Definitions: []*Definition{
					{Type: &TypeDefinition{InputObject: &InputObjectTypeDefinition{
						Description: &Description{
							Start: 0,
							Raw:   `"Parameters for creating an item"`,
						},
						Keyword: 34,
						Name:    &Name{Value: "ItemInput", Start: 40},
						Fields: &InputFieldsDefinition{
							LBrace: 50,
							RBrace: 82,
							Defs: []*InputValueDefinition{
								{
									Name:  &Name{Value: "projectId", Start: 53},
									Colon: 62,
									Type: &TypeRef{
										Named: &Name{Value: "ID", Start: 64},
									},
								},
								{
									Name:  &Name{Value: "text", Start: 68},
									Colon: 72,
									Type: &TypeRef{
										NonNull: &NonNullType{
											Named: &Name{Value: "String", Start: 74},
											Pos:   80,
										},
									},
								},
							},
						},
					}}},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotErrs := make(map[Pos]bool)
			for pos := range test.wantErrs {
				gotErrs[pos] = false
			}
			got, errs := Parse(test.input)
			if len(errs) > 0 {
				t.Log("errors:")
				for _, err := range errs {
					if position, ok := ErrorPosition(err); ok {
						t.Logf("%v: %v", position, err)
					} else {
						t.Log(err)
					}
				}
				for _, err := range errs {
					pos, ok := ErrorPos(err)
					if !ok {
						continue
					}
					if _, expected := gotErrs[pos]; !expected {
						t.Errorf("error at unexpected position %v (offset %v)", pos.ToPosition(test.input), pos)
						continue
					}
					gotErrs[pos] = true
				}
			}
			for pos, ok := range gotErrs {
				if !ok {
					t.Errorf("did not get error at %v (offset %v)", pos.ToPosition(test.input), pos)
				}
			}
			diff := cmp.Diff(test.want, got, cmpopts.EquateEmpty())
			if diff != "" {
				t.Errorf("-want +got:\n%s", diff)
			}
		})
	}
}

func BenchmarkParse(b *testing.B) {
	b.Run("Schema", func(b *testing.B) {
		const input = `
"""
The DateTime scalar type represents a DateTime. The DateTime is serialized as an RFC 3339 quoted string
"""
scalar DateTime

"""A single task in a project"""
type Item {
  completed: Boolean!
  completedAt: DateTime
  createdAt: DateTime!
  id: ID!
  text: String!
}

"""Fields for creating an Item"""
input ItemInput {
  projectId: ID!
  text: String!
}

type Mutation {
  """Create a new item"""
  createItem(input: ItemInput!): Item

  """Create a new project"""
  createProject(name: String!): Project

  """Delete a project"""
  deleteProject(id: ID!): ID
}

"""A group of tasks"""
type Project {
  createdAt: DateTime!
  id: ID!
  items(
    """Whether to include completed items in the list"""
    includeCompleted: Boolean
  ): [Item]
  name: String!
}

type Query {
  """The inbox project"""
  inbox(date: String): Project

  """List of all active projects"""
  projects: [Project]
}
`
		b.SetBytes(int64(len(input)))
		for i := 0; i < b.N; i++ {
			if _, errs := Parse(input); len(errs) > 0 {
				for _, err := range errs {
					if p, ok := ErrorPosition(err); ok {
						b.Errorf("%v: %v", p, err)
					} else {
						b.Error(err)
					}
				}
				b.FailNow()
			}
		}
	})
}
