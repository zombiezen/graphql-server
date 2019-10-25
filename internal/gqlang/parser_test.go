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
			name:  "FieldAlias",
			input: " { myAlias: field } ",
			want: &Document{
				Definitions: []*Definition{
					{Operation: &Operation{
						Start: 1,
						Type:  Query,
						SelectionSet: &SelectionSet{
							LBrace: 1,
							Sel: []*Selection{
								{Field: &Field{
									Alias: &Name{Value: "myAlias", Start: 3},
									Name:  &Name{Value: "field", Start: 12},
								}},
							},
							RBrace: 18,
						},
					}},
				},
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
			name:  "EnumType",
			input: `enum Foo { BAR, "Last" BAZ }`,
			want: &Document{
				Definitions: []*Definition{
					{Type: &TypeDefinition{Enum: &EnumTypeDefinition{
						Keyword: 0,
						Name: &Name{
							Start: 5,
							Value: "Foo",
						},
						Values: &EnumValuesDefinition{
							LBrace: 9,
							Values: []*EnumValueDefinition{
								{
									Value: &Name{
										Start: 11,
										Value: "BAR",
									},
								},
								{
									Description: &Description{
										Start: 16,
										Raw:   `"Last"`,
									},
									Value: &Name{
										Start: 23,
										Value: "BAZ",
									},
								},
							},
							RBrace: 27,
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
		{
			name: "InputObjectTypeWithDefaultValues",
			input: `input Filter {
	includeCompleted: Boolean! = false
}
`,
			want: &Document{
				Definitions: []*Definition{
					{Type: &TypeDefinition{InputObject: &InputObjectTypeDefinition{
						Keyword: 0,
						Name:    &Name{Value: "Filter", Start: 6},
						Fields: &InputFieldsDefinition{
							LBrace: 13,
							Defs: []*InputValueDefinition{
								{
									Name:  &Name{Value: "includeCompleted", Start: 16},
									Colon: 32,
									Type: &TypeRef{NonNull: &NonNullType{
										Named: &Name{Value: "Boolean", Start: 34},
										Pos:   41,
									}},
									Default: &DefaultValue{
										Eq: 43,
										Value: &InputValue{Scalar: &ScalarValue{
											Start: 45,
											Type:  BooleanScalar,
											Raw:   "false",
										}},
									},
								},
							},
							RBrace: 51,
						},
					}}},
				},
			},
		},
		{
			name: "InputObjectLiteral",
			input: `{
	findDog(complex: { name: "Fido" })
}`,
			want: &Document{
				Definitions: []*Definition{
					{Operation: &Operation{
						Start: 0,
						Type:  Query,
						SelectionSet: &SelectionSet{
							LBrace: 0,
							Sel: []*Selection{
								{Field: &Field{
									Name: &Name{
										Start: 3,
										Value: "findDog",
									},
									Arguments: &Arguments{
										LParen: 10,
										Args: []*Argument{
											{
												Name: &Name{
													Start: 11,
													Value: "complex",
												},
												Colon: 18,
												Value: &InputValue{
													InputObject: &InputObjectValue{
														LBrace: 20,
														Fields: []*InputObjectField{
															{
																Name: &Name{
																	Start: 22,
																	Value: "name",
																},
																Colon: 26,
																Value: &InputValue{Scalar: &ScalarValue{
																	Start: 28,
																	Type:  StringScalar,
																	Raw:   `"Fido"`,
																}},
															},
														},
														RBrace: 35,
													},
												},
											},
										},
										RParen: 36,
									},
								}},
							},
							RBrace: 38,
						},
					}},
				},
			},
		},
		{
			name:  "EmptyListLiteral",
			input: `{ foo(list: []) }`,
			want: &Document{
				Definitions: []*Definition{
					{Operation: &Operation{
						Start: 0,
						Type:  Query,
						SelectionSet: &SelectionSet{
							LBrace: 0,
							Sel: []*Selection{
								{Field: &Field{
									Name: &Name{
										Start: 2,
										Value: "foo",
									},
									Arguments: &Arguments{
										LParen: 5,
										Args: []*Argument{
											{
												Name: &Name{
													Start: 6,
													Value: "list",
												},
												Colon: 10,
												Value: &InputValue{
													List: &ListValue{
														LBracket: 12,
														RBracket: 13,
													},
												},
											},
										},
										RParen: 14,
									},
								}},
							},
							RBrace: 16,
						},
					}},
				},
			},
		},
		{
			name:  "ListLiteral",
			input: `{ foo(list: [123, 456]) }`,
			want: &Document{
				Definitions: []*Definition{
					{Operation: &Operation{
						Start: 0,
						Type:  Query,
						SelectionSet: &SelectionSet{
							LBrace: 0,
							Sel: []*Selection{
								{Field: &Field{
									Name: &Name{
										Start: 2,
										Value: "foo",
									},
									Arguments: &Arguments{
										LParen: 5,
										Args: []*Argument{
											{
												Name: &Name{
													Start: 6,
													Value: "list",
												},
												Colon: 10,
												Value: &InputValue{
													List: &ListValue{
														LBracket: 12,
														Values: []*InputValue{
															{Scalar: &ScalarValue{
																Start: 13,
																Type:  IntScalar,
																Raw:   "123",
															}},
															{Scalar: &ScalarValue{
																Start: 18,
																Type:  IntScalar,
																Raw:   "456",
															}},
														},
														RBracket: 21,
													},
												},
											},
										},
										RParen: 22,
									},
								}},
							},
							RBrace: 24,
						},
					}},
				},
			},
		},
		{
			name:  "UnterminatedString/Block",
			input: `"""foo`,
			wantErrs: posSet{
				6: {},
			},
		},
		{
			name:  "UnterminatedString/JustBlockStart",
			input: `"""`,
			wantErrs: posSet{
				3: {},
			},
		},
		{
			name:  "UnterminatedString/BlockWithEscape",
			input: `"""foo\"""`,
			wantErrs: posSet{
				10: {},
			},
		},
		{
			name:  "UnterminatedString/LineBreakEmpty",
			input: "\"\nscalar Bar",
			wantErrs: posSet{
				1: {},
			},
		},
		{
			name:  "UnterminatedString/LineBreak",
			input: "\"foo\nscalar Bar",
			wantErrs: posSet{
				4: {},
			},
		},
		{
			name:  "StringEscape/EndsWithDoubleBackslash",
			input: `"foo\\" scalar Bar`,
			want: &Document{
				Definitions: []*Definition{
					{Type: &TypeDefinition{Scalar: &ScalarTypeDefinition{
						Description: &Description{
							Start: 0,
							Raw:   `"foo\\"`,
						},
						Keyword: 8,
						Name: &Name{
							Start: 15,
							Value: "Bar",
						},
					}}},
				},
			},
		},
		{
			name:  "StringEscape/BadSequence",
			input: `"foo\hbar" scalar Bar`,
			wantErrs: posSet{
				5: {},
			},
		},
		{
			name:  "StringEscape/HexSequence",
			input: `"foo\ubeef" scalar Bar`,
			want: &Document{
				Definitions: []*Definition{
					{Type: &TypeDefinition{Scalar: &ScalarTypeDefinition{
						Description: &Description{
							Start: 0,
							Raw:   `"foo\ubeef"`,
						},
						Keyword: 12,
						Name: &Name{
							Start: 19,
							Value: "Bar",
						},
					}}},
				},
			},
		},
		{
			name:  "StringEscape/HexSequenceAtEnd",
			input: `"foo\u" scalar Bar`,
			wantErrs: posSet{
				6: {},
			},
		},
		{
			name:  "StringEscape/BadHexSequence",
			input: `"foo\u0xyz" scalar Bar`,
			wantErrs: posSet{
				7: {},
				8: {},
				9: {},
			},
		},
		{
			name:  "StringEscape/DoubleQuoteAtEnd",
			input: "\"foo\\\"\n scalar Bar",
			wantErrs: posSet{
				6: {},
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
	benches := []struct {
		name  string
		input string
	}{
		{
			name:  "SmallOperation",
			input: `{ name }`,
		},
		{
			name: "Operation",
			input: `
query ProjectList {
	inbox {
		id
		items(includeCompleted: true) {
			id
			name
			labels { name }
		}
	}

	projects {
		id
		name
	}
}
`,
		},
		{
			name: "Schema",
			input: `
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
    includeCompleted: Boolean = false
  ): [Item]
  name: String!
}

type Query {
  """The inbox project"""
  inbox(date: String): Project

  """List of all active projects"""
  projects: [Project]
}
`,
		},
	}
	for _, bench := range benches {
		b.Run(bench.name, func(b *testing.B) {
			b.SetBytes(int64(len(bench.input)))
			for i := 0; i < b.N; i++ {
				if _, errs := Parse(bench.input); len(errs) > 0 {
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
}
