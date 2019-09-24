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

import "sync"

// gqlType represents a GraphQL type.
//
// Types can be compared for equality using ==. Types with the same name from
// different schemas are never equal.
type gqlType struct {
	scalar  string
	list    *gqlType
	obj     *objectType
	nonNull bool

	// nullVariant is the same type with the nonNull flag flipped.
	// This is to ensure that either version of the type has a consistent address.
	nullVariant *gqlType

	listInit sync.Once
	listOf_  *gqlType
}

type objectType struct {
	name   string
	fields map[string]*gqlType
}

// Predefined types.
var (
	intType     = newScalarType("Int")
	floatType   = newScalarType("Float")
	stringType  = newScalarType("String")
	booleanType = newScalarType("Boolean")
	idType      = newScalarType("ID")
)

func newScalarType(name string) *gqlType {
	nullable := &gqlType{scalar: name}
	nonNullable := &gqlType{scalar: name, nonNull: true}
	nullable.nullVariant = nonNullable
	nonNullable.nullVariant = nullable
	return nullable
}

func newObjectType(info *objectType) *gqlType {
	nullable := &gqlType{obj: info}
	nonNullable := &gqlType{obj: info, nonNull: true}
	nullable.nullVariant = nonNullable
	nonNullable.nullVariant = nullable
	return nullable
}

func listOf(elem *gqlType) *gqlType {
	elem.listInit.Do(func() {
		nullable := &gqlType{list: elem}
		nonNullable := &gqlType{list: elem, nonNull: true}
		nullable.nullVariant = nonNullable
		nonNullable.nullVariant = nullable
		elem.listOf_ = nullable
	})
	return elem.listOf_
}

// String returns the type reference string.
func (typ *gqlType) String() string {
	suffix := ""
	if typ.nonNull {
		suffix = "!"
	}
	switch {
	case typ == nil:
		return "<nil>"
	case typ.isScalar():
		return typ.scalar + suffix
	case typ.isList():
		return "[" + typ.list.String() + "]" + suffix
	case typ.isObject():
		return typ.obj.name + suffix
	default:
		return "<invalid type>"
	}
}

// isNullable reports whether the type permits null.
func (typ *gqlType) isNullable() bool {
	return !typ.nonNull
}

func (typ *gqlType) toNullable() *gqlType {
	if typ.isNullable() {
		return typ
	}
	return typ.nullVariant
}

func (typ *gqlType) toNonNullable() *gqlType {
	if !typ.isNullable() {
		return typ
	}
	return typ.nullVariant
}

func (typ *gqlType) isScalar() bool {
	return typ.scalar != ""
}

func (typ *gqlType) isList() bool {
	return typ.list != nil
}

func (typ *gqlType) isObject() bool {
	return typ.obj != nil
}

func (typ *gqlType) needsSelectionSet() bool {
	for typ.isList() {
		typ = typ.list
	}
	return typ.isObject()
}
