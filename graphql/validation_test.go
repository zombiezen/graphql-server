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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"zombiezen.com/go/graphql-server/internal/gqlang"
)

func TestValidateRequest(t *testing.T) {
	// Schema from https://graphql.github.io/graphql-spec/June2018/#example-26a9d
	// TODO(soon): Fill out all missing parts.
	const schemaSource = `
		type Query {
			dog: Dog
		}

		scalar DogCommand

		type Dog {
			name: String!
			nickname: String
			barkVolume: Int
			doesKnowCommand(dogCommand: DogCommand): Boolean!
			isHousetrained(atOtherHomes: Boolean): Boolean!
			owner: Human
		}

		type Human {
			name: String!
		}

		type Mutation {
			mutateDog: ID
		}`
	schema, err := ParseSchema(schemaSource)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name               string
		request            string
		wantErrorLocations map[Location]struct{}
	}{
		{
			name: "OnlyExecutable/Valid",
			request: `
				query getDogName {
					dog {
						name
						nickname
					}
				}`,
			wantErrorLocations: nil,
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-12752
			name: "OnlyExecutable/Invalid",
			request: `
				query getDogName {
					dog {
						name
						color
					}
				}

				type Dog {
					color: String
				}`,
			wantErrorLocations: map[Location]struct{}{
				{9, 33}: {},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-069e1
			name: "NameUniqueness/Valid",
			request: `
				query getDogName {
					dog {
						name
					}
				}

				query getOwnerName {
					dog {
						owner {
							name
						}
					}
				}`,
			wantErrorLocations: nil,
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-5e409
			name: "NameUniqueness/Invalid",
			request: `
				query getName {
					dog {
						name
					}
				}

				query getName {
					dog {
						owner {
							name
						}
					}
				}`,
			wantErrorLocations: map[Location]struct{}{
				{2, 39}: {},
				{8, 39}: {},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-77c2e
			name: "NameUniqueness/InvalidDifferentTypes",
			request: `
				query dogOperation {
					dog {
						name
					}
				}

				mutation dogOperation {
					mutateDog {
						id
					}
				}`,
			wantErrorLocations: map[Location]struct{}{
				{2, 39}: {},
				{8, 42}: {},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-be853
			name: "LoneAnonymousOperation/Valid",
			request: `
				{
					dog {
						name
					}
				}`,
			wantErrorLocations: nil,
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-44b85
			name: "LoneAnonymousOperation/Invalid",
			request: `
				{
					dog {
						name
					}
				}

				query getName {
					dog {
						owner {
							name
						}
					}
				}`,
			wantErrorLocations: map[Location]struct{}{
				{2, 33}: {},
			},
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-48706
			name: "FieldSelection/NotDefined",
			request: `
				{
					dog {
						meowVolume
					}
				}`,
			wantErrorLocations: map[Location]struct{}{
				{4, 49}: {},
			},
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-e23c5
			name: "FieldSelection/Leaf/ScalarValid",
			request: `
				{
					dog {
						barkVolume
					}
				}`,
			wantErrorLocations: nil,
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-13b69
			name: "FieldSelection/Leaf/ScalarWithSelectionSet",
			request: `
				{
					dog {
						barkVolume {
							sinceWhen
						}
					}
				}`,
			wantErrorLocations: map[Location]struct{}{
				{4, 60}: {},
			},
		},
		{
			name: "FieldSelection/Leaf/Object",
			request: `
				{
					dog
				}`,
			wantErrorLocations: map[Location]struct{}{
				{3, 44}: {},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, errs := gqlang.Parse(test.request)
			if len(errs) > 0 {
				t.Fatal(errs)
			}
			errs = schema.validateRequest(test.request, doc)
			gotErrorLocations := make(map[Location]struct{})
			for _, err := range errs {
				for _, loc := range toResponseError(err).Locations {
					gotErrorLocations[loc] = struct{}{}
				}
			}
			diff := cmp.Diff(test.wantErrorLocations, gotErrorLocations, cmpopts.EquateEmpty())
			if diff != "" {
				t.Errorf("error locations (-want +got):\n%s", diff)
			}
		})
	}
}
