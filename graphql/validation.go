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
	"strconv"

	"golang.org/x/xerrors"
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
	for _, defn := range doc.Definitions {
		if defn.Operation == nil || defn.Operation.Name == nil {
			continue
		}
		name := defn.Operation.Name.Value
		posList := operationsByName[name]
		if defn.Operation.Name.Start == posList[0] {
			continue
		}
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
		if defn.typ().isNullable() {
			continue
		}
		if len(argumentsByName[name]) == 0 && defn.defaultValue.IsNull() {
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
	// No sense in validating argument values if the arrangment is wrong.
	if len(errs) > 0 {
		return errs
	}
	for name, defn := range defns {
		args := argumentsByName[name]
		if len(args) == 0 {
			continue
		}
		argErrs := validateValue(source, defn.typ(), args[0].Value)
		for _, err := range argErrs {
			errs = append(errs, xerrors.Errorf("argument %s: %w", name, err))
		}
	}
	return errs
}

func validateValue(source string, typ *gqlType, val *gqlang.InputValue) []error {
	if val.Null != nil {
		if !typ.isNullable() {
			return []error{&ResponseError{
				Message: fmt.Sprintf("null not permitted for %v", typ),
				Locations: []Location{
					astPositionToLocation(val.Null.Start.ToPosition(source)),
				},
			}}
		}
		return nil
	}
	genericErr := &ResponseError{
		Message: fmt.Sprintf("cannot coerce %v to %v", val, typ),
		Locations: []Location{
			astPositionToLocation(val.Start().ToPosition(source)),
		},
	}
	switch nullableType := typ.toNullable(); {
	case nullableType == intType:
		if val.Scalar == nil || val.Scalar.Type != gqlang.IntScalar {
			return []error{genericErr}
		}
		scalar := val.Scalar.Value()
		if _, err := strconv.ParseInt(scalar, 10, 32); err != nil {
			return []error{&ResponseError{
				Message:   fmt.Sprintf("%q is not in the range of a 32-bit integer", scalar),
				Locations: genericErr.Locations,
			}}
		}
	case nullableType == floatType:
		if val.Scalar == nil || (val.Scalar.Type != gqlang.FloatScalar && val.Scalar.Type != gqlang.IntScalar) {
			return []error{genericErr}
		}
		scalar := val.Scalar.Value()
		if _, err := strconv.ParseFloat(scalar, 64); err != nil {
			return []error{&ResponseError{
				Message:   fmt.Sprintf("%q is not representable as a float", scalar),
				Locations: genericErr.Locations,
			}}
		}
	case nullableType == stringType:
		if val.Scalar == nil || val.Scalar.Type != gqlang.StringScalar {
			return []error{genericErr}
		}
	case nullableType == booleanType:
		if val.Scalar == nil || val.Scalar.Type != gqlang.BooleanScalar {
			return []error{genericErr}
		}
	case nullableType == idType:
		if val.Scalar == nil || (val.Scalar.Type != gqlang.StringScalar && val.Scalar.Type != gqlang.IntScalar) {
			return []error{genericErr}
		}
	case nullableType.isScalar():
		if val.Scalar == nil {
			return []error{genericErr}
		}
	case nullableType.isInputObject():
		if val.InputObject == nil {
			return []error{genericErr}
		}
		fieldsByName := make(map[string][]*gqlang.InputObjectField)
		var errs []error
		for _, field := range val.InputObject.Fields {
			name := field.Name.Value
			if _, exists := typ.input.fields[name]; !exists {
				// https://graphql.github.io/graphql-spec/June2018/#sec-Input-Object-Field-Names
				errs = append(errs, &ResponseError{
					Message: fmt.Sprintf("unknown input field %s for %v", name, typ),
					Locations: []Location{
						astPositionToLocation(field.Name.Start.ToPosition(source)),
					},
				})
				continue
			}
			fieldsByName[name] = append(fieldsByName[name], field)
		}
		for _, field := range val.InputObject.Fields {
			name := field.Name.Value
			fieldList := fieldsByName[name]
			if len(fieldList) > 1 && field == fieldList[0] {
				// https://graphql.github.io/graphql-spec/June2018/#sec-Input-Object-Field-Uniqueness
				e := &ResponseError{
					Message: fmt.Sprintf("multiple input fields for %v.%s", nullableType, name),
				}
				for _, g := range fieldList {
					e.Locations = append(e.Locations, astPositionToLocation(g.Name.Start.ToPosition(source)))
				}
				errs = append(errs, e)
			}
		}
		// https://graphql.github.io/graphql-spec/June2018/#sec-Input-Object-Required-Fields
		for name, defn := range typ.input.fields {
			if len(fieldsByName[name]) == 0 {
				if !defn.typ().isNullable() && defn.defaultValue.IsNull() {
					errs = append(errs, &ResponseError{
						Message: fmt.Sprintf("missing required input field for %v.%s", nullableType, name),
						Locations: []Location{
							astPositionToLocation(val.InputObject.RBrace.ToPosition(source)),
						},
					})
				}
				continue
			}
			field := fieldsByName[name][0]
			if !defn.typ().isNullable() && field.Value.Null != nil {
				errs = append(errs, &ResponseError{
					Message: fmt.Sprintf("required input field %v.%s is null", nullableType, name),
					Locations: []Location{
						astPositionToLocation(field.Value.Null.Start.ToPosition(source)),
					},
				})
				continue
			}
			fieldErrs := validateValue(source, defn.typ(), field.Value)
			for _, err := range fieldErrs {
				errs = append(errs, xerrors.Errorf("input field %s: %w", name, err))
			}
		}
		return errs
	}
	return nil
}

func posListToLocationList(source string, posList []gqlang.Pos) []Location {
	locList := make([]Location, len(posList))
	for i := range locList {
		locList[i] = astPositionToLocation(posList[i].ToPosition(source))
	}
	return locList
}
