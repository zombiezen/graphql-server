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
