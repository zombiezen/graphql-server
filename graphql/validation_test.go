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
)

func TestValidate(t *testing.T) {
	t.Parallel()
	// Schema from https://graphql.github.io/graphql-spec/June2018/#example-26a9d,
	// https://graphql.github.io/graphql-spec/June2018/#example-1891c, and
	// https://graphql.github.io/graphql-spec/June2018/#example-f3185.
	// TODO(soon): Fill out all missing parts.
	const schemaSource = `
		type Query {
			dog: Dog
			pack: [Dog!]!
			arguments: Arguments
			findDog(complex: ComplexInput): Dog
			dogById(id: ID!): Dog
			booleanList(booleanListArg: [Boolean!]): Boolean
			catOrDog: CatOrDog
		}

		enum DogCommand { SIT, DOWN, HEEL }

		scalar CustomScalar

		type Dog {
			name: String!
			nickname: String
			barkVolume: Int
			doesKnowCommand(dogCommand: DogCommand): Boolean!
			isHousetrained(atOtherHomes: Boolean): Boolean!
			owner: Human
		}

		type Alien {
			name: String!
			homePlanet: String
		}

		type Human {
			name: String!
			# TODO(someday): Type should be [Pet!]
			pets: [Dog!]
		}

		enum CatCommand { JUMP }

		type Cat {
			name: String!
			nickname: String
			doesKnowCommand(catCommand: CatCommand!): Boolean!
			meowVolume: Int
		}

		union CatOrDog = Cat | Dog
		union DogOrHuman = Dog | Human
		union HumanOrAlien = Human | Alien

		type Mutation {
			mutateDog: ID
		}

		type Arguments {
			multipleReqs(x: Int!, y: Int!): Int!
			booleanArgField(booleanArg: Boolean): Boolean
			floatArgField(floatArg: Float): Float
			intArgField(intArg: Int): Int
			nonNullBooleanArgField(nonNullBooleanArg: Boolean!): Boolean!
			booleanListArgField(booleanListArg: [Boolean]!): [Boolean]
			optionalNonNullBooleanArgField(optionalBooleanArg: Boolean! = false): Boolean!
			customScalar(arg: CustomScalar): CustomScalar
		}

		input ComplexInput { name: String!, owner: String }`
	schema, err := ParseSchema(schemaSource, nil)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		request    string
		wantErrors []*ResponseError
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
			wantErrors: nil,
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
			wantErrors: []*ResponseError{
				{
					Locations: []Location{{9, 33}},
				},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-069e1
			name: "OperationNameUniqueness/Valid",
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
			wantErrors: nil,
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-5e409
			name: "OperationNameUniqueness/Invalid",
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
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{2, 39},
						{8, 39},
					},
				},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-77c2e
			name: "OperationNameUniqueness/InvalidDifferentTypes",
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
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{2, 39},
						{8, 42},
					},
				},
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
			wantErrors: nil,
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
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{2, 33},
					},
				},
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
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 49},
					},
					Path: []PathSegment{
						{Field: "dog"},
						{Field: "meowVolume"},
					},
				},
			},
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-48706
			name: "FieldSelection/NotDefinedWithAlias",
			request: `
				{
					dog {
						foo: meowVolume
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 54},
					},
					Path: []PathSegment{
						{Field: "dog"},
						{Field: "foo"},
					},
				},
			},
		},
		{
			// Inspired by http://spec.graphql.org/June2018/#example-245fa
			name: "FieldSelection/Union/IndirectFieldSelection",
			request: `
				{
					catOrDog {
						__typename
						... on Dog {
							barkVolume
						}
					}
				}`,
			wantErrors: nil,
		},
		{
			// Inspired by http://spec.graphql.org/June2018/#example-245fa
			name: "FieldSelection/Union/DirectFieldSelection",
			request: `
				{
					catOrDog {
						__typename
						barkVolume
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{5, 49},
					},
					Path: []PathSegment{
						{Field: "catOrDog"},
						{Field: "barkVolume"},
					},
				},
			},
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-4e10c
			name: "FieldSelection/Merging/IdenticalFields",
			request: `
				{
					dog {
						name
						name
					}
				}`,
			wantErrors: nil,
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-4e10c
			name: "FieldSelection/Merging/IdenticalAliasesAndFields",
			request: `
				{
					dog {
						otherName: name
						otherName: name
					}
				}`,
			wantErrors: nil,
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-4e10c
			name: "FieldSelection/Merging/ConflictingBecauseAlias",
			request: `
				{
					dog {
						name: nickname
						name
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 49},
						{5, 49},
					},
					Path: []PathSegment{
						{Field: "dog"},
						{Field: "name"},
					},
				},
			},
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-b6369
			name: "FieldSelection/Merging/IdenticalFieldsWithIdenticalArgs",
			request: `
				{
					dog {
						doesKnowCommand(dogCommand: SIT)
						doesKnowCommand(dogCommand: SIT)
					}
				}`,
			wantErrors: nil,
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-b6369
			name: "FieldSelection/Merging/IdenticalFieldsWithIdenticalValues",
			request: `
				query($dogCommand: DogCommand!) {
					dog {
						doesKnowCommand(dogCommand: $dogCommand)
						doesKnowCommand(dogCommand: $dogCommand)
					}
				}`,
			wantErrors: nil,
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-00fbf
			name: "FieldSelection/Merging/ConflictingArgsOnValues",
			request: `
				{
					dog {
						doesKnowCommand(dogCommand: SIT)
						doesKnowCommand(dogCommand: HEEL)
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 49},
						{5, 49},
					},
					Path: []PathSegment{
						{Field: "dog"},
						{Field: "doesKnowCommand"},
					},
				},
			},
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-00fbf
			name: "FieldSelection/Merging/ConflictingArgsValueAndVar",
			request: `
				query($dogCommand: DogCommand!) {
					dog {
						doesKnowCommand(dogCommand: SIT)
						doesKnowCommand(dogCommand: $dogCommand)
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 49},
						{5, 49},
					},
					Path: []PathSegment{
						{Field: "dog"},
						{Field: "doesKnowCommand"},
					},
				},
			},
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-00fbf
			name: "FieldSelection/Merging/ConflictingArgsWithVars",
			request: `
				query($varOne: DogCommand!, $varTwo: DogCommand!) {
					dog {
						doesKnowCommand(dogCommand: $varOne)
						doesKnowCommand(dogCommand: $varTwo)
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 49},
						{5, 49},
					},
					Path: []PathSegment{
						{Field: "dog"},
						{Field: "doesKnowCommand"},
					},
				},
			},
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-00fbf
			name: "FieldSelection/Merging/DifferingArgs",
			request: `
				{
					dog {
						doesKnowCommand(dogCommand: SIT)
						doesKnowCommand
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 49},
						{5, 49},
					},
					Path: []PathSegment{
						{Field: "dog"},
						{Field: "doesKnowCommand"},
					},
				},
			},
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-77852
			name: "FieldSelection/Merging/AcrossSets/DistinctFields",
			request: `
				{
					dog {
						name
					}
					dog {
						nickname
					}
				}`,
			wantErrors: nil,
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-77852
			name: "FieldSelection/Merging/AcrossSets/ConflictingArgsOnValues",
			request: `
				{
					dog {
						doesKnowCommand(dogCommand: SIT)
					}
					dog {
						doesKnowCommand(dogCommand: HEEL)
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 49},
						{7, 49},
					},
					Path: []PathSegment{
						{Field: "dog"},
						{Field: "doesKnowCommand"},
					},
				},
			},
		},
		{
			name: "FieldSelection/Merging/AcrossFragments/DistinctFields",
			request: `
				{
					dog {
						... {
							name
						}
						... {
							nickname
						}
					}
				}`,
			wantErrors: nil,
		},
		{
			name: "FieldSelection/Merging/AcrossFragments/ConflictingArgsOnValues",
			request: `
				{
					dog {
						... {
							doesKnowCommand(dogCommand: SIT)
						}
						... {
							doesKnowCommand(dogCommand: HEEL)
						}
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{5, 57},
						{8, 57},
					},
					Path: []PathSegment{
						{Field: "dog"},
						{Field: "doesKnowCommand"},
					},
				},
			},
		},
		{
			name: "FieldSelection/Merging/AcrossFragments/ConflictingArgsOnValuesWithNamed",
			request: `
				{
					dog {
						doesKnowCommand(dogCommand: SIT)
						...heel
					}
				}

				fragment heel on Dog {
					doesKnowCommand(dogCommand: HEEL)
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 49},
						{10, 41},
					},
					Path: []PathSegment{
						{Field: "dog"},
						{Field: "doesKnowCommand"},
					},
				},
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
			wantErrors: nil,
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
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 60},
					},
					Path: []PathSegment{
						{Field: "dog"},
						{Field: "barkVolume"},
					},
				},
			},
		},
		{
			name: "FieldSelection/Leaf/Object",
			request: `
				{
					dog
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{3, 44},
					},
					Path: []PathSegment{
						{Field: "dog"},
					},
				},
			},
		},
		{
			name: "FieldSelection/Leaf/ListOfObjects",
			request: `
				{
					pack {
						name
					}
				}`,
			wantErrors: nil,
		},
		{
			name: "FieldSelection/Leaf/ListOfObjectsWithoutSelectionSet",
			request: `
				{
					pack
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{3, 45},
					},
					Path: []PathSegment{
						{Field: "pack"},
					},
				},
			},
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-760cb
			name: "Arguments/Names/Valid",
			request: `
				{
					dog {
						doesKnowCommand(dogCommand: SIT)
					}
				}`,
			wantErrors: nil,
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-d5639
			name: "Arguments/Names/Invalid",
			request: `
				{
					dog {
						doesKnowCommand(command: CLEAN_UP_HOUSE)
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 65},
					},
					Path: []PathSegment{
						{Field: "dog"},
						{Field: "doesKnowCommand"},
					},
				},
			},
		},
		{
			name: "Arguments/Unique",
			request: `
				{
					dog {
						doesKnowCommand(dogCommand: SIT, dogCommand: DOWN)
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 65},
						{4, 82},
					},
					Path: []PathSegment{
						{Field: "dog"},
						{Field: "doesKnowCommand"},
					},
				},
			},
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-503bd
			name: "Arguments/Required/Valid",
			request: `
				query goodBooleanArg {
					arguments {
						booleanArgField(booleanArg: true)
					}
				}

				query goodNonNullArg {
					arguments {
						nonNullBooleanArgField(nonNullBooleanArg: true)
					}
				}`,
			wantErrors: nil,
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-1f1d2
			name: "Arguments/Required/OmitNullable",
			request: `
				{
					arguments {
						booleanArgField
					}
				}`,
			wantErrors: nil,
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-f12a1
			name: "Arguments/Required/MissingRequiredArg",
			request: `
				{
					arguments {
						nonNullBooleanArgField
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 71},
					},
					Path: []PathSegment{
						{Field: "arguments"},
						{Field: "nonNullBooleanArgField"},
					},
				},
			},
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-0bc81
			name: "Arguments/Required/NullForRequiredArg",
			request: `
				{
					arguments {
						nonNullBooleanArgField(nonNullBooleanArg: null)
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 91},
					},
					Path: []PathSegment{
						{Field: "arguments"},
						{Field: "nonNullBooleanArgField"},
					},
				},
			},
		},
		{
			name: "Arguments/Required/NullForNonNullableArg",
			request: `
				{
					arguments {
						optionalNonNullBooleanArgField(optionalBooleanArg: null)
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 100},
					},
					Path: []PathSegment{
						{Field: "arguments"},
						{Field: "optionalNonNullBooleanArgField"},
					},
				},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-3703b
			name: "Fragments/NameUniqueness/Valid",
			request: `
				{
					dog {
						...fragmentOne
						...fragmentTwo
					}
				}

				fragment fragmentOne on Dog {
					name
				}

				fragment fragmentTwo on Dog {
					owner {
						name
					}
				}
			`,
			wantErrors: nil,
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-2c3e3
			name: "Fragments/NameUniqueness/Invalid",
			request: `
				{
					dog {
						...fragmentOne
					}
				}

				fragment fragmentOne on Dog {
					name
				}

				fragment fragmentOne on Dog {
					owner {
						name
					}
				}
			`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{8, 42},
						{12, 42},
					},
				},
			},
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-1b2da
			name: "Fragments/SpreadTypeExistence/Valid",
			request: `
				fragment correctType on Dog {
					name
				}

				fragment inlineFragment on Dog {
					... on Dog {
						name
					}
				}

				# Use these in query to avoid errors.
				{ dog { ...correctType, ...inlineFragment } }
			`,
			wantErrors: nil,
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-463f6
			name: "Fragments/SpreadTypeExistence/Invalid",
			request: `
				fragment notOnExistingType on NotInSchema {
					name
				}

				fragment inlineNotExistingType on Dog {
					... on NotInSchema {
						name
					}
				}

				# Use these in query to avoid errors.
				{ dog { ...notOnExistingType, ...inlineNotExistingType } }
			`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{2, 63},
					},
					Path: []PathSegment{
						{Field: "dog"},
					},
				},
				{
					Locations: []Location{
						{7, 48},
					},
					Path: []PathSegment{
						{Field: "dog"},
					},
				},
			},
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-3c8d4
			name: "Fragments/CompositeTypes/Object",
			request: `
				fragment fragOnObject on Dog {
					name
				}

				# Use these in query to avoid errors.
				{ dog { ...fragOnObject } }
			`,
			wantErrors: nil,
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-4d5e5
			name: "Fragments/CompositeTypes/Invalid",
			request: `
				fragment fragOnScalar on Int {
					name
				}

				fragment inlineFragOnScalar on Dog {
					... on Boolean {
						somethingElse
					}
				}

				# Use these in query to avoid errors.
				{ dog { ...fragOnScalar, ...inlineFragOnScalar } }
			`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{2, 58},
					},
					Path: []PathSegment{
						{Field: "dog"},
					},
				},
				{
					Locations: []Location{
						{7, 48},
					},
					Path: []PathSegment{
						{Field: "dog"},
					},
				},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-9e1e3
			name: "Fragments/Unused",
			request: `
				fragment nameFragment on Dog { # unused
					name
				}

				{
					dog {
						name
					}
				}
			`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{2, 33},
					},
				},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-28421
			name: "FragmentSpreads/TargetUndefined",
			request: `
				{
					dog {
						...undefinedFragment
					}
				}
			`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 52},
					},
					Path: []PathSegment{
						{Field: "dog"},
					},
				},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-9ceb4
			name: "FragmentSpreads/Cycles/InfiniteSpread",
			request: `
				{
					dog {
						...nameFragment
					}
				}

				fragment nameFragment on Dog {
					name
					...barkVolumeFragment
				}

				fragment barkVolumeFragment on Dog {
					barkVolume
					...nameFragment
				}
			`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{15, 44}, // The final ...nameFragment
					},
				},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-6bbad
			name: "FragmentSpreads/Cycles/InfiniteRecursion",
			request: `
				{
					dog {
						...dogFragment
					}
				}

				fragment dogFragment on Dog {
					name
					owner {
						...ownerFragment
					}
				}

				fragment ownerFragment on Human {
					name
					pets {
						...dogFragment
					}
				}
			`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{18, 52}, // The final ...dogFragment
					},
				},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-0fc38
			name: "FragmentSpreads/Types/Object/Valid",
			request: `
				fragment dogFragment on Dog {
					... on Dog {
						barkVolume
					}
				}

				# Use fragment in query to avoid errors.
				{ dog { ...dogFragment } }
			`,
			wantErrors: nil,
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-4d411
			name: "FragmentSpreads/Types/Object/Invalid",
			request: `
				fragment catInDogFragmentInvalid on Dog {
					... on Cat {
						meowVolume
					}
				}

				# Use fragment in query to avoid errors.
				{ dog { ...catInDogFragmentInvalid } }
			`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{3, 48},
					},
					Path: []PathSegment{
						{Field: "dog"},
					},
				},
			},
		},
		{
			// http://spec.graphql.org/June2018/#example-41843
			name: "FragmentSpreads/Types/AbstractInObjectScope/Union",
			request: `
				fragment catOrDogNameFragment on CatOrDog {
					... on Cat {
						meowVolume
					}
				}

				fragment unionWithObjectFragment on Dog {
					...catOrDogNameFragment
				}

				# Use fragment in query to avoid errors.
				{ dog { ...unionWithObjectFragment } }
			`,
			wantErrors: nil,
		},
		{
			// http://spec.graphql.org/June2018/#example-85110
			name: "FragmentSpreads/Types/ObjectInAbstractScope/Union",
			request: `
				fragment catOrDogFragment on CatOrDog {
					... on Cat {
						meowVolume
					}
				}

				# Use fragment in query to avoid errors.
				{ catOrDog { ...catOrDogFragment } }
			`,
			wantErrors: nil,
		},
		{
			name: "FragmentSpreads/Types/ObjectInAbstractScope/UnionFail",
			request: `
				fragment catOrDogFragment on CatOrDog {
					... on Human {
						name
					}
				}

				# Use fragment in query to avoid errors.
				{ catOrDog { ...catOrDogFragment } }
			`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{3, 48},
					},
					Path: []PathSegment{
						{Field: "catOrDog"},
					},
				},
			},
		},
		{
			// Inspired by http://spec.graphql.org/June2018/#example-dc875
			name: "FragmentSpreads/Types/AbstractInAbstractScope/Union",
			request: `
				fragment catOrDogFragment on CatOrDog {
					...dogOrHumanFragment
				}

				fragment dogOrHumanFragment on DogOrHuman {
					... on Dog {
						barkVolume
					}
				}

				# Use fragment in query to avoid errors.
				{ catOrDog { ...catOrDogFragment } }
			`,
			wantErrors: nil,
		},
		{
			// Inspired by http://spec.graphql.org/June2018/#example-c9c63
			name: "FragmentSpreads/Types/AbstractInAbstractScope/UnionFail",
			request: `
				fragment catOrDogFragment on CatOrDog {
					...humanOrAlienFragment
				}

				fragment humanOrAlienFragment on HumanOrAlien {
					... on Human {
						name
					}
				}

				# Use fragment in query to avoid errors.
				{ catOrDog { ...catOrDogFragment } }
			`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{3, 44},
					},
					Path: []PathSegment{
						{Field: "catOrDog"},
					},
				},
			},
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-7ee0e
			name: "Values/Type/Valid",
			request: `
				query goodBooleanArg {
					arguments {
						booleanArgField(booleanArg: true)
					}
				}

				query coercedIntIntoFloatArg {
					arguments {
						# Note: The input coercion rules for Float allow Int literals.
						floatArgField(floatArg: 123)
					}
				}
			`,
			wantErrors: nil,
		},
		{
			name: "Values/Type/Valid/EmptyList",
			request: `{
				arguments {
					booleanListArgField(booleanListArg: [])
				}
			}`,
			wantErrors: nil,
		},
		{
			name: "Values/Type/Valid/List",
			request: `{
				arguments {
					booleanListArgField(booleanListArg: [false, true])
				}
			}`,
			wantErrors: nil,
		},
		{
			name: "Values/Type/Valid/ElementToList",
			request: `{
				arguments {
					booleanListArgField(booleanListArg: false)
				}
			}`,
			wantErrors: nil,
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-3a7c1
			name: "Values/Type/StringToInt",
			request: `
				{
					arguments {
						intArgField(intArg: "123")
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 69},
					},
					Path: []PathSegment{
						{Field: "arguments"},
						{Field: "intArgField"},
					},
				},
			},
		},
		{
			name: "Values/Type/BadEnumName",
			request: `{
				dog {
					doesKnowCommand(dogCommand: THINK)
				}
			}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{3, 69},
					},
					Path: []PathSegment{
						{Field: "dog"},
						{Field: "doesKnowCommand"},
					},
				},
			},
		},
		{
			name: "Values/Type/EnumToCustomScalar",
			request: `{
				arguments {
					customScalar(arg: WOOF)
				}
			}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{3, 59},
					},
					Path: []PathSegment{
						{Field: "arguments"},
						{Field: "customScalar"},
					},
				},
			},
		},
		{
			name: "Values/Type/StringToCustomScalar",
			request: `{
				arguments {
					customScalar(arg: "woof")
				}
			}`,
			wantErrors: nil,
		},
		{
			name: "Values/Type/IntToCustomScalar",
			request: `{
				arguments {
					customScalar(arg: 123)
				}
			}`,
			wantErrors: nil,
		},
		{
			name:       "Values/Type/ID/Literal/Int",
			request:    `{ dogById(id: 123) { name } }`,
			wantErrors: nil,
		},
		{
			name:       "Values/Type/ID/Literal/String",
			request:    `{ dogById(id: "Fido") { name } }`,
			wantErrors: nil,
		},
		{
			name:    "Values/Type/ID/Literal/Float",
			request: `{ dogById(id: 123.0) { name } }`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{1, 15},
					},
					Path: []PathSegment{
						{Field: "dogById"},
					},
				},
			},
		},
		{
			name: "Values/Type/WrongListTypes",
			request: `{
				arguments {
					booleanListArgField(booleanListArg: ["foo", 123])
				}
			}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{3, 78},
					},
					Path: []PathSegment{
						{Field: "arguments"},
						{Field: "booleanListArgField"},
					},
				},
				{
					Locations: []Location{
						{3, 85},
					},
					Path: []PathSegment{
						{Field: "arguments"},
						{Field: "booleanListArgField"},
					},
				},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-a940b
			name: "Values/Input/FieldName/Valid",
			request: `
				{
					findDog(complex: { name: "Fido" }) {
						name
					}
				}`,
			wantErrors: nil,
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-1a5f6
			name: "Values/Input/FieldTypeCheck",
			request: `
				{
					findDog(complex: { name: 42 }) {
						name
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{3, 66},
					},
					Path: []PathSegment{
						{Field: "findDog"},
					},
				},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-1a5f6
			name: "Values/Input/FieldName/Invalid",
			request: `
				{
					findDog(complex: { favoriteCookieFlavor: "Bacon", name: "Fido" }) {
						name
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{3, 60},
					},
					Path: []PathSegment{
						{Field: "findDog"},
					},
				},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-5d541
			name: "Values/Input/Uniqueness",
			request: `
				{
					findDog(complex: { name: "Fido", name: "Fido" }) {
						name
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{3, 60},
						{3, 74},
					},
					Path: []PathSegment{
						{Field: "findDog"},
					},
				},
			},
		},
		{
			name: "Values/Input/Required/Missing",
			request: `
				{
					findDog(complex: { owner: "Fred" }) {
						name
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{3, 74},
					},
					Path: []PathSegment{
						{Field: "findDog"},
					},
				},
			},
		},
		{
			name: "Values/Input/Required/Null",
			request: `
				{
					findDog(complex: { name: null }) {
						name
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{3, 66},
					},
					Path: []PathSegment{
						{Field: "findDog"},
					},
				},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-b767a
			name: "Variables/Uniqueness/Fail",
			request: `
				query houseTrainedQuery($atOtherHomes: Boolean, $atOtherHomes: Boolean) {
					dog {
						isHousetrained(atOtherHomes: $atOtherHomes)
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{2, 57},
						{2, 81},
					},
				},
			},
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-6f6b9
			name: "Variables/Uniqueness/NotAcrossOperations",
			request: `
				query A($atOtherHomes: Boolean) {
					dog {
						isHousetrained(atOtherHomes: $atOtherHomes)
					}
				}

				query B($atOtherHomes: Boolean) {
					dog {
						isHousetrained(atOtherHomes: $atOtherHomes)
					}
				}`,
			wantErrors: nil,
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-77f18
			name: "Variables/InputType/Valid",
			request: `
				query takesBoolean($atOtherHomes: Boolean) {
					dog {
						isHousetrained(atOtherHomes: $atOtherHomes)
					}
				}

				query takesComplexInput($complexInput: ComplexInput) {
					findDog(complex: $complexInput) {
						name
					}
				}

				query TakesListOfBooleanBang($booleans: [Boolean!]) {
					booleanList(booleanListArg: $booleans)
				}`,
			wantErrors: nil,
		},
		{
			// Inspired by https://graphql.github.io/graphql-spec/June2018/#example-aeba9
			name: "Variables/InputType/Invalid",
			request: `
				query takesDog($dog: Dog) {
					dog { name }
				}

				query takesDogBang($dog: Dog!) {
					dog { name }
				}

				query takesListOfDog($dogs: [Dog]) {
					dog { name }
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{2, 54},
					},
				},
				{
					Locations: []Location{
						{6, 58},
					},
				},
				{
					Locations: []Location{
						{10, 61},
					},
				},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-a5099
			name: "Variables/Defined/Valid",
			request: `
				query variableIsDefined($atOtherHomes: Boolean) {
					dog {
						isHousetrained(atOtherHomes: $atOtherHomes)
					}
				}`,
			wantErrors: nil,
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-c8425
			name: "Variables/Defined/Invalid",
			request: `
				query variableIsNotDefined {
					dog {
						isHousetrained(atOtherHomes: $atOtherHomes)
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 78},
					},
					Path: []PathSegment{
						{Field: "dog"},
						{Field: "isHousetrained"},
					},
				},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-516af
			name: "Variables/Unused",
			request: `
				query variableUnused($atOtherHomes: Boolean) {
					dog {
						isHousetrained
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{2, 54},
					},
				},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-2028e
			name: "Variables/TypeCheck/Wrong",
			request: `
				query intCannotGoIntoBoolean($intArg: Int) {
					arguments {
						booleanArgField(booleanArg: $intArg)
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 77},
					},
					Path: []PathSegment{
						{Field: "arguments"},
						{Field: "booleanArgField"},
					},
				},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-ed727
			name: "Variables/TypeCheck/Nullability",
			request: `
				query booleanArgQuery($booleanArg: Boolean) {
					arguments {
						nonNullBooleanArgField(nonNullBooleanArg: $booleanArg)
					}
				}`,
			wantErrors: []*ResponseError{
				{
					Locations: []Location{
						{4, 91},
					},
					Path: []PathSegment{
						{Field: "arguments"},
						{Field: "nonNullBooleanArgField"},
					},
				},
			},
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-0877c
			name: "Variables/TypeCheck/NullabilityWithDefault",
			request: `
				query booleanArgQueryWithDefault($booleanArg: Boolean) {
					arguments {
						optionalNonNullBooleanArgField(optionalBooleanArg: $booleanArg)
					}
				}`,
			wantErrors: nil,
		},
		{
			// https://graphql.github.io/graphql-spec/June2018/#example-d24d9
			name: "Variables/TypeCheck/NullabilityWithVarDefault",
			request: `
				query booleanArgQueryWithDefault($booleanArg: Boolean = true) {
					arguments {
						nonNullBooleanArgField(nonNullBooleanArg: $booleanArg)
					}
				}`,
			wantErrors: nil,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, got := schema.Validate(test.request)
			for _, err := range got {
				t.Logf("Error: %s", err.Message)
			}
			if diff := compareErrors(test.wantErrors, got); diff != "" {
				t.Errorf("errors (-want +got):\n%s", diff)
			}
		})
	}
}

func compareErrors(want, got []*ResponseError) string {
	return cmp.Diff(want, got,
		cmpopts.EquateEmpty(),
		cmpopts.IgnoreFields(ResponseError{}, "Message"),
		cmpopts.SortSlices(func(l, m Location) bool {
			if l.Line == m.Line {
				return l.Column < m.Column
			}
			return l.Line < m.Line
		}))
}
