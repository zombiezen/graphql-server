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
	fields []*selectionField
}

func newSelectionSet(ast *gqlang.SelectionSet) *SelectionSet {
	if ast == nil {
		return nil
	}
	set := new(SelectionSet)
	for _, sel := range ast.Sel {
		if sel.Field != nil {
			// TODO(soon): args
			field := &selectionField{
				name: sel.Field.Name.Value,
				key:  sel.Field.Name.Value,
				sub:  newSelectionSet(sel.Field.SelectionSet),
			}
			if sel.Field.Alias != nil {
				field.key = sel.Field.Alias.Value
			}
			set.fields = append(set.fields, field)
		}
	}
	return set
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

// Arguments returns all the different arguments that the given field is
// invoked with. The caller must not modify the returned maps.
func (sel *SelectionSet) Arguments(name string) []map[string]Value {
	if sel == nil {
		return nil
	}
	var args []map[string]Value
	for _, f := range sel.fields {
		if f.name == name {
			args = append(args, f.args)
		}
	}
	return args
}

type selectionField struct {
	// key is the response object key to be used. Usually the same as name.
	key string
	// name is the object field name.
	name string

	args map[string]Value
	sub  *SelectionSet
}
