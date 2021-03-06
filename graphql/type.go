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
	"sync"
)

// gqlType represents a GraphQL type.
//
// Types can be compared for equality using ==. Types with the same name from
// different schemas are never equal.
type gqlType struct {
	description string

	scalar   string
	enum     *enumType
	listElem *gqlType
	obj      *objectType
	union    *unionType
	input    *inputObjectType
	nonNull  bool

	// nullVariant is the same type with the nonNull flag flipped.
	// This is to ensure that either version of the type has a consistent address.
	nullVariant *gqlType

	listInit sync.Once
	listOf_  *gqlType
}

type enumType struct {
	name   string
	values []enumValue
}

func (enum *enumType) has(sym string) bool {
	for _, v := range enum.values {
		if v.name == sym {
			return true
		}
	}
	return false
}

type enumValue struct {
	name        string
	description string

	deprecated        bool
	deprecationReason NullString
}

func (v enumValue) Name() string {
	return v.name
}

func (v enumValue) Description() NullString {
	return NullString{S: v.description, Valid: v.description != ""}
}

func (v enumValue) IsDeprecated() bool {
	return v.deprecated
}

func (v enumValue) DeprecationReason() NullString {
	return v.deprecationReason
}

type objectType struct {
	name   string
	fields []objectTypeField
}

func (obj *objectType) field(name string) *objectTypeField {
	for i := range obj.fields {
		if obj.fields[i].name == name {
			return &obj.fields[i]
		}
	}
	return nil
}

type unionType struct {
	name          string
	possibleTypes []*gqlType // all nullable object types
}

type inputObjectType struct {
	name   string
	fields inputValueDefinitionList
}

// Predefined types.
var (
	intType     = newScalarType("Int", "")
	floatType   = newScalarType("Float", "")
	stringType  = newScalarType("String", "")
	booleanType = newScalarType("Boolean", "")
	idType      = newScalarType("ID", "")
)

func newScalarType(name, description string) *gqlType {
	nullable := &gqlType{
		scalar:      name,
		description: description,
	}
	nonNullable := &gqlType{
		scalar:      name,
		description: description,
		nonNull:     true,
	}
	nullable.nullVariant = nonNullable
	nonNullable.nullVariant = nullable
	return nullable
}

func newEnumType(info *enumType, description string) *gqlType {
	nullable := &gqlType{
		enum:        info,
		description: description,
	}
	nonNullable := &gqlType{
		description: description,
		enum:        info,
		nonNull:     true,
	}
	nullable.nullVariant = nonNullable
	nonNullable.nullVariant = nullable
	return nullable
}

func newObjectType(info *objectType, description string) *gqlType {
	nullable := &gqlType{
		obj:         info,
		description: description,
	}
	nonNullable := &gqlType{
		obj:         info,
		description: description,
		nonNull:     true,
	}
	nullable.nullVariant = nonNullable
	nonNullable.nullVariant = nullable
	return nullable
}

func newUnionType(info *unionType, description string) *gqlType {
	nullable := &gqlType{
		union:       info,
		description: description,
	}
	nonNullable := &gqlType{
		union:       info,
		description: description,
		nonNull:     true,
	}
	nullable.nullVariant = nonNullable
	nonNullable.nullVariant = nullable
	return nullable
}

func newInputObjectType(info *inputObjectType, description string) *gqlType {
	nullable := &gqlType{
		input:       info,
		description: description,
	}
	nonNullable := &gqlType{
		input:       info,
		description: description,
		nonNull:     true,
	}
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
	case typ.isEnum():
		return typ.enum.name + suffix
	case typ.isList():
		return "[" + typ.listElem.String() + "]" + suffix
	case typ.isObject():
		return typ.obj.name + suffix
	case typ.isUnion():
		return typ.union.name + suffix
	case typ.isInputObject():
		return typ.input.name + suffix
	default:
		return "<invalid type>"
	}
}

// Kind returns the __TypeKind field for introspection.
func (typ *gqlType) Kind() string {
	switch {
	case !typ.isNullable():
		return "NON_NULL"
	case typ.isScalar():
		return "SCALAR"
	case typ.isObject():
		return "OBJECT"
	case typ.isUnion():
		return "UNION"
	case typ.isEnum():
		return "ENUM"
	case typ.isInputObject():
		return "INPUT_OBJECT"
	case typ.isList():
		return "LIST"
	default:
		panic("invalid type")
	}
}

// Name returns the name field for introspection.
func (typ *gqlType) Name() NullString {
	switch {
	case !typ.isNullable():
		// Non-null is a wrapper.
		return NullString{}
	case typ.isScalar():
		return NullString{S: typ.scalar, Valid: true}
	case typ.isObject():
		return NullString{S: typ.obj.name, Valid: true}
	case typ.isUnion():
		return NullString{S: typ.union.name, Valid: true}
	case typ.isEnum():
		return NullString{S: typ.enum.name, Valid: true}
	case typ.isInputObject():
		return NullString{S: typ.input.name, Valid: true}
	default:
		return NullString{}
	}
}

// Description returns the type's documentation.
func (typ *gqlType) Description() NullString {
	return NullString{S: typ.description, Valid: typ.description != ""}
}

// Fields returns the list of object fields.
func (typ *gqlType) Fields(args map[string]Value) *[]objectTypeField {
	if !typ.isObject() {
		return nil
	}
	var fields []objectTypeField
	for _, f := range typ.obj.fields {
		if !f.deprecated || args["includeDeprecated"].Boolean() {
			fields = append(fields, f)
		}
	}
	return &fields
}

// Interfaces is not implemented.
func (typ *gqlType) Interfaces() *[]interface{} {
	if !typ.isObject() {
		return nil
	}
	return new([]interface{})
}

// PossibleTypes returns the list of types that a union or interface can represent.
func (typ *gqlType) PossibleTypes() *[]*gqlType {
	// TODO(someday): Interface.
	switch {
	case typ.isUnion():
		return &typ.union.possibleTypes
	default:
		return nil
	}
}

// EnumValues returns the list of permitted values for an enumeration type.
func (typ *gqlType) EnumValues(args map[string]Value) *[]enumValue {
	if !typ.isEnum() {
		return nil
	}
	var vals []enumValue
	for _, v := range typ.enum.values {
		if !v.deprecated || args["includeDeprecated"].Boolean() {
			vals = append(vals, v)
		}
	}
	return &vals
}

// InputValues returns the list of input object fields.
func (typ *gqlType) InputFields() *[]inputValueDefinition {
	if !typ.isInputObject() {
		return nil
	}
	values := append([]inputValueDefinition(nil), typ.input.fields...)
	return &values
}

// OfType returns the element type of non-nullable or list types.
func (typ *gqlType) OfType() *gqlType {
	switch {
	case !typ.isNullable():
		return typ.toNullable()
	case typ.isList():
		return typ.listElem
	default:
		return nil
	}
}

// isNullable reports whether the type permits null.
func (typ *gqlType) isNullable() bool {
	return !typ.nonNull
}

func (typ *gqlType) toNullable() *gqlType {
	if typ == nil || typ.isNullable() {
		return typ
	}
	return typ.nullVariant
}

func (typ *gqlType) toNonNullable() *gqlType {
	if typ == nil || !typ.isNullable() {
		return typ
	}
	return typ.nullVariant
}

func (typ *gqlType) isScalar() bool {
	return typ.scalar != ""
}

func (typ *gqlType) isEnum() bool {
	return typ.enum != nil
}

func (typ *gqlType) isList() bool {
	return typ.listElem != nil
}

func (typ *gqlType) isObject() bool {
	return typ.obj != nil
}

func (typ *gqlType) isUnion() bool {
	return typ.union != nil
}

func (typ *gqlType) isInputObject() bool {
	return typ.input != nil
}

// isInputType reports whether typ can be used as an input.
// See https://graphql.github.io/graphql-spec/June2018/#IsInputType%28%29
func (typ *gqlType) isInputType() bool {
	for typ.isList() {
		typ = typ.listElem
	}
	return typ.isScalar() || typ.isEnum() || typ.isInputObject()
}

// isOutputType reports whether typ can be used as an output.
// See https://graphql.github.io/graphql-spec/June2018/#IsOutputType%28%29
func (typ *gqlType) isOutputType() bool {
	for typ.isList() {
		typ = typ.listElem
	}
	// TODO(someday): Or interface.
	return typ.isScalar() || typ.isEnum() || typ.isObject() || typ.isUnion()
}

// selectionSetType returns the type used for selection sets or nil if the type
// is not a composite type. This is largely to unwrap lists.
func (typ *gqlType) selectionSetType() *gqlType {
	for typ.isList() {
		typ = typ.listElem
	}
	// TODO(someday): Permit interface.
	if !typ.isObject() && !typ.isUnion() {
		return nil
	}
	return typ
}

// isAbstract reports whether the type is a union or interface.
func (typ *gqlType) isAbstract() bool {
	// TODO(someday): Or interface.
	return typ.isUnion()
}

// possibleTypes returns the set of non-abstract types that an object of this
// type could be at runtime. All types are normalized to nullable types.
// See https://graphql.github.io/graphql-spec/June2018/#GetPossibleTypes%28%29
func (typ *gqlType) possibleTypes() map[*gqlType]struct{} {
	// TODO(someday): Add more for interface.
	switch {
	case typ.isUnion():
		possible := make(map[*gqlType]struct{}, len(typ.union.possibleTypes))
		for _, t := range typ.union.possibleTypes {
			possible[t] = struct{}{}
		}
		return possible
	default:
		return map[*gqlType]struct{}{typ.toNullable(): {}}
	}
}

// areTypesCompatible reports if a value variableType can be passed to a usage
// expecting locationType. See https://graphql.github.io/graphql-spec/June2018/#AreTypesCompatible%28%29
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

type objectTypeField struct {
	name        string
	description string
	typ         *gqlType
	args        inputValueDefinitionList

	deprecated        bool
	deprecationReason NullString
}

func (f *objectTypeField) Name() string {
	return f.name
}

func (f *objectTypeField) Description() NullString {
	return NullString{S: f.description, Valid: f.description != ""}
}

func (f *objectTypeField) Args() []inputValueDefinition {
	args := append([]inputValueDefinition(nil), f.args...)
	return args
}

func (f objectTypeField) Type() *gqlType {
	return f.typ
}

func (f objectTypeField) IsDeprecated() bool {
	return f.deprecated
}

func (f objectTypeField) DeprecationReason() NullString {
	return f.deprecationReason
}

type inputValueDefinition struct {
	name        string
	description string

	// defaultValue.typ will always be set. Most of the time, defaultValue
	// is valid value of the type. However, if the type is non-nullable and
	// does not have a default, the value will be typed null: this indicates a
	// required argument or input field.
	defaultValue Value
}

func (ivd inputValueDefinition) Name() string {
	return ivd.name
}

func (ivd inputValueDefinition) Description() NullString {
	return NullString{S: ivd.description, Valid: ivd.description != ""}
}

func (ivd inputValueDefinition) Type() *gqlType {
	return ivd.defaultValue.typ
}

// DefaultValue returns the default value in GraphQL syntax.
// https://graphql.github.io/graphql-spec/June2018/#sec-The-__InputValue-Type
func (ivd inputValueDefinition) DefaultValue() NullString {
	if ivd.defaultValue.IsNull() {
		return NullString{}
	}
	return NullString{
		S:     ivd.defaultValue.String(),
		Valid: true,
	}
}

type inputValueDefinitionList []inputValueDefinition

func (list inputValueDefinitionList) byName(name string) *inputValueDefinition {
	for i := range list {
		if list[i].name == name {
			return &list[i]
		}
	}
	return nil
}
