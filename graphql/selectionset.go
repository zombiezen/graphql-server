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
	sel := new(SelectionSet)
	errs := sel.merge(source, variables, typ, ast)
	return sel, errs
}

// merge merges the fields from the given AST, which is assumed to have been
// validated. This combines the duties of CollectFields() and
// MergeSelectionSets() from the spec.
//
// See https://graphql.github.io/graphql-spec/June2018/#sec-Field-Collection and
// https://graphql.github.io/graphql-spec/June2018/#MergeSelectionSets%28%29
func (sel *SelectionSet) merge(source string, variables map[string]Value, typ *objectType, ast *gqlang.SelectionSet) []error {
	if ast == nil {
		return nil
	}
	var errs []error
	for _, s := range ast.Sel {
		if s.Field != nil {
			errs = append(errs, sel.addField(source, variables, typ, s.Field)...)
		}
	}
	return errs
}

func (sel *SelectionSet) addField(source string, variables map[string]Value, typ *objectType, f *gqlang.Field) []error {
	name := f.Name.Value
	key := name
	if f.Alias != nil {
		key = f.Alias.Value
	}
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

	field := sel.find(key)
	var errs []error
	if field == nil {
		// Validation rejects any fields that have the same key but differing
		// invocations. Only evaluate the arguments from the first one, then merge
		// the selection set of subsequent ones.
		field = &SelectedField{
			name: name,
			key:  key,
			loc:  astPositionToLocation(f.Start().ToPosition(source)),
		}
		sel.fields = append(sel.fields, field)

		var argErrs []error
		field.args, argErrs = coerceArgumentValues(source, variables, fieldInfo, f.Arguments)
		for _, err := range argErrs {
			errs = append(errs, wrapFieldError(field.key, field.loc, err))
		}
		if fieldInfo.typ.selectionSetType() != nil {
			field.sub = new(SelectionSet)
		}
	}

	if fieldSelType := fieldInfo.typ.selectionSetType(); fieldSelType != nil {
		subErrs := field.sub.merge(source, variables, fieldSelType.obj, f.SelectionSet)
		for _, err := range subErrs {
			errs = append(errs, wrapFieldError(field.key, field.loc, err))
		}
	}
	return errs
}

func (sel *SelectionSet) find(key string) *SelectedField {
	if sel == nil {
		return nil
	}
	for _, f := range sel.fields {
		if f.key == key {
			return f
		}
	}
	return nil
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
	// loc is the first location that the field encounters.
	loc Location
	// args holds the argument values that the field will be invoked with.
	args map[string]Value
	// sub is set for fields that have a composite type.
	sub *SelectionSet
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
