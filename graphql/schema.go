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
func ParseSchema(source string) (*Schema, error) {
	return parseSchema(source, false)
}

func parseSchema(source string, internal bool) (*Schema, error) {
	doc, errs := gqlang.Parse(source)
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
	for _, defn := range doc.Definitions {
		if defn.Operation != nil {
			return nil, xerrors.Errorf("parse schema: %v: operations not allowed", defn.Operation.Start.ToPosition(source))
		}
	}
	typeMap, err := buildTypeMap(source, internal, doc)
	if err != nil {
		return nil, xerrors.Errorf("parse schema: %v", err)
	}
	schema := &Schema{
		query:    typeMap["Query"],
		mutation: typeMap["Mutation"],
		types:    typeMap,
	}
	if !internal {
		if schema.query == nil {
			return nil, xerrors.New("parse schema: could not find Query type")
		}
		if !schema.query.isObject() {
			return nil, xerrors.Errorf("parse schema: query type %v must be an object", schema.query)
		}
		if schema.mutation != nil && !schema.mutation.isObject() {
			return nil, xerrors.Errorf("parse schema: mutation type %v must be an object", schema.mutation)
		}
	}
	return schema, nil
}

const reservedPrefix = "__"

func buildTypeMap(source string, internal bool, doc *gqlang.Document) (map[string]*gqlType, error) {
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
		if t == nil {
			continue
		}
		name := t.Name()
		if !internal && strings.HasPrefix(name.Value, reservedPrefix) {
			return nil, xerrors.Errorf("%v: use of reserved name %q", name.Start.ToPosition(source), name.Value)
		}
		if typeMap[name.Value] != nil {
			return nil, xerrors.Errorf("%v: multiple types with name %q", name.Start.ToPosition(source), name.Value)
		}

		switch {
		case t.Scalar != nil:
			typeMap[name.Value] = newScalarType(name.Value, t.Scalar.Description.Value())
		case t.Enum != nil:
			info := &enumType{
				name:    name.Value,
				symbols: make(map[string]struct{}),
			}
			for _, v := range defn.Type.Enum.Values.Values {
				sym := v.Value.Value
				if !internal && strings.HasPrefix(sym, reservedPrefix) {
					return nil, xerrors.Errorf("%v: use of reserved name %q", v.Value.Start.ToPosition(source), sym)
				}
				if info.has(sym) {
					return nil, xerrors.Errorf("%v: multiple enum values with name %q", sym)
				}
				info.symbols[sym] = struct{}{}
			}
			typeMap[name.Value] = newEnumType(info, t.Enum.Description.Value())
		case t.Object != nil:
			typeMap[name.Value] = newObjectType(&objectType{
				name:   name.Value,
				fields: make(map[string]objectTypeField),
			}, t.Object.Description.Value())
		case t.InputObject != nil:
			typeMap[name.Value] = newInputObjectType(&inputObjectType{
				name:   name.Value,
				fields: make(map[string]inputValueDefinition),
			}, t.InputObject.Description.Value())
		}
	}
	// Second pass: fill in object definitions.
	for _, defn := range doc.Definitions {
		if defn.Type == nil {
			continue
		}
		switch {
		case defn.Type.Object != nil:
			if err := fillObjectTypeFields(source, internal, typeMap, defn.Type.Object); err != nil {
				return nil, err
			}
		case defn.Type.InputObject != nil:
			if err := fillInputObjectTypeFields(source, internal, typeMap, defn.Type.InputObject); err != nil {
				return nil, err
			}
		}
	}
	return typeMap, nil
}

func fillObjectTypeFields(source string, internal bool, typeMap map[string]*gqlType, obj *gqlang.ObjectTypeDefinition) error {
	info := typeMap[obj.Name.Value].obj
	for _, fieldDefn := range obj.Fields.Defs {
		fieldName := fieldDefn.Name.Value
		if !internal && strings.HasPrefix(fieldName, reservedPrefix) {
			return xerrors.Errorf("%v: use of reserved name %q", fieldDefn.Name.Start.ToPosition(source), fieldName)
		}
		if _, found := info.fields[fieldName]; found {
			return xerrors.Errorf("%v: multiple fields named %q in %s", fieldDefn.Name.Start.ToPosition(source), fieldName, obj.Name)
		}
		typ := resolveTypeRef(typeMap, fieldDefn.Type)
		if typ == nil {
			return xerrors.Errorf("%v: undefined type %v", fieldDefn.Type.Start().ToPosition(source), fieldDefn.Type)
		}
		if !typ.isOutputType() {
			return xerrors.Errorf("%v: %v is not an output type", fieldDefn.Type.Start().ToPosition(source), fieldDefn.Type)
		}
		f := objectTypeField{
			name:        fieldName,
			description: fieldDefn.Description.Value(),
			typ:         typ,
		}
		if fieldDefn.Args != nil {
			f.args = make(map[string]inputValueDefinition)
			for _, arg := range fieldDefn.Args.Args {
				argName := arg.Name.Value
				if !internal && strings.HasPrefix(argName, reservedPrefix) {
					return xerrors.Errorf("%v: use of reserved name %q", arg.Name.Start.ToPosition(source), argName)
				}
				if _, found := f.args[argName]; found {
					return xerrors.Errorf("%v: multiple arguments named %q for field %s.%s", arg.Name.Start.ToPosition(source), argName, obj.Name, fieldName)
				}
				typ := resolveTypeRef(typeMap, arg.Type)
				if typ == nil {
					return xerrors.Errorf("%v: undefined type %v", arg.Type.Start().ToPosition(source), arg.Type)
				}
				if !typ.isInputType() {
					return xerrors.Errorf("%v: %v is not an input type", arg.Type.Start().ToPosition(source), arg.Type)
				}
				defaultValue := Value{typ: typ}
				if arg.Default != nil {
					if errs := validateConstantValue(source, typ, arg.Default.Value); len(errs) > 0 {
						return errs[0]
					}
					defaultValue = coerceConstantInputValue(typ, arg.Default.Value)
				}
				f.args[argName] = inputValueDefinition{defaultValue: defaultValue}
			}
		}
		info.fields[fieldName] = f
		info.fieldOrder = append(info.fieldOrder, fieldName)
	}
	return nil
}

func fillInputObjectTypeFields(source string, internal bool, typeMap map[string]*gqlType, obj *gqlang.InputObjectTypeDefinition) error {
	info := typeMap[obj.Name.Value].input
	for _, fieldDefn := range obj.Fields.Defs {
		fieldName := fieldDefn.Name.Value
		if !internal && strings.HasPrefix(fieldName, reservedPrefix) {
			return xerrors.Errorf("%v: use of reserved name %q", fieldDefn.Name.Start.ToPosition(source), fieldName)
		}
		if _, found := info.fields[fieldName]; found {
			return xerrors.Errorf("%v: multiple fields named %q in %s", fieldDefn.Name.Start.ToPosition(source), fieldName, obj.Name)
		}
		typ := resolveTypeRef(typeMap, fieldDefn.Type)
		if typ == nil {
			return xerrors.Errorf("%v: undefined type %v", fieldDefn.Type.Start().ToPosition(source), fieldDefn.Type)
		}
		if !typ.isInputType() {
			return xerrors.Errorf("%v: %v is not an input type", fieldDefn.Type.Start().ToPosition(source), fieldDefn.Type)
		}
		var f inputValueDefinition
		if fieldDefn.Default != nil {
			f.defaultValue = coerceConstantInputValue(typ, fieldDefn.Default.Value)
		} else {
			f.defaultValue.typ = typ
		}
		info.fields[fieldDefn.Name.Value] = f
	}
	return nil
}

func resolveTypeRef(typeMap map[string]*gqlType, ref *gqlang.TypeRef) *gqlType {
	switch {
	case ref.Named != nil:
		return typeMap[ref.Named.Value]
	case ref.List != nil:
		elem := resolveTypeRef(typeMap, ref.List.Type)
		if elem == nil {
			return nil
		}
		return listOf(elem)
	case ref.NonNull != nil && ref.NonNull.Named != nil:
		base := typeMap[ref.NonNull.Named.Value]
		if base == nil {
			return nil
		}
		return base.toNonNullable()
	case ref.NonNull != nil && ref.NonNull.List != nil:
		elem := resolveTypeRef(typeMap, ref.NonNull.List.Type)
		if elem == nil {
			return nil
		}
		return listOf(elem).toNonNullable()
	default:
		panic("unrecognized type reference form")
	}
}

func (schema *Schema) operationType(opType gqlang.OperationType) *gqlType {
	switch opType {
	case gqlang.Query:
		return schema.query
	case gqlang.Mutation:
		return schema.mutation
	case gqlang.Subscription:
		return nil
	default:
		panic("unknown operation type")
	}
}
