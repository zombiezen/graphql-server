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
	"context"
	"reflect"
	"sync"
)

// Predefined introspection field names.
const (
	typeByNameFieldName = "__type"
	schemaFieldName     = "__schema"
	typeNameFieldName   = "__typename"
)

// schemaType returns the built-in __Schema type.
func schemaType() *gqlType {
	return introspectionSchema().types["__Schema"]
}

// typeType returns the built-in __Type type.
func typeType() *gqlType {
	return introspectionSchema().types["__Type"]
}

func typeNameField() *objectTypeField {
	return &objectTypeField{
		name: typeNameFieldName,
		typ:  stringType.toNonNullable(),
	}
}

func typeByNameField() *objectTypeField {
	return &objectTypeField{
		name: typeByNameFieldName,
		typ:  typeType(),
		args: []inputValueDefinition{
			{
				name:         "name",
				defaultValue: Value{typ: stringType.toNonNullable()},
			},
		},
	}
}

func schemaField() *objectTypeField {
	return &objectTypeField{
		name: schemaFieldName,
		typ:  schemaType().toNonNullable(),
	}
}

// schemaObject is a representation of __Schema.
type schemaObject struct {
	Types            []*gqlType
	QueryType        *gqlType
	MutationType     *gqlType
	SubscriptionType *gqlType
	Directives       *[]interface{}
}

func (schema *Schema) introspectSchema(ctx context.Context, variables map[string]Value, field *SelectedField) (Value, []error) {
	s := &schemaObject{
		QueryType:    schema.query,
		MutationType: schema.mutation,
	}
	for _, name := range schema.typeOrder {
		s.Types = append(s.Types, schema.types[name])
	}
	return schema.valueFromGo(ctx, variables, reflect.ValueOf(s), schemaType().toNonNullable(), field.SelectionSet())
}

// introspectType implements the "__type" field at the root of a query.
func (schema *Schema) introspectType(ctx context.Context, variables map[string]Value, field *SelectedField) (Value, []error) {
	name := field.Arg("name").Scalar()
	typ := schema.types[name]
	if typ == nil {
		// Not found; return null.
		return Value{typ: typeType()}, nil
	}
	return schema.valueFromGo(ctx, variables, reflect.ValueOf(typ), typeType(), field.SelectionSet())
}

var introspect struct {
	sync.Once
	schema *Schema
	err    error
}

func introspectionSchema() *Schema {
	// https://graphql.github.io/graphql-spec/June2018/#sec-Schema-Introspection
	introspect.Once.Do(func() {
		introspect.schema, introspect.err = parseSchema(`
type __Schema {
  types: [__Type!]!
  queryType: __Type!
  mutationType: __Type
  subscriptionType: __Type
  directives: [__Directive!]!
}

type __Type {
  kind: __TypeKind!
  name: String
  description: String

  # OBJECT and INTERFACE only
  fields(includeDeprecated: Boolean = false): [__Field!]

  # OBJECT only
  interfaces: [__Type!]

  # INTERFACE and UNION only
  possibleTypes: [__Type!]

  # ENUM only
  enumValues(includeDeprecated: Boolean = false): [__EnumValue!]

  # INPUT_OBJECT only
  inputFields: [__InputValue!]

  # NON_NULL and LIST only
  ofType: __Type
}

type __Field {
  name: String!
  description: String
  args: [__InputValue!]!
  type: __Type!
  isDeprecated: Boolean!
  deprecationReason: String
}

type __InputValue {
  name: String!
  description: String
  type: __Type!
  defaultValue: String
}

type __EnumValue {
  name: String!
  description: String
  isDeprecated: Boolean!
  deprecationReason: String
}

enum __TypeKind {
  SCALAR
  OBJECT
  INTERFACE
  UNION
  ENUM
  INPUT_OBJECT
  LIST
  NON_NULL
}

type __Directive {
  name: String!
  description: String
  locations: [__DirectiveLocation!]!
  args: [__InputValue!]!
}

enum __DirectiveLocation {
  QUERY
  MUTATION
  SUBSCRIPTION
  FIELD
  FRAGMENT_DEFINITION
  FRAGMENT_SPREAD
  INLINE_FRAGMENT
  SCHEMA
  SCALAR
  OBJECT
  FIELD_DEFINITION
  ARGUMENT_DEFINITION
  INTERFACE
  UNION
  ENUM
  ENUM_VALUE
  INPUT_OBJECT
  INPUT_FIELD_DEFINITION
}
		`, true)
	})
	if introspect.err != nil {
		panic(introspect.err)
	}
	return introspect.schema
}
