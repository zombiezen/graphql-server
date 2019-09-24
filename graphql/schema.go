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

// Schema is a parsed set of type definitions.
type Schema struct {
	query    *gqlType
	mutation *gqlType
	types    map[string]*gqlType
}

// ParseSchema parses a GraphQL document containing type definitions.
func ParseSchema(input string) (*Schema, error) {
	doc, errs := gqlang.Parse(input)
	if len(errs) > 0 {
		msgBuilder := new(strings.Builder)
		msgBuilder.WriteString("parse schema:")
		for _, err := range errs {
			msgBuilder.WriteByte('\n')
			if p, ok := gqlang.ErrorPosition(err); ok {
				msgBuilder.WriteString(p.String())
				msgBuilder.WriteString(": ")
			}
			msgBuilder.WriteString(err.Error())
		}
		return nil, xerrors.New(msgBuilder.String())
	}
	typeMap := make(map[string]*gqlType)
	builtins := []*gqlType{
		booleanType,
		floatType,
		intType,
		stringType,
		idType,
	}
	for _, b := range builtins {
		typeMap[b.String()] = b
	}
	// First pass: fill out lookup table.
	for _, defn := range doc.Definitions {
		t := defn.Type
		switch {
		case t == nil:
			continue
		case t.Scalar != nil:
			typeMap[t.Scalar.Name.Value] = newScalarType(t.Scalar.Name.Value)
		case t.Object != nil:
			typeMap[t.Object.Name.Value] = newObjectType(&objectType{
				name:   t.Object.Name.Value,
				fields: make(map[string]*gqlType),
			})
		}
	}
	// Second pass: fill in object definitions.
	for _, defn := range doc.Definitions {
		if defn.Type == nil || defn.Type.Object == nil {
			continue
		}
		obj := defn.Type.Object
		info := typeMap[obj.Name.Value].obj
		for _, fieldDefn := range obj.Fields.Defs {
			info.fields[fieldDefn.Name.Value] = resolveTypeRef(typeMap, fieldDefn.Type)
		}
	}
	schema := &Schema{
		query:    typeMap["Query"],
		mutation: typeMap["Mutation"],
		types:    typeMap,
	}
	if schema.query == nil {
		return nil, xerrors.New("parse schema: no query type specified")
	}
	return schema, nil
}

func resolveTypeRef(typeMap map[string]*gqlType, ref *gqlang.TypeRef) *gqlType {
	switch {
	case ref.Named != nil:
		return typeMap[ref.Named.Value]
	case ref.List != nil:
		return listOf(resolveTypeRef(typeMap, ref.List.Type))
	case ref.NonNull != nil && ref.NonNull.Named != nil:
		return typeMap[ref.NonNull.Named.Value].toNonNullable()
	case ref.NonNull != nil && ref.NonNull.List != nil:
		return listOf(resolveTypeRef(typeMap, ref.NonNull.List.Type)).toNonNullable()
	default:
		panic("unrecognized type reference form")
	}
}
