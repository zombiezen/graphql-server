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
	"fmt"

	"zombiezen.com/go/graphql-server/internal/gqlang"
)

// validateRequest validates a parsed GraphQL request according to the procedure
// defined in https://graphql.github.io/graphql-spec/June2018/#sec-Validation.
func (schema *Schema) validateRequest(source string, doc *gqlang.Document) []error {
	var errs []error
	var anonPosList []gqlang.Pos
	operationsByName := make(map[string][]gqlang.Pos)
	for _, defn := range doc.Definitions {
		if defn.Operation == nil {
			// https://graphql.github.io/graphql-spec/June2018/#sec-Executable-Definitions
			errs = append(errs, &ResponseError{
				Message: "not an operation nor a fragment",
				Locations: []Location{
					astPositionToLocation(defn.Start().ToPosition(source)),
				},
			})
			continue
		}
		if defn.Operation.Name == nil {
			anonPosList = append(anonPosList, defn.Operation.Start)
			continue
		}
		name := defn.Operation.Name.Value
		operationsByName[name] = append(operationsByName[name], defn.Operation.Name.Start)
	}
	if len(anonPosList) > 1 {
		// https://graphql.github.io/graphql-spec/June2018/#sec-Lone-Anonymous-Operation
		errs = append(errs, &ResponseError{
			Message:   "multiple anonymous operations",
			Locations: posListToLocationList(source, anonPosList),
		})
	}
	if len(anonPosList) > 0 && len(operationsByName) > 0 {
		// https://graphql.github.io/graphql-spec/June2018/#sec-Lone-Anonymous-Operation
		errs = append(errs, &ResponseError{
			Message:   "anonymous operations mixed with named operations",
			Locations: posListToLocationList(source, anonPosList),
		})
	}
	for name, posList := range operationsByName {
		if len(posList) > 1 {
			// https://graphql.github.io/graphql-spec/June2018/#sec-Operation-Name-Uniqueness
			errs = append(errs, &ResponseError{
				Message:   fmt.Sprintf("multiple operations with name %q", name),
				Locations: posListToLocationList(source, posList),
			})
		}
	}
	if len(errs) > 0 {
		return errs
	}
	for _, defn := range doc.Definitions {
		op := defn.Operation
		if op == nil {
			continue
		}
		opType := schema.operationType(op.Type)
		if opType == nil {
			errs = append(errs, &ResponseError{
				Message: fmt.Sprintf("%v unsupported", op.Type),
				Locations: []Location{
					astPositionToLocation(defn.Operation.Start.ToPosition(source)),
				},
			})
			continue
		}
		errs = append(errs, validateSelectionSet(source, opType, op.SelectionSet)...)
	}
	return errs
}

func validateSelectionSet(source string, typ *gqlType, set *gqlang.SelectionSet) []error {
	var errs []error
	for _, selection := range set.Sel {
		fieldInfo := typ.obj.fields[selection.Field.Name.Value]
		loc := astPositionToLocation(selection.Field.Name.Start.ToPosition(source))
		if fieldInfo.typ == nil {
			// Field not found.
			// https://graphql.github.io/graphql-spec/June2018/#sec-Field-Selections-on-Objects-Interfaces-and-Unions-Types
			errs = append(errs, &ResponseError{
				Message:   fmt.Sprintf("field %q not found on type %v", selection.Field.Name.Value, typ),
				Locations: []Location{loc},
				Path: []PathSegment{
					{Field: selection.Field.Key()},
				},
			})
			continue
		}
		// https://graphql.github.io/graphql-spec/June2018/#sec-Validation.Arguments
		argsErrs := validateArguments(source, fieldInfo.args, selection.Field.Arguments)
		if len(argsErrs) > 0 {
			argsPos := selection.Field.Name.End()
			if selection.Field.Arguments != nil {
				argsPos = selection.Field.Arguments.RParen
			}
			argsLoc := astPositionToLocation(argsPos.ToPosition(source))
			for _, err := range argsErrs {
				errs = append(errs, wrapFieldError(selection.Field.Key(), argsLoc, err))
			}
		}

		// https://graphql.github.io/graphql-spec/June2018/#sec-Leaf-Field-Selections
		if subsetType := fieldInfo.typ.selectionSetType(); subsetType != nil {
			if selection.Field.SelectionSet == nil {
				errs = append(errs, &ResponseError{
					Message: fmt.Sprintf("object field %q missing selection set", selection.Field.Name.Value),
					Locations: []Location{
						astPositionToLocation(selection.Field.End().ToPosition(source)),
					},
					Path: []PathSegment{
						{Field: selection.Field.Key()},
					},
				})
				continue
			}
			subErrs := validateSelectionSet(source, subsetType, selection.Field.SelectionSet)
			for _, err := range subErrs {
				errs = append(errs, wrapFieldError(selection.Field.Key(), loc, err))
			}
		} else if selection.Field.SelectionSet != nil {
			errs = append(errs, &ResponseError{
				Message: fmt.Sprintf("scalar field %q must not have selection set", selection.Field.Name.Value),
				Locations: []Location{
					astPositionToLocation(selection.Field.SelectionSet.LBrace.ToPosition(source)),
				},
				Path: []PathSegment{
					{Field: selection.Field.Key()},
				},
			})
		}
	}
	return errs
}

func validateArguments(source string, defns map[string]inputValueDefinition, args *gqlang.Arguments) []error {
	var argumentNames []string
	argumentsByName := make(map[string][]*gqlang.Argument)
	var endLocation []Location
	var errs []error
	if args != nil {
		for _, arg := range args.Args {
			name := arg.Name.Value
			if len(argumentsByName[name]) == 0 {
				argumentNames = append(argumentNames, name)
			}
			argumentsByName[name] = append(argumentsByName[name], arg)
		}
		for _, name := range argumentNames {
			// https://graphql.github.io/graphql-spec/June2018/#sec-Argument-Names
			if defns[name].typ() == nil {
				err := &ResponseError{
					Message: fmt.Sprintf("unknown argument %s", name),
				}
				for _, arg := range argumentsByName[name] {
					err.Locations = append(err.Locations, astPositionToLocation(arg.Name.Start.ToPosition(source)))
				}
				errs = append(errs, err)
				continue
			}
			// https://graphql.github.io/graphql-spec/June2018/#sec-Argument-Uniqueness
			if args := argumentsByName[name]; len(args) > 1 {
				err := &ResponseError{
					Message: fmt.Sprintf("multiple values for argument %s", name),
				}
				for _, arg := range argumentsByName[name] {
					err.Locations = append(err.Locations, astPositionToLocation(arg.Name.Start.ToPosition(source)))
				}
				errs = append(errs, err)
				continue
			}
		}
		endLocation = []Location{
			astPositionToLocation(args.RParen.ToPosition(source)),
		}
	}
	// https://graphql.github.io/graphql-spec/June2018/#sec-Required-Arguments
	for name, defn := range defns {
		if defn.typ().isNullable() || !defn.defaultValue.IsNull() {
			continue
		}
		if len(argumentsByName[name]) == 0 {
			errs = append(errs, &ResponseError{
				Message:   fmt.Sprintf("missing required argument %s", name),
				Locations: endLocation,
			})
			continue
		}
		arg := argumentsByName[name][0]
		if arg.Value.Null != nil {
			errs = append(errs, &ResponseError{
				Message: fmt.Sprintf("required argument %s cannot be null", name),
				Locations: []Location{
					astPositionToLocation(arg.Value.Null.Start.ToPosition(source)),
				},
			})
		}
	}
	return errs
}

func posListToLocationList(source string, posList []gqlang.Pos) []Location {
	locList := make([]Location, len(posList))
	for i := range locList {
		locList[i] = astPositionToLocation(posList[i].ToPosition(source))
	}
	return locList
}
