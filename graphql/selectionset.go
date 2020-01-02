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
	"strings"

	"golang.org/x/xerrors"
	"zombiezen.com/go/graphql-server/internal/gqlang"
)

// A SelectionSet is a collection of object fields that a client is requesting
// for the server to return. The zero value or nil is an empty set.
type SelectionSet struct {
	fields []*SelectedField
}

// selectionSetScope defines the symbols and position information used
// during validation.
type selectionSetScope struct {
	source    string
	doc       *gqlang.Document // for the fragments
	types     map[string]*gqlType
	variables map[string]Value
}

// newSelectionSet returns a new selection set from the request's AST.
// It assumes that the AST has been validated.
func newSelectionSet(s *selectionSetScope, typ *gqlType, ast *gqlang.SelectionSet) (*SelectionSet, []error) {
	if ast == nil {
		return nil, nil
	}
	sel := new(SelectionSet)
	errs := sel.merge(s, typ, ast)
	return sel, errs
}

// merge merges the fields from the given AST, which is assumed to have been
// validated. This combines the duties of CollectFields() and
// MergeSelectionSets() from the spec.
//
// See https://graphql.github.io/graphql-spec/June2018/#sec-Field-Collection and
// https://graphql.github.io/graphql-spec/June2018/#MergeSelectionSets%28%29
func (set *SelectionSet) merge(s *selectionSetScope, typ *gqlType, ast *gqlang.SelectionSet) []error {
	if ast == nil {
		return nil
	}
	var errs []error
	for _, sel := range ast.Sel {
		switch {
		case sel.Field != nil:
			errs = append(errs, set.addField(s, typ, sel.Field)...)
		case sel.FragmentSpread != nil:
			name := sel.FragmentSpread.Name.Value
			frag := s.doc.FindFragment(name)
			fragType := s.types[frag.Type.Name.Value]
			mergeErrs := set.merge(s, fragType, frag.SelectionSet)
			for _, err := range mergeErrs {
				errs = append(errs, xerrors.Errorf("fragment %s: %w", name, err))
			}
		case sel.InlineFragment != nil:
			fragType := typ
			if sel.InlineFragment.Type != nil {
				fragType = s.types[sel.InlineFragment.Type.Name.Value]
			}
			errs = append(errs, set.merge(s, fragType, sel.InlineFragment.SelectionSet)...)
		default:
			panic("unknown selection type")
		}
	}
	return errs
}

func (set *SelectionSet) addField(s *selectionSetScope, typ *gqlType, f *gqlang.Field) []error {
	name := f.Name.Value
	key := name
	if f.Alias != nil {
		key = f.Alias.Value
	}
	// TODO(someday): Make generic for interface or union.
	fieldInfo := typ.obj.field(name)
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

	field := set.find(key)
	var errs []error
	if field == nil {
		// Validation rejects any fields that have the same key but differing
		// invocations. Only evaluate the arguments from the first one, then merge
		// the selection set of subsequent ones.
		field = &SelectedField{
			name: name,
			key:  key,
			loc:  astPositionToLocation(f.Start().ToPosition(s.source)),
		}
		set.fields = append(set.fields, field)

		var argErrs []error
		field.args, argErrs = coerceArgumentValues(s, fieldInfo.args, f.Arguments)
		for _, err := range argErrs {
			errs = append(errs, wrapFieldError(field.key, field.loc, err))
		}
		if fieldInfo.typ.selectionSetType() != nil {
			field.sub = new(SelectionSet)
		}
	}

	if fieldSelType := fieldInfo.typ.selectionSetType(); fieldSelType != nil {
		subErrs := field.sub.merge(s, fieldSelType, f.SelectionSet)
		for _, err := range subErrs {
			errs = append(errs, wrapFieldError(field.key, field.loc, err))
		}
	}
	return errs
}

func (set *SelectionSet) find(key string) *SelectedField {
	if set == nil {
		return nil
	}
	for _, f := range set.fields {
		if f.key == key {
			return f
		}
	}
	return nil
}

// Len returns the number of fields in the selection set.
func (set *SelectionSet) Len() int {
	if set == nil {
		return 0
	}
	return len(set.fields)
}

// Field returns the i'th field in the selection set. Field will panic if i is
// not in the range [0, set.Len()).
func (set *SelectionSet) Field(i int) *SelectedField {
	// Calling Field on a nil set is invalid, so letting the nil pointer
	// dereference panic.
	return set.fields[i]
}

// Has reports whether the selection set includes the field with the given name.
//
// The argument may contain dots to check for subfields. For example, Has("a.b")
// returns whether the selection set contains a field "a" whose selection set
// contains a field "b".
func (set *SelectionSet) Has(name string) bool {
	return set.HasAny(name)
}

// HasAny reports whether the selection set includes fields with any of the
// given names. If no names are given, then HasAny returns false.
//
// The argument may contain dots to check for subfields. For example,
// HasAny("a.b") returns whether the selection set contains a field "a" whose
// selection set contains a field "b".
func (set *SelectionSet) HasAny(names ...string) bool {
	type frame struct {
		fields []*SelectedField
		query  fieldTree
	}

	if set == nil || len(names) == 0 {
		return false
	}
	stack := []frame{{
		fields: set.fields,
		query:  newFieldTree(names),
	}}
	for len(stack) > 0 {
		currFrame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, f := range currFrame.fields {
			node := currFrame.query[f.name]
			if node.selected {
				return true
			}
			if f.sub != nil && len(currFrame.query) > 0 {
				stack = append(stack, frame{
					fields: f.sub.fields,
					query:  node.subtree,
				})
			}
		}
	}
	return false
}

// OnlyUses returns true if and only if the selection set does not include
// fields beyond those given as arguments and __typename.
//
// The arguments may contain dots to check for subfields. For example, "a.b"
// refers to a field named "b" inside the selection set of a field named "a".
func (set *SelectionSet) OnlyUses(names ...string) bool {
	type frame struct {
		fields  []*SelectedField
		allowed fieldTree
	}

	stack := []frame{{
		fields:  set.fields,
		allowed: newFieldTree(names),
	}}
	for len(stack) > 0 {
		currFrame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, f := range currFrame.fields {
			if f.name != typeNameFieldName && !currFrame.allowed.has(f.name) {
				return false
			}
			if f.sub != nil && !currFrame.allowed[f.name].selected {
				stack = append(stack, frame{
					fields:  f.sub.fields,
					allowed: currFrame.allowed[f.name].subtree,
				})
			}
		}
	}
	return true
}

// A fieldTree stores a tree of field names.
type fieldTree map[string]fieldTreeNode

type fieldTreeNode struct {
	selected bool
	subtree  fieldTree
}

func newFieldTree(names []string) fieldTree {
	tree := make(fieldTree)
	for _, name := range names {
		name = strings.TrimLeft(name, ".")
		curr := tree
		for len(name) > 0 {
			var part string
			if i := strings.IndexByte(name, '.'); i != -1 {
				part, name = name[:i], strings.TrimLeft(name[i+1:], ".")
			} else {
				part, name = name, ""
			}
			node := curr[part]
			if len(name) == 0 {
				node.selected = true
				curr[part] = node
			} else if node.subtree == nil {
				node.subtree = make(fieldTree)
				curr[part] = node
			}
			curr = node.subtree
		}
	}
	return tree
}

func (tree fieldTree) has(part string) bool {
	_, ok := tree[part]
	return ok
}

// FieldsWithName returns the fields in the selection set with the given name.
// There may be multiple in the case of a field alias.
func (set *SelectionSet) FieldsWithName(name string) []*SelectedField {
	if set == nil {
		return nil
	}
	var fields []*SelectedField
	for _, f := range set.fields {
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

func (f *SelectedField) toRequest() FieldRequest {
	return FieldRequest{
		Name:      f.name,
		Args:      f.args,
		Selection: f.sub,
	}
}

// Name returns the name of the field. This may be different than the key used
// in the response when the query is using aliases.
func (f *SelectedField) Name() string {
	return f.name
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
