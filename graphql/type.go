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
	scalar   string
	listElem *gqlType
	obj      *objectType
	input    *inputObjectType
	nonNull  bool

	// nullVariant is the same type with the nonNull flag flipped.
	// This is to ensure that either version of the type has a consistent address.
	nullVariant *gqlType

	listInit sync.Once
	listOf_  *gqlType
}

type objectType struct {
	name   string
	fields map[string]objectTypeField
}

type objectTypeField struct {
	typ  *gqlType
	args map[string]inputValueDefinition
}

type inputObjectType struct {
	name   string
	fields map[string]inputValueDefinition
}

type inputValueDefinition struct {
	// defaultValue.typ will always be set. Most of the time, defaultValue
	// is valid value of the type. However, if the type is non-nullable and
	// does not have a default, the value will be typed null.
	//
	// This is the only way to distinguish not having a default from having a
	// null default, but it's the only situation in which not having a default is
	// relevant in the GraphQL specification.
	defaultValue Value
}

func (ivd inputValueDefinition) typ() *gqlType {
	return ivd.defaultValue.typ
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

func newInputObjectType(info *inputObjectType) *gqlType {
	nullable := &gqlType{input: info}
	nonNullable := &gqlType{input: info, nonNull: true}
	nullable.nullVariant = nonNullable
	nonNullable.nullVariant = nullable
	return nullable
}

func listOf(elem *gqlType) *gqlType {
	elem.listInit.Do(func() {
		nullable := &gqlType{listElem: elem}
		nonNullable := &gqlType{listElem: elem, nonNull: true}
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
		return "[" + typ.listElem.String() + "]" + suffix
	case typ.isObject():
		return typ.obj.name + suffix
	case typ.isInputObject():
		return typ.input.name + suffix
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
	return typ.listElem != nil
}

func (typ *gqlType) isObject() bool {
	return typ.obj != nil
}

func (typ *gqlType) isInputObject() bool {
	return typ.input != nil
}

// isInputType reports whether typ can be used as an input.
// See https://graphql.github.io/graphql-spec/June2018/#IsInputType()
func (typ *gqlType) isInputType() bool {
	for typ.isList() {
		typ = typ.listElem
	}
	// TODO(soon): Enum.
	return typ.isScalar() || typ.isInputObject()
}

// isOutputType reports whether typ can be used as an output.
// See https://graphql.github.io/graphql-spec/June2018/#IsOutputType()
func (typ *gqlType) isOutputType() bool {
	for typ.isList() {
		typ = typ.listElem
	}
	// TODO(soon): Interface, union, or enum.
	return typ.isScalar() || typ.isObject()
}

func (typ *gqlType) selectionSetType() *gqlType {
	for typ.isList() {
		typ = typ.listElem
	}
	if !typ.isObject() {
		return nil
	}
	return typ
}

// areTypesCompatible reports if a value variableType can be passed to a usage
// expecting locationType. See https://graphql.github.io/graphql-spec/June2018/#AreTypesCompatible()
func areTypesCompatible(locationType, variableType *gqlType) bool {
	for {
		switch {
		case !locationType.isNullable():
			if variableType.isNullable() {
				return false
			}
			locationType = locationType.toNullable()
			variableType = variableType.toNullable()
		case !variableType.isNullable():
			variableType = variableType.toNullable()
		case locationType.isList():
			if !variableType.isList() {
				return false
			}
			locationType = locationType.listElem
			variableType = variableType.listElem
		case variableType.isList():
			return false
		default:
			return locationType == variableType
		}
	}
}
