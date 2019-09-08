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
// https://graphql.github.io/graphql-spec/June2018/#sec-Selection-Sets
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
	Var   *Variable
	Colon Pos
	Type  *TypeRef
}

// A Name is an identifier.
// https://graphql.github.io/graphql-spec/June2018/#sec-Names
type Name struct {
	Value string
	Start Pos
}

// String returns the name.
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
// https://graphql.github.io/graphql-spec/June2018/#Type
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
