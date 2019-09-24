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

// Package gqlang provides a parser for the GraphQL language.
package gqlang

import "fmt"

// Document is a parsed GraphQL source.
// https://graphql.github.io/graphql-spec/June2018/#sec-Language.Document
type Document struct {
	Definitions []*Definition
}

// Definition is a top-level GraphQL construct like an operation, a fragment, or
// a type. Only one of its fields will be set.
// https://graphql.github.io/graphql-spec/June2018/#sec-Language.Document
type Definition struct {
	Operation *Operation
	Type      *TypeDefinition
}

// Start returns the position of the definition's first token.
func (defn *Definition) Start() Pos {
	switch {
	case defn.Operation != nil:
		return defn.Operation.Start
	case defn.Type != nil:
		return defn.Type.Start()
	default:
		panic("unknown definition")
	}
}

// Operation is a query, a mutation, or a subscription.
// https://graphql.github.io/graphql-spec/June2018/#sec-Language.Operations
type Operation struct {
	Start               Pos
	Type                OperationType
	Name                *Name
	VariableDefinitions *VariableDefinitions
	SelectionSet        *SelectionSet
}

func (op *Operation) asDefinition() *Definition {
	if op == nil {
		return nil
	}
	return &Definition{Operation: op}
}

// OperationType is one of query, mutation, or subscription.
type OperationType int

// Types of operation.
const (
	Query OperationType = iota
	Mutation
	Subscription
)

// String returns the keyword that corresponds to the operation type.
func (typ OperationType) String() string {
	switch typ {
	case Query:
		return "query"
	case Mutation:
		return "mutation"
	case Subscription:
		return "subscription"
	default:
		return fmt.Sprintf("OperationType(%d)", int(typ))
	}
}

// SelectionSet is the set of information an operation requests.
// https://graphql.github.io/graphql-spec/June2018/#SelectionSet
type SelectionSet struct {
	LBrace Pos
	Sel    []*Selection
	RBrace Pos
}

// A Selection is either a field or a fragment.
// https://graphql.github.io/graphql-spec/June2018/#sec-Selection-Sets
type Selection struct {
	Field *Field
}

// A Field is a discrete piece of information available to request within a
// selection set.
// https://graphql.github.io/graphql-spec/June2018/#sec-Language.Fields
type Field struct {
	Alias        *Name
	Name         *Name
	Arguments    *Arguments
	SelectionSet *SelectionSet
}

// End returns the byte offset after the end of the field.
func (f *Field) End() Pos {
	if f.SelectionSet != nil {
		return f.SelectionSet.RBrace + 1
	}
	if f.Arguments != nil {
		return f.Arguments.RParen + 1
	}
	return f.Name.End()
}

// Arguments is a set of named arguments on a field.
// https://graphql.github.io/graphql-spec/June2018/#sec-Language.Arguments
type Arguments struct {
	LParen Pos
	Args   []*Argument
	RParen Pos
}

// Argument is a single element in Arguments.
// https://graphql.github.io/graphql-spec/June2018/#sec-Language.Arguments
type Argument struct {
	Name  *Name
	Colon Pos
	Value *InputValue
}

// An InputValue is a scalar or a variable reference.
// https://graphql.github.io/graphql-spec/June2018/#sec-Input-Values
type InputValue struct {
	Scalar      *ScalarValue
	VariableRef *Variable
}

// ScalarValue is a primitive literal like a string or integer.
type ScalarValue struct {
	Start Pos
	Type  ScalarType
	Raw   string
}

// String returns sval.Raw.
func (sval *ScalarValue) String() string {
	return sval.Raw
}

// AsBool reads the scalar's boolean value.
func (sval *ScalarValue) AsBool() bool {
	if sval.Type != BooleanScalar {
		return false
	}
	return sval.Raw == "true"
}

// ScalarType indicates the type of a ScalarValue.
type ScalarType int

// Scalar types.
const (
	NullScalar ScalarType = iota
	BooleanScalar
	EnumScalar
	IntScalar
	FloatScalar
	StringScalar
)

// A Variable is an input to a GraphQL operation.
// https://graphql.github.io/graphql-spec/June2018/#Variable
type Variable struct {
	Dollar Pos
	Name   *Name
}

// DefaultValue specifies the default value of an input.
// https://graphql.github.io/graphql-spec/June2018/#DefaultValue
type DefaultValue struct {
	Eq    Pos
	Value *InputValue
}

// String returns the variable in the form "$foo".
func (v *Variable) String() string {
	return "$" + v.Name.String()
}

// VariableDefinitions is the set of variables an operation defines.
// https://graphql.github.io/graphql-spec/June2018/#Variable
type VariableDefinitions struct {
	LParen Pos
	Defs   []*VariableDefinition
	RParen Pos
}

// VariableDefinition is an element of VariableDefinitions.
// https://graphql.github.io/graphql-spec/June2018/#Variable
type VariableDefinition struct {
	Var     *Variable
	Colon   Pos
	Type    *TypeRef
	Default *DefaultValue
}

// A Name is an identifier.
// https://graphql.github.io/graphql-spec/June2018/#sec-Names
type Name struct {
	Value string
	Start Pos
}

// End returns the position of the byte after the last character of the name.
func (n *Name) End() Pos {
	return n.Start + Pos(len(n.Value))
}

// String returns the name or the empty string if the name is nil.
func (n *Name) String() string {
	if n == nil {
		return ""
	}
	return n.Value
}

// A TypeRef is a named type, a list type, or a non-null type.
// https://graphql.github.io/graphql-spec/June2018/#Type
type TypeRef struct {
	Named   *Name
	List    *ListType
	NonNull *NonNullType
}

// ListType declares a homogenous sequence of another type.
// https://graphql.github.io/graphql-spec/June2018/#ListType
type ListType struct {
	LBracket Pos
	Type     *TypeRef
	RBracket Pos
}

// NonNullType declares a named or list type that cannot be null.
// https://graphql.github.io/graphql-spec/June2018/#Type
type NonNullType struct {
	Named *Name
	List  *ListType
	Pos   Pos
}

// A Description is a string that documents a type system definition.
// https://graphql.github.io/graphql-spec/June2018/#Description
type Description struct {
	Start Pos
	Raw   string
}

// TypeDefinition holds a type definition.
// https://graphql.github.io/graphql-spec/June2018/#TypeDefinition
type TypeDefinition struct {
	// One of the following must be non-nil:

	Scalar      *ScalarTypeDefinition
	Object      *ObjectTypeDefinition
	InputObject *InputObjectTypeDefinition
}

// Start returns the position of the type definition's first token.
func (defn *TypeDefinition) Start() Pos {
	switch {
	case defn.Scalar != nil:
		return defn.Scalar.Keyword
	case defn.Object != nil:
		return defn.Object.Keyword
	case defn.InputObject != nil:
		return defn.InputObject.Keyword
	default:
		panic("unknown type definition")
	}
}

// Description returns the type definition's description or nil if it does not
// have one.
func (defn *TypeDefinition) Description() *Description {
	switch {
	case defn == nil:
		return nil
	case defn.Scalar != nil:
		return defn.Scalar.Description
	case defn.Object != nil:
		return defn.Object.Description
	case defn.InputObject != nil:
		return defn.InputObject.Description
	default:
		return nil
	}
}

// Name returns the type definition's name.
func (defn *TypeDefinition) Name() *Name {
	switch {
	case defn == nil:
		return nil
	case defn.Scalar != nil:
		return defn.Scalar.Name
	case defn.Object != nil:
		return defn.Object.Name
	case defn.InputObject != nil:
		return defn.InputObject.Name
	default:
		return nil
	}
}

func (defn *TypeDefinition) asDefinition() *Definition {
	return &Definition{Type: defn}
}

// ScalarTypeDefinition names a scalar type.
// https://graphql.github.io/graphql-spec/June2018/#ScalarTypeDefinition
type ScalarTypeDefinition struct {
	Description *Description
	Keyword     Pos
	Name        *Name
}

func (defn *ScalarTypeDefinition) asTypeDefinition() *TypeDefinition {
	return &TypeDefinition{Scalar: defn}
}

// ObjectTypeDefinition names an output object type.
// https://graphql.github.io/graphql-spec/June2018/#ObjectTypeDefinition
type ObjectTypeDefinition struct {
	Description *Description
	Keyword     Pos
	Name        *Name
	Fields      *FieldsDefinition
}

func (defn *ObjectTypeDefinition) asTypeDefinition() *TypeDefinition {
	return &TypeDefinition{Object: defn}
}

// FieldsDefinition is the list of fields in an ObjectTypeDefinition.
// https://graphql.github.io/graphql-spec/June2018/#FieldsDefinition
type FieldsDefinition struct {
	LBrace Pos
	Defs   []*FieldDefinition
	RBrace Pos
}

// FieldDefinition specifies a single field in an ObjectTypeDefinition.
// https://graphql.github.io/graphql-spec/June2018/#FieldsDefinition
type FieldDefinition struct {
	Description *Description
	Name        *Name
	Args        *ArgumentsDefinition
	Colon       Pos
	Type        *TypeRef
}

// ArgumentsDefinition specifies the arguments for a FieldDefinition.
// https://graphql.github.io/graphql-spec/June2018/#ArgumentsDefinition
type ArgumentsDefinition struct {
	LParen Pos
	Args   []*InputValueDefinition
	RParen Pos
}

// InputObjectTypeDefinition names an input object type.
// https://graphql.github.io/graphql-spec/June2018/#InputObjectTypeDefinition
type InputObjectTypeDefinition struct {
	Description *Description
	Keyword     Pos
	Name        *Name
	Fields      *InputFieldsDefinition
}

func (defn *InputObjectTypeDefinition) asTypeDefinition() *TypeDefinition {
	return &TypeDefinition{InputObject: defn}
}

// InputFieldsDefinition is the list of fields in an InputObjectTypeDefinition.
// https://graphql.github.io/graphql-spec/June2018/#InputFieldsDefinition
type InputFieldsDefinition struct {
	LBrace Pos
	Defs   []*InputValueDefinition
	RBrace Pos
}

// InputValueDefinition specifies an argument in a FieldDefinition or a field
// in an InputObjectTypeDefinition.
// https://graphql.github.io/graphql-spec/June2018/#InputValueDefinition
type InputValueDefinition struct {
	Description *Description
	Name        *Name
	Colon       Pos
	Type        *TypeRef
	Default     *DefaultValue
}
