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
	"zombiezen.com/go/graphql-server/internal/gqlang"
)

// A SelectionSet is a collection of object fields that a client is requesting
// for the server to return. The zero value or nil is an empty set.
type SelectionSet struct {
	fields []*SelectedField
}

// newSelectionSet returns a new selection set from the request's AST.
// It assumes that the AST has been validated.
func newSelectionSet(source string, variables map[string]Value, typ *objectType, ast *gqlang.SelectionSet) (*SelectionSet, []error) {
	if ast == nil {
		return nil, nil
	}
	set := new(SelectionSet)
	var errs []error
	for _, sel := range ast.Sel {
		if sel.Field != nil {
			name := sel.Field.Name.Value
			fieldInfo := typ.field(name)
			// Validation determines whether this is a valid reference to the
			// reserved fields.
			switch name {
			case typeNameFieldName:
				fieldInfo = typeNameField()
			case schemaFieldName:
				fieldInfo = schemaField()
			case typeByNameFieldName:
				fieldInfo = typeByNameField()
			}
			field := &SelectedField{
				name: name,
				key:  name,
				loc:  astPositionToLocation(sel.Field.Start().ToPosition(source)),
			}
			if sel.Field.Alias != nil {
				field.key = sel.Field.Alias.Value
			}
			set.fields = append(set.fields, field)

			if fieldSelType := fieldInfo.typ.selectionSetType(); fieldSelType != nil {
				var subErrs []error
				field.sub, subErrs = newSelectionSet(source, variables, fieldSelType.obj, sel.Field.SelectionSet)
				for _, err := range subErrs {
					errs = append(errs, wrapFieldError(field.key, field.loc, err))
				}
			}
			var argErrs []error
			field.args, argErrs = coerceArgumentValues(source, variables, fieldInfo, sel.Field.Arguments)
			for _, err := range argErrs {
				errs = append(errs, wrapFieldError(field.key, field.loc, err))
			}
		}
	}
	return set, errs
}

// Has reports whether the selection set includes the field with the given name.
func (sel *SelectionSet) Has(name string) bool {
	if sel == nil {
		return false
	}
	for _, f := range sel.fields {
		if f.name == name {
			return true
		}
	}
	return false
}

// FieldsWithName returns the fields in the selection set with the given name.
// There may be multiple in the case of a field alias.
func (sel *SelectionSet) FieldsWithName(name string) []*SelectedField {
	if sel == nil {
		return nil
	}
	var fields []*SelectedField
	for _, f := range sel.fields {
		if f.name == name {
			fields = append(fields, f)
		}
	}
	return fields
}

// SelectedField is a field in a selection set.
type SelectedField struct {
	// key is the response object key to be used. Usually the same as name.
	key string
	// name is the object field name.
	name string

	loc  Location
	args map[string]Value
	sub  *SelectionSet
}

// Arg returns the argument with the given name or a null Value if the argument
// doesn't exist.
func (f *SelectedField) Arg(name string) Value {
	return f.args[name]
}

// SelectionSet returns the field's selection set or nil if the field doesn't
// have one.
func (f *SelectedField) SelectionSet() *SelectionSet {
	return f.sub
}
