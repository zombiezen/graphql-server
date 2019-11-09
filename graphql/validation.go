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
	fragments, errs := schema.validateStructure(source, doc)
	if len(errs) > 0 {
		return errs
	}
	docScope := &validationScope{
		source:    source,
		types:     schema.types,
		fragments: fragments,
	}
	// Ensure there are no cycles before validating operations, since otherwise
	// they could have unbounded recursion.
	for _, defn := range doc.Definitions {
		if defn.Fragment == nil {
			continue
		}
		name := defn.Fragment.Name.Value
		if err := detectFragmentCycles(docScope, map[string]struct{}{name: {}}, defn.Fragment.SelectionSet); err != nil {
			return []error{err}
		}
	}
	for _, defn := range doc.Definitions {
		if defn.Operation == nil {
			continue
		}
		errs = append(errs, schema.validateOperation(source, fragments, defn.Operation)...)
	}
	errs = append(errs, validateFragmentUsage(docScope, doc.Definitions)...)
	return errs
}

// validateStructure validates the well-formedness of the top-level definitions.
func (schema *Schema) validateStructure(source string, doc *gqlang.Document) (map[string]*fragmentValidationState, []error) {
	var errs []error
	var anonPosList []gqlang.Pos
	operationsByName := make(map[string][]gqlang.Pos)
	fragmentsByName := make(map[string][]gqlang.Pos)
	fragments := make(map[string]*fragmentValidationState)
	for _, defn := range doc.Definitions {
		switch {
		case defn.Operation != nil:
			if defn.Operation.Name == nil {
				anonPosList = append(anonPosList, defn.Operation.Start)
				continue
			}
			name := defn.Operation.Name.Value
			operationsByName[name] = append(operationsByName[name], defn.Operation.Name.Start)
		case defn.Fragment != nil:
			name := defn.Fragment.Name.Value
			fragmentsByName[name] = append(fragmentsByName[name], defn.Fragment.Name.Start)
			fragments[name] = &fragmentValidationState{
				FragmentDefinition: defn.Fragment,
			}
		default:
			// https://graphql.github.io/graphql-spec/June2018/#sec-Executable-Definitions
			errs = append(errs, &ResponseError{
				Message: "not an operation nor a fragment",
				Locations: []Location{
					astPositionToLocation(defn.Start().ToPosition(source)),
				},
			})
			continue
		}
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
	// https://graphql.github.io/graphql-spec/June2018/#sec-Operation-Name-Uniqueness
	for _, defn := range doc.Definitions {
		if defn.Operation == nil || defn.Operation.Name == nil {
			continue
		}
		name := defn.Operation.Name.Value
		posList := operationsByName[name]
		if defn.Operation.Name.Start != posList[0] {
			continue
		}
		if len(posList) > 1 {
			errs = append(errs, &ResponseError{
				Message:   fmt.Sprintf("multiple operations with name %q", name),
				Locations: posListToLocationList(source, posList),
			})
		}
	}
	// https://graphql.github.io/graphql-spec/draft/#sec-Fragment-Name-Uniqueness
	for _, defn := range doc.Definitions {
		if defn.Fragment == nil {
			continue
		}
		name := defn.Fragment.Name.Value
		posList := fragmentsByName[name]
		if defn.Fragment.Name.Start != posList[0] {
			continue
		}
		if len(posList) > 1 {
			errs = append(errs, &ResponseError{
				Message:   fmt.Sprintf("multiple fragments with name %q", name),
				Locations: posListToLocationList(source, posList),
			})
		}
	}
	return fragments, errs
}

func detectFragmentCycles(v *validationScope, visited map[string]struct{}, set *gqlang.SelectionSet) error {
	if set == nil {
		return nil
	}
	for _, sel := range set.Sel {
		switch {
		case sel.Field != nil:
			if err := detectFragmentCycles(v, visited, sel.Field.SelectionSet); err != nil {
				return err
			}
		case sel.InlineFragment != nil:
			if err := detectFragmentCycles(v, visited, sel.InlineFragment.SelectionSet); err != nil {
				return err
			}
		case sel.FragmentSpread != nil:
			name := sel.FragmentSpread.Name.Value
			if _, seen := visited[name]; seen {
				return &ResponseError{
					Message: fmt.Sprintf("fragment %s is self-referential", name),
					Locations: []Location{
						astPositionToLocation(sel.FragmentSpread.Name.Start.ToPosition(v.source)),
					},
				}
			}
			frag := v.fragments[name]
			if frag == nil {
				continue
			}
			visited[name] = struct{}{}
			if err := detectFragmentCycles(v, visited, frag.SelectionSet); err != nil {
				return err
			}
			delete(visited, name)
		}
	}
	return nil
}

func (schema *Schema) validateOperation(source string, fragments map[string]*fragmentValidationState, op *gqlang.Operation) []error {
	opType := schema.operationType(op.Type)
	if opType == nil {
		return []error{&ResponseError{
			Message: fmt.Sprintf("%v unsupported", op.Type),
			Locations: []Location{
				astPositionToLocation(op.Start.ToPosition(source)),
			},
		}}
	}
	variables, varErrs := validateVariables(source, schema.types, op.VariableDefinitions)
	if len(varErrs) > 0 {
		if op.Name != nil {
			for i, err := range varErrs {
				varErrs[i] = xerrors.Errorf("operation %s: %w", op.Name, err)
			}
		}
		return varErrs
	}
	v := &validationScope{
		source:    source,
		types:     schema.types,
		variables: variables,
		fragments: fragments,
	}
	var errs []error
	selErrs := validateSelectionSet(v, op.Type == gqlang.Query, opType, op.SelectionSet)
	if op.Name != nil {
		for _, err := range selErrs {
			errs = append(errs, xerrors.Errorf("operation %s: %w", op.Name, err))
		}
	} else {
		errs = append(errs, selErrs...)
	}
	useErrs := validateVariableUsage(v, op.VariableDefinitions)
	if op.Name != nil {
		for _, err := range useErrs {
			errs = append(errs, xerrors.Errorf("operation %s: %w", op.Name, err))
		}
	} else {
		errs = append(errs, useErrs...)
	}
	return errs
}

// validationScope defines the symbols and position information used
// during validation.
type validationScope struct {
	source    string
	types     map[string]*gqlType
	variables map[string]*validatedVariable
	fragments map[string]*fragmentValidationState
}

type validatedVariable struct {
	name         string
	defaultValue Value
	used         bool
}

func (vv *validatedVariable) typ() *gqlType {
	return vv.defaultValue.typ
}

// fragmentValidationState records whether a fragment is used.
type fragmentValidationState struct {
	*gqlang.FragmentDefinition
	used bool
}

// validateVariables validates an operation's variables and resolves their types.
func validateVariables(source string, typeMap map[string]*gqlType, defns *gqlang.VariableDefinitions) (map[string]*validatedVariable, []error) {
	if defns == nil {
		return nil, nil
	}
	variablesByName := make(map[string][]gqlang.Pos)
	result := make(map[string]*validatedVariable)
	var errs []error
	for _, defn := range defns.Defs {
		name := defn.Var.Name.Value
		variablesByName[name] = append(variablesByName[name], defn.Var.Dollar)

		// https://graphql.github.io/graphql-spec/June2018/#sec-Variables-Are-Input-Types
		typ := resolveTypeRef(typeMap, defn.Type)
		if typ == nil {
			errs = append(errs, &ResponseError{
				Message: fmt.Sprintf("undefined type %v", defn.Type),
				Locations: []Location{
					astPositionToLocation(defn.Type.Start().ToPosition(source)),
				},
			})
			continue
		}
		if !typ.isInputType() {
			errs = append(errs, &ResponseError{
				Message: fmt.Sprintf("%v is not an input type", defn.Type),
				Locations: []Location{
					astPositionToLocation(defn.Type.Start().ToPosition(source)),
				},
			})
			continue
		}
		vv := &validatedVariable{
			name:         name,
			defaultValue: Value{typ: typ},
		}
		result[name] = vv
		if defn.Default != nil {
			defaultErrs := validateConstantValue(source, typ, defn.Default.Value)
			if len(defaultErrs) > 0 {
				for _, err := range defaultErrs {
					errs = append(errs, xerrors.Errorf("variable $%s: %w", name, err))
				}
				continue
			}
			vv.defaultValue = coerceConstantInputValue(typ, defn.Default.Value)
		}
	}

	// https://graphql.github.io/graphql-spec/June2018/#sec-Variable-Uniqueness
	for _, defn := range defns.Defs {
		name := defn.Var.Name.Value
		posList := variablesByName[name]
		if defn.Var.Dollar != posList[0] {
			continue
		}
		if len(posList) > 1 {
			errs = append(errs, &ResponseError{
				Message:   fmt.Sprintf("multiple variables with name %q", name),
				Locations: posListToLocationList(source, posList),
			})
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return result, nil
}

// validateVariableUsage verifies that all variables defined in the operation are used.
// See https://graphql.github.io/graphql-spec/June2018/#sec-All-Variables-Used
func validateVariableUsage(v *validationScope, defns *gqlang.VariableDefinitions) []error {
	if defns == nil {
		return nil
	}
	var errs []error
	for _, defn := range defns.Defs {
		name := defn.Var.Name.Value
		if !v.variables[name].used {
			errs = append(errs, &ResponseError{
				Message: fmt.Sprintf("unused variable $%s", name),
				Locations: []Location{
					astPositionToLocation(defn.Var.Dollar.ToPosition(v.source)),
				},
			})
		}
	}
	return errs
}

// validateFragmentUsage verifies that all fragments defined in the document are used.
// See https://graphql.github.io/graphql-spec/draft/#sec-Fragments-Must-Be-Used
func validateFragmentUsage(v *validationScope, defns []*gqlang.Definition) []error {
	var errs []error
	for _, defn := range defns {
		if defn.Fragment == nil {
			continue
		}
		name := defn.Fragment.Name.Value
		if !v.fragments[name].used {
			errs = append(errs, &ResponseError{
				Message: fmt.Sprintf("unused fragment %s", name),
				Locations: []Location{
					astPositionToLocation(defn.Fragment.Keyword.ToPosition(v.source)),
				},
			})
		}
	}
	return errs
}

func validateSelectionSet(v *validationScope, isRootQuery bool, typ *gqlType, set *gqlang.SelectionSet) []error {
	var errs []error
	for _, selection := range set.Sel {
		switch {
		case selection.Field != nil:
			errs = append(errs, validateField(v, isRootQuery, typ, selection.Field)...)
		case selection.FragmentSpread != nil:
			name := selection.FragmentSpread.Name
			frag := v.fragments[name.Value]
			if frag == nil {
				errs = append(errs, &ResponseError{
					Message: fmt.Sprintf("undefined fragment %s", name.Value),
					Locations: []Location{
						astPositionToLocation(name.Start.ToPosition(v.source)),
					},
				})
				continue
			}
			frag.used = true
			condTyp, condErr := validateFragmentTypeCondition(v, typ, frag.Type)
			if condErr != nil {
				errs = append(errs, xerrors.Errorf("fragment %s: %w", name.Value, condErr))
				continue
			}
			selErrs := validateSelectionSet(v, isRootQuery, condTyp, frag.SelectionSet)
			for _, err := range selErrs {
				errs = append(errs, xerrors.Errorf("fragment %s: %w", name.Value, err))
			}
		case selection.InlineFragment != nil:
			cond := selection.InlineFragment.Type
			condTyp, condErr := validateFragmentTypeCondition(v, typ, cond)
			if condErr != nil {
				errs = append(errs, condErr)
				continue
			}
			errs = append(errs, validateSelectionSet(v, isRootQuery, condTyp, selection.InlineFragment.SelectionSet)...)
		default:
			panic("unknown selection type")
		}
	}
	var groups fieldGroups
	groups.addSet(v, typ, set)
	for _, group := range groups {
		errs = append(errs, checkFieldMergability(v, group)...)
	}
	return errs
}

func validateFragmentTypeCondition(v *validationScope, parent *gqlType, cond *gqlang.TypeCondition) (*gqlType, error) {
	if cond == nil {
		return parent, nil
	}
	typName := cond.Name.Value
	typ := v.types[typName]
	if typ == nil {
		return nil, &ResponseError{
			Message: fmt.Sprintf("unknown type %s", typName),
			Locations: []Location{
				astPositionToLocation(cond.Name.Start.ToPosition(v.source)),
			},
		}
	}
	if typ.selectionSetType() == nil {
		return nil, &ResponseError{
			Message: fmt.Sprintf("type %s must be a composite", typName),
			Locations: []Location{
				astPositionToLocation(cond.Name.Start.ToPosition(v.source)),
			},
		}
	}
	parentPossibles := parent.possibleTypes()
	possible := false
	for p := range typ.possibleTypes() {
		if _, found := parentPossibles[p]; found {
			possible = true
			break
		}
	}
	if !possible {
		return nil, &ResponseError{
			Message: fmt.Sprintf("objects of type %v can never be a %s", parent.toNullable(), typName),
			Locations: []Location{
				astPositionToLocation(cond.Name.Start.ToPosition(v.source)),
			},
		}
	}
	return typ, nil
}

func validateField(v *validationScope, isRootQuery bool, typ *gqlType, field *gqlang.Field) []error {
	fieldInfo := typ.obj.field(field.Name.Value)
	if field.Name.Value == typeNameFieldName {
		fieldInfo = typeNameField()
	} else if isRootQuery {
		// Top-level queries have a few extra fields for introspection:
		// https://graphql.github.io/graphql-spec/June2018/#sec-Schema-Introspection
		switch field.Name.Value {
		case typeByNameFieldName:
			fieldInfo = typeByNameField()
		case schemaFieldName:
			fieldInfo = schemaField()
		}
	}
	loc := astPositionToLocation(field.Name.Start.ToPosition(v.source))
	if fieldInfo == nil {
		// Field not found.
		// https://graphql.github.io/graphql-spec/June2018/#sec-Field-Selections-on-Objects-Interfaces-and-Unions-Types
		return []error{&ResponseError{
			Message:   fmt.Sprintf("field %q not found on type %v", field.Name.Value, typ),
			Locations: []Location{loc},
			Path: []PathSegment{
				{Field: field.Key().Value},
			},
		}}
	}
	// https://graphql.github.io/graphql-spec/June2018/#sec-Validation.Arguments
	var errs []error
	argsErrs := validateArguments(v, fieldInfo.args, field.Arguments)
	if len(argsErrs) > 0 {
		argsPos := field.Name.End()
		if field.Arguments != nil {
			argsPos = field.Arguments.RParen
		}
		argsLoc := astPositionToLocation(argsPos.ToPosition(v.source))
		for _, err := range argsErrs {
			errs = append(errs, wrapFieldError(field.Key().Value, argsLoc, err))
		}
	}

	// https://graphql.github.io/graphql-spec/June2018/#sec-Leaf-Field-Selections
	if subsetType := fieldInfo.typ.selectionSetType(); subsetType != nil {
		if field.SelectionSet == nil {
			errs = append(errs, &ResponseError{
				Message: fmt.Sprintf("object field %q missing selection set", field.Name.Value),
				Locations: []Location{
					astPositionToLocation(field.End().ToPosition(v.source)),
				},
				Path: []PathSegment{
					{Field: field.Key().Value},
				},
			})
			return errs
		}
		subErrs := validateSelectionSet(v, false, subsetType, field.SelectionSet)
		for _, err := range subErrs {
			errs = append(errs, wrapFieldError(field.Key().Value, loc, err))
		}
	} else if field.SelectionSet != nil {
		errs = append(errs, &ResponseError{
			Message: fmt.Sprintf("scalar field %q must not have selection set", field.Name.Value),
			Locations: []Location{
				astPositionToLocation(field.SelectionSet.LBrace.ToPosition(v.source)),
			},
			Path: []PathSegment{
				{Field: field.Key().Value},
			},
		})
	}
	return errs
}

// checkFieldMergability ensures that the fields with the same key can be
// merged, returning errors if they can't.
// See https://graphql.github.io/graphql-spec/June2018/#FieldsInSetCanMerge%28%29
func checkFieldMergability(v *validationScope, fields []groupedField) []error {
	key := fields[0].Key().Value
	locs := make([]Location, 0, len(fields))
	for _, f := range fields {
		locs = append(locs, astPositionToLocation(f.Start().ToPosition(v.source)))
	}

	// TODO(someday): Is there a way to avoid quadratic behavior?
	var errs []error
	for i, fieldA := range fields {
		for _, fieldB := range fields[i+1:] {
			if !sameResponseShape(v, fieldA, fieldB) {
				errs = append(errs, &fieldError{
					key:  key,
					locs: locs,
					err:  xerrors.Errorf("incompatible fields for %s", key),
				})
				continue
			}
			if fieldA.parentType != fieldB.parentType && fieldA.parentType.isObject() && fieldB.parentType.isObject() {
				continue
			}
			if fieldA.Name.Value != fieldB.Name.Value {
				errs = append(errs, &fieldError{
					key:  key,
					locs: locs,
					err:  xerrors.Errorf("different fields found for %s", key),
				})
				continue
			}
			if !fieldA.Arguments.IdenticalTo(fieldB.Arguments) {
				errs = append(errs, &fieldError{
					key:  key,
					locs: locs,
					err:  xerrors.Errorf("different arguments found for %s", key),
				})
				continue
			}
			var subgroups fieldGroups
			subgroups.addSet(v, fieldA.typ(), fieldA.SelectionSet)
			subgroups.addSet(v, fieldB.typ(), fieldB.SelectionSet)
			for _, group := range subgroups {
				for _, err := range checkFieldMergability(v, group) {
					// checkFieldMergability will always attach locations.
					errs = append(errs, &fieldError{
						key: key,
						err: err,
					})
				}
			}
		}
	}
	return errs
}

// sameResponseShape reports whether two fields have the same structure.
// See https://graphql.github.io/graphql-spec/June2018/#SameResponseShape%28%29
func sameResponseShape(v *validationScope, fieldA, fieldB groupedField) bool {
	typeA, typeB := fieldA.typ(), fieldB.typ()
	for {
		if !typeA.isNullable() || !typeB.isNullable() {
			if typeA.isNullable() || typeB.isNullable() {
				return false
			}
			typeA = typeA.toNullable()
			typeB = typeB.toNullable()
		}
		if !typeA.isList() && !typeB.isList() {
			break
		}
		if !typeA.isList() || !typeB.isList() {
			return false
		}
		typeA = typeA.listElem
		typeB = typeB.listElem
	}
	if typeA.isScalar() || typeA.isEnum() || typeB.isScalar() || typeB.isEnum() {
		return typeA == typeB
	}
	if typeA.selectionSetType() == nil || typeB.selectionSetType() == nil {
		return false
	}
	var groups fieldGroups
	groups.addSet(v, typeA, fieldA.SelectionSet)
	groups.addSet(v, typeB, fieldB.SelectionSet)
	for _, group := range groups {
		subA := group[0]
		for _, subB := range group[1:] {
			if !sameResponseShape(v, subA, subB) {
				return false
			}
		}
	}
	return true
}

// fieldGroups groups fields by response key encountered during validation so
// they can be checked for mergability. The zero value is an empty list of
// fields.
type fieldGroups [][]groupedField

type groupedField struct {
	*gqlang.Field
	parentType *gqlType
}

func (groups *fieldGroups) addSet(v *validationScope, typ *gqlType, set *gqlang.SelectionSet) {
	if set == nil {
		return
	}
	// This method intentionally skips undefined fields or types, because it makes
	// the merging logic more complex and because such issues will be caught by
	// other validation checks.
	for _, sel := range set.Sel {
		switch {
		case sel.Field != nil:
			if info := typ.obj.field(sel.Field.Name.Value); info == nil {
				continue
			}
			groups.add(typ, sel.Field)
		case sel.FragmentSpread != nil:
			frag := v.fragments[sel.FragmentSpread.Name.Value]
			if frag == nil {
				continue
			}
			fragType := v.types[frag.Type.Name.Value]
			if fragType == nil || fragType.selectionSetType() == nil {
				continue
			}
			groups.addSet(v, fragType, frag.SelectionSet)
		case sel.InlineFragment != nil:
			fragType := typ
			if sel.InlineFragment.Type != nil {
				fragType = v.types[sel.InlineFragment.Type.Name.Value]
				if fragType == nil || fragType.selectionSetType() == nil {
					continue
				}
			}
			groups.addSet(v, fragType, sel.InlineFragment.SelectionSet)
		default:
			panic("unknown selection type")
		}
	}
}

func (groups *fieldGroups) add(parentType *gqlType, field *gqlang.Field) {
	k := field.Key().Value
	gf := groupedField{field, parentType}
	for i, group := range *groups {
		groupKey := group[0].Key().Value
		if k == groupKey {
			(*groups)[i] = append(group, gf)
			return
		}
	}
	*groups = append(*groups, []groupedField{gf})
}

func (gf groupedField) typ() *gqlType {
	return gf.parentType.obj.field(gf.Name.Value).typ
}

func validateArguments(v *validationScope, defns inputValueDefinitionList, args *gqlang.Arguments) []error {
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
			if defns.byName(name) == nil {
				err := &ResponseError{
					Message: fmt.Sprintf("unknown argument %s", name),
				}
				for _, arg := range argumentsByName[name] {
					err.Locations = append(err.Locations, astPositionToLocation(arg.Name.Start.ToPosition(v.source)))
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
					err.Locations = append(err.Locations, astPositionToLocation(arg.Name.Start.ToPosition(v.source)))
				}
				errs = append(errs, err)
				continue
			}
		}
		endLocation = []Location{
			astPositionToLocation(args.RParen.ToPosition(v.source)),
		}
	}
	// https://graphql.github.io/graphql-spec/June2018/#sec-Required-Arguments
	for _, defn := range defns {
		if defn.Type().isNullable() {
			continue
		}
		if len(argumentsByName[defn.name]) == 0 {
			if defn.defaultValue.IsNull() {
				errs = append(errs, &ResponseError{
					Message:   fmt.Sprintf("missing required argument %s", defn.name),
					Locations: endLocation,
				})
			}
			continue
		}
		arg := argumentsByName[defn.name][0]
		if arg.Value.Null != nil {
			errs = append(errs, &ResponseError{
				Message: fmt.Sprintf("required argument %s cannot be null", defn.name),
				Locations: []Location{
					astPositionToLocation(arg.Value.Null.Start.ToPosition(v.source)),
				},
			})
		}
	}
	// No sense in validating argument values if the arrangment is wrong.
	if len(errs) > 0 {
		return errs
	}
	for _, defn := range defns {
		args := argumentsByName[defn.name]
		if len(args) == 0 {
			continue
		}
		argErrs := validateValue(v, defn.Type(), !defn.defaultValue.IsNull(), args[0].Value)
		for _, err := range argErrs {
			errs = append(errs, xerrors.Errorf("argument %s: %w", defn.name, err))
		}
	}
	return errs
}

func validateConstantValue(source string, typ *gqlType, val *gqlang.InputValue) []error {
	return validateValue(&validationScope{source: source}, typ, false, val)
}

func validateValue(v *validationScope, typ *gqlType, hasDefault bool, val *gqlang.InputValue) []error {
	if val.Null != nil {
		if !typ.isNullable() {
			return []error{&ResponseError{
				Message: fmt.Sprintf("null not permitted for %v", typ),
				Locations: []Location{
					astPositionToLocation(val.Null.Start.ToPosition(v.source)),
				},
			}}
		}
		return nil
	}
	if val.VariableRef != nil {
		vv := v.variables[val.VariableRef.Name.Value]
		// https://graphql.github.io/graphql-spec/June2018/#sec-All-Variable-Uses-Defined
		if vv == nil {
			return []error{&ResponseError{
				Message: fmt.Sprintf("undefined variable $%s", val.VariableRef.Name.Value),
				Locations: []Location{
					astPositionToLocation(val.VariableRef.Dollar.ToPosition(v.source)),
				},
			}}
		}
		if err := validateVariableRef(typ, hasDefault, vv); err != nil {
			return []error{&ResponseError{
				Message: err.Error(),
				Locations: []Location{
					astPositionToLocation(val.VariableRef.Dollar.ToPosition(v.source)),
				},
			}}
		}
		return nil
	}
	genericErr := &ResponseError{
		Message: fmt.Sprintf("cannot coerce %v to %v", val, typ),
		Locations: []Location{
			astPositionToLocation(val.Start().ToPosition(v.source)),
		},
	}
	switch {
	case typ.isScalar():
		if val.Scalar == nil || val.Scalar.Type == gqlang.EnumScalar {
			return []error{genericErr}
		}
		err := validateScalar(typ, val.Scalar.Value(), scalarTypeAffinity(val.Scalar.Type))
		if err != nil {
			return []error{&ResponseError{
				Message:   err.Error(),
				Locations: genericErr.Locations,
			}}
		}
	case typ.isEnum():
		if val.Scalar == nil || val.Scalar.Type != gqlang.EnumScalar {
			return []error{genericErr}
		}
		if got := val.Scalar.Value(); !typ.enum.has(got) {
			return []error{&ResponseError{
				Message:   fmt.Sprintf("%v is not a valid value for %v", got, typ),
				Locations: genericErr.Locations,
			}}
		}
	case typ.isList():
		if val.List == nil {
			// Attempt to validate as single-element list.
			// Yes, I'm just as surprised as you are at this behavior,
			// see https://graphql.github.io/graphql-spec/June2018/#sec-Type-System.List
			return validateValue(v, typ.listElem, false, val)
		}
		var errs []error
		for i, elem := range val.List.Values {
			elemErrs := validateValue(v, typ.listElem, false, elem)
			for _, err := range elemErrs {
				errs = append(errs, xerrors.Errorf("list[%d]: %w", i, err))
			}
		}
		return errs
	case typ.isInputObject():
		if val.InputObject == nil {
			return []error{genericErr}
		}
		fieldsByName := make(map[string][]*gqlang.InputObjectField)
		var errs []error
		for _, field := range val.InputObject.Fields {
			name := field.Name.Value
			if typ.input.fields.byName(name) == nil {
				// https://graphql.github.io/graphql-spec/June2018/#sec-Input-Object-Field-Names
				errs = append(errs, &ResponseError{
					Message: fmt.Sprintf("unknown input field %s for %v", name, typ),
					Locations: []Location{
						astPositionToLocation(field.Name.Start.ToPosition(v.source)),
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
					Message: fmt.Sprintf("multiple input fields for %v.%s", typ.toNullable(), name),
				}
				for _, g := range fieldList {
					e.Locations = append(e.Locations, astPositionToLocation(g.Name.Start.ToPosition(v.source)))
				}
				errs = append(errs, e)
			}
		}
		// https://graphql.github.io/graphql-spec/June2018/#sec-Input-Object-Required-Fields
		for _, defn := range typ.input.fields {
			if len(fieldsByName[defn.name]) == 0 {
				if !defn.Type().isNullable() && defn.defaultValue.IsNull() {
					errs = append(errs, &ResponseError{
						Message: fmt.Sprintf("missing required input field for %v.%s", typ.toNullable(), defn.name),
						Locations: []Location{
							astPositionToLocation(val.InputObject.RBrace.ToPosition(v.source)),
						},
					})
				}
				continue
			}
			field := fieldsByName[defn.name][0]
			if !defn.Type().isNullable() && field.Value.Null != nil {
				errs = append(errs, &ResponseError{
					Message: fmt.Sprintf("required input field %v.%s is null", typ.toNullable(), defn.name),
					Locations: []Location{
						astPositionToLocation(field.Value.Null.Start.ToPosition(v.source)),
					},
				})
				continue
			}
			fieldErrs := validateValue(v, defn.Type(), !defn.defaultValue.IsNull(), field.Value)
			for _, err := range fieldErrs {
				errs = append(errs, xerrors.Errorf("input field %s: %w", defn.name, err))
			}
		}
		return errs
	}
	return nil
}

// validateVariableRef returns an error if the variable usage is not allowed.
// See https://graphql.github.io/graphql-spec/June2018/#sec-All-Variable-Usages-are-Allowed
func validateVariableRef(typ *gqlType, usageHasDefault bool, vv *validatedVariable) error {
	vv.used = true
	locationType := typ
	if !typ.isNullable() && vv.typ().isNullable() {
		if vv.defaultValue.IsNull() && !usageHasDefault {
			return xerrors.Errorf("nullable variable $%s not permitted for %v", vv.name, typ)
		}
		locationType = typ.toNullable()
	}
	if !areTypesCompatible(locationType, vv.typ()) {
		return xerrors.Errorf("variable $%s (type %v) not allowed as %v", vv.name, vv.typ(), typ)
	}
	return nil
}

func validateScalar(typ *gqlType, scalar string, aff scalarAffinity) error {
	format := "cannot coerce %v to %v"
	if aff == noAffinity || aff == stringAffinity {
		format = "cannot coerce %q to %v"
	}
	genericErr := xerrors.Errorf(format, scalar, typ)
	switch nullableType := typ.toNullable(); {
	case nullableType == intType:
		if aff != noAffinity && aff != intAffinity {
			return genericErr
		}
		if _, err := strconv.ParseInt(scalar, 10, 32); err != nil {
			return xerrors.Errorf("%q is not in the range of a 32-bit integer", scalar)
		}
	case nullableType == floatType:
		if aff != noAffinity && aff != intAffinity && aff != floatAffinity {
			return genericErr
		}
		if _, err := strconv.ParseFloat(scalar, 64); err != nil {
			return xerrors.Errorf("%q is not representable as a float", scalar)
		}
	case nullableType == stringType:
		if aff != noAffinity && aff != stringAffinity {
			return genericErr
		}
	case nullableType == booleanType:
		if aff != noAffinity && aff != booleanAffinity {
			return genericErr
		}
	case nullableType == idType:
		if aff != noAffinity && aff != stringAffinity && aff != intAffinity {
			return genericErr
		}
	}
	return nil
}

type scalarAffinity int

const (
	noAffinity      = scalarAffinity(0)
	stringAffinity  = scalarAffinity(1 + gqlang.StringScalar)
	booleanAffinity = scalarAffinity(1 + gqlang.BooleanScalar)
	intAffinity     = scalarAffinity(1 + gqlang.IntScalar)
	floatAffinity   = scalarAffinity(1 + gqlang.FloatScalar)
)

func scalarTypeAffinity(styp gqlang.ScalarType) scalarAffinity {
	return 1 + scalarAffinity(styp)
}

func posListToLocationList(source string, posList []gqlang.Pos) []Location {
	locList := make([]Location, len(posList))
	for i := range locList {
		locList[i] = astPositionToLocation(posList[i].ToPosition(source))
	}
	return locList
}
