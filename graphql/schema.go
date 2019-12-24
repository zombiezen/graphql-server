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
	"io"
	"os"
	"reflect"
	"strings"
	"sync"

	"golang.org/x/xerrors"
	"zombiezen.com/go/graphql-server/internal/gqlang"
)

// Schema is a parsed set of type definitions.
type Schema struct {
	query     *gqlType
	mutation  *gqlType
	types     map[string]*gqlType
	typeOrder []string

	mu      sync.RWMutex
	goTypes map[typeKey]*typeDescriptor
}

// SchemaOptions specifies how the schema source will be interpreted. nil is
// treated the same as the zero value.
type SchemaOptions struct {
	// IgnoreDescriptions will strip descriptions from the schema as it is parsed.
	IgnoreDescriptions bool
}

type schemaOptions struct {
	*SchemaOptions
	internal bool
}

func (opts schemaOptions) description(d *gqlang.Description) string {
	if opts.SchemaOptions != nil && opts.IgnoreDescriptions {
		return ""
	}
	return d.Value()
}

// ParseSchema parses a GraphQL document containing type definitions.
// It is assumed that the schema is trusted.
func ParseSchema(source string, opts *SchemaOptions) (*Schema, error) {
	schema, err := parseSchema(source, schemaOptions{SchemaOptions: opts})
	if err != nil {
		return nil, xerrors.Errorf("parse schema: %w", err)
	}
	return schema, nil
}

func parseSchema(source string, opts schemaOptions) (*Schema, error) {
	doc, errs := gqlang.Parse(source)
	if len(errs) > 0 {
		msgBuilder := new(strings.Builder)
		msgBuilder.WriteString("syntax errors:")
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
	var typeOrder []string
	for _, defn := range doc.Definitions {
		if defn.Operation != nil {
			return nil, xerrors.Errorf("%v: operations not allowed", defn.Operation.Start.ToPosition(source))
		}
		if defn.Type != nil {
			typeOrder = append(typeOrder, defn.Type.Name().String())
		}
	}
	typeMap, err := buildTypeMap(source, opts, doc)
	if err != nil {
		return nil, err
	}
	schema := &Schema{
		query:     typeMap["Query"],
		mutation:  typeMap["Mutation"],
		types:     typeMap,
		typeOrder: typeOrder,
		goTypes:   make(map[typeKey]*typeDescriptor),
	}
	if !opts.internal {
		if schema.query == nil {
			return nil, xerrors.New("could not find Query type")
		}
		if !schema.query.isObject() {
			return nil, xerrors.Errorf("query type %v must be an object", schema.query)
		}
		if schema.mutation != nil && !schema.mutation.isObject() {
			return nil, xerrors.Errorf("mutation type %v must be an object", schema.mutation)
		}
	}
	return schema, nil
}

// ParseSchemaFile parses the GraphQL file containing type definitions named
// by path. It is assumed that the schema is trusted.
func ParseSchemaFile(path string, opts *SchemaOptions) (*Schema, error) {
	f, err := os.Open(path)
	if err != nil {
		// The error will contain the path.
		return nil, xerrors.Errorf("parse schema file: %w", err)
	}
	source := new(strings.Builder)
	_, err = io.Copy(source, f)
	f.Close()
	if err != nil {
		return nil, xerrors.Errorf("parse schema file %s: %w", path, err)
	}
	schema, err := parseSchema(source.String(), schemaOptions{SchemaOptions: opts})
	if err != nil {
		return nil, xerrors.Errorf("parse schema file %s: %w", path, err)
	}
	return schema, nil
}

const reservedPrefix = "__"

func builtins(includeIntrospection bool) []*gqlType {
	b := []*gqlType{
		booleanType,
		floatType,
		intType,
		stringType,
		idType,
	}
	if !includeIntrospection {
		return b
	}
	i := introspectionSchema()
	return append(b,
		i.types["__Schema"],
		i.types["__Type"],
		i.types["__Field"],
		i.types["__InputValue"],
		i.types["__EnumValue"],
		i.types["__TypeKind"],
		i.types["__Directive"],
		i.types["__DirectiveLocation"],
	)
}

func buildTypeMap(source string, opts schemaOptions, doc *gqlang.Document) (map[string]*gqlType, error) {
	typeMap := make(map[string]*gqlType)
	for _, b := range builtins(!opts.internal) {
		typeMap[b.String()] = b
	}
	// First pass: fill out lookup table.
	for _, defn := range doc.Definitions {
		t := defn.Type
		if t == nil {
			continue
		}
		name := t.Name()
		if !opts.internal && strings.HasPrefix(name.Value, reservedPrefix) {
			return nil, xerrors.Errorf("%v: use of reserved name %q", name.Start.ToPosition(source), name.Value)
		}
		if typeMap[name.Value] != nil {
			return nil, xerrors.Errorf("%v: multiple types with name %q", name.Start.ToPosition(source), name.Value)
		}

		switch {
		case t.Scalar != nil:
			typeMap[name.Value] = newScalarType(name.Value, opts.description(t.Scalar.Description))
		case t.Enum != nil:
			info := &enumType{
				name: name.Value,
			}
			for _, v := range defn.Type.Enum.Values.Values {
				sym := v.Value.Value
				if !opts.internal && strings.HasPrefix(sym, reservedPrefix) {
					return nil, xerrors.Errorf("%v: use of reserved name %q", v.Value.Start.ToPosition(source), sym)
				}
				if info.has(sym) {
					return nil, xerrors.Errorf("%v: multiple enum values with name %q", v.Value.Start.ToPosition(source), sym)
				}
				ev := enumValue{
					name:        sym,
					description: opts.description(v.Description),
				}
				var err error
				ev.deprecated, ev.deprecationReason, err = processTypeDirectives(source, typeMap, v.Directives)
				if err != nil {
					return nil, err
				}
				info.values = append(info.values, ev)
			}
			typeMap[name.Value] = newEnumType(info, opts.description(t.Enum.Description))
		case t.Object != nil:
			typeMap[name.Value] = newObjectType(&objectType{
				name: name.Value,
			}, opts.description(t.Object.Description))
		case t.InputObject != nil:
			typeMap[name.Value] = newInputObjectType(&inputObjectType{
				name: name.Value,
			}, opts.description(t.InputObject.Description))
		}
	}
	// Second pass: fill in object definitions.
	for _, defn := range doc.Definitions {
		if defn.Type == nil {
			continue
		}
		switch {
		case defn.Type.Object != nil:
			if err := fillObjectTypeFields(source, opts, typeMap, defn.Type.Object); err != nil {
				return nil, err
			}
		case defn.Type.InputObject != nil:
			if err := fillInputObjectTypeFields(source, opts, typeMap, defn.Type.InputObject); err != nil {
				return nil, err
			}
		}
	}
	return typeMap, nil
}

func fillObjectTypeFields(source string, opts schemaOptions, typeMap map[string]*gqlType, obj *gqlang.ObjectTypeDefinition) error {
	info := typeMap[obj.Name.Value].obj
	for _, fieldDefn := range obj.Fields.Defs {
		fieldName := fieldDefn.Name.Value
		if !opts.internal && strings.HasPrefix(fieldName, reservedPrefix) {
			return xerrors.Errorf("%v: use of reserved name %q", fieldDefn.Name.Start.ToPosition(source), fieldName)
		}
		if info.field(fieldName) != nil {
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
			description: opts.description(fieldDefn.Description),
			typ:         typ,
		}
		if fieldDefn.Args != nil {
			for _, arg := range fieldDefn.Args.Args {
				argName := arg.Name.Value
				if !opts.internal && strings.HasPrefix(argName, reservedPrefix) {
					return xerrors.Errorf("%v: use of reserved name %q", arg.Name.Start.ToPosition(source), argName)
				}
				if f.args.byName(argName) != nil {
					return xerrors.Errorf("%v: multiple arguments named %q for field %s.%s", arg.Name.Start.ToPosition(source), argName, obj.Name, fieldName)
				}
				typ := resolveTypeRef(typeMap, arg.Type)
				if typ == nil {
					return xerrors.Errorf("%v: undefined type %v", arg.Type.Start().ToPosition(source), arg.Type)
				}
				if !typ.isInputType() {
					return xerrors.Errorf("%v: %v is not an input type", arg.Type.Start().ToPosition(source), arg.Type)
				}
				argDef := inputValueDefinition{
					name:         argName,
					description:  opts.description(arg.Description),
					defaultValue: Value{typ: typ},
				}
				if arg.Default != nil {
					if errs := validateConstantValue(source, typ, arg.Default.Value); len(errs) > 0 {
						return errs[0]
					}
					argDef.defaultValue = coerceConstantInputValue(typ, arg.Default.Value)
				}
				f.args = append(f.args, argDef)
			}
		}
		var err error
		f.deprecated, f.deprecationReason, err = processTypeDirectives(source, typeMap, fieldDefn.Directives)
		if err != nil {
			return err
		}
		info.fields = append(info.fields, f)
	}
	return nil
}

func fillInputObjectTypeFields(source string, opts schemaOptions, typeMap map[string]*gqlType, obj *gqlang.InputObjectTypeDefinition) error {
	info := typeMap[obj.Name.Value].input
	for _, fieldDefn := range obj.Fields.Defs {
		fieldName := fieldDefn.Name.Value
		if !opts.internal && strings.HasPrefix(fieldName, reservedPrefix) {
			return xerrors.Errorf("%v: use of reserved name %q", fieldDefn.Name.Start.ToPosition(source), fieldName)
		}
		if info.fields.byName(fieldName) != nil {
			return xerrors.Errorf("%v: multiple fields named %q in %s", fieldDefn.Name.Start.ToPosition(source), fieldName, obj.Name)
		}
		typ := resolveTypeRef(typeMap, fieldDefn.Type)
		if typ == nil {
			return xerrors.Errorf("%v: undefined type %v", fieldDefn.Type.Start().ToPosition(source), fieldDefn.Type)
		}
		if !typ.isInputType() {
			return xerrors.Errorf("%v: %v is not an input type", fieldDefn.Type.Start().ToPosition(source), fieldDefn.Type)
		}
		f := inputValueDefinition{
			name:         fieldName,
			description:  opts.description(fieldDefn.Description),
			defaultValue: Value{typ: typ},
		}
		if fieldDefn.Default != nil {
			f.defaultValue = coerceConstantInputValue(typ, fieldDefn.Default.Value)
		}
		info.fields = append(info.fields, f)
	}
	return nil
}

func processTypeDirectives(source string, typeMap map[string]*gqlType, directives gqlang.Directives) (deprecated bool, deprecationReason NullString, _ error) {
	v := &validationScope{
		source: source,
		types:  typeMap,
	}
	s := &selectionSetScope{
		source: source,
		types:  typeMap,
	}
	for _, d := range directives {
		if d.Name.Value != deprecatedDirective.Name {
			return false, NullString{}, xerrors.Errorf("%v: unknown directive @%s", d.At.ToPosition(source), d.Name.Value)
		}
		if deprecated {
			return false, NullString{}, xerrors.Errorf("%v: multiple @%s directives", d.At.ToPosition(source), d.Name.Value)
		}

		deprecated = true
		argErrs := validateArguments(v, deprecatedDirective.Args, d.Arguments)
		if len(argErrs) > 0 {
			return false, NullString{}, xerrors.Errorf("%v: @%s directive: %w", d.Arguments.LParen.ToPosition(source), d.Name.Value, argErrs[0])
		}
		var args map[string]Value
		args, argErrs = coerceArgumentValues(s, deprecatedDirective.Args, d.Arguments)
		if len(argErrs) > 0 {
			return false, NullString{}, xerrors.Errorf("%v: @%s directive: %w", d.Arguments.LParen.ToPosition(source), d.Name.Value, argErrs[0])
		}
		if r := args["reason"]; !r.IsNull() {
			deprecationReason = NullString{S: r.Scalar(), Valid: true}
		}
	}
	return deprecated, deprecationReason, nil
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

// Validate parses and type-checks an executable GraphQL document.
func (schema *Schema) Validate(query string) (*ValidatedQuery, []*ResponseError) {
	doc, errs := gqlang.Parse(query)
	if len(errs) > 0 {
		respErrs := make([]*ResponseError, 0, len(errs))
		for _, err := range errs {
			respErrs = append(respErrs, toResponseError(err))
		}
		return nil, respErrs
	}
	errs = schema.validateRequest(query, doc)
	if len(errs) > 0 {
		respErrs := make([]*ResponseError, 0, len(errs))
		for _, err := range errs {
			respErrs = append(respErrs, toResponseError(err))
		}
		return nil, respErrs
	}
	return &ValidatedQuery{
		schema: schema,
		source: query,
		doc:    doc,
	}, nil
}

func (schema *Schema) typeDescriptor(key typeKey) *typeDescriptor {
	if key.goType.Kind() == reflect.Interface {
		return nil
	}

	// Fast path: descriptor already computed.
	schema.mu.RLock()
	desc := schema.goTypes[key]
	schema.mu.RUnlock()
	if desc != nil {
		return desc
	}

	// Compute type descriptor.
	schema.mu.Lock()
	desc = schema.typeDescriptorLocked(key)
	schema.mu.Unlock()
	return desc
}

func (schema *Schema) typeDescriptorLocked(key typeKey) *typeDescriptor {
	if key.goType.Kind() == reflect.Interface {
		return nil
	}

	desc := schema.goTypes[key]
	if desc != nil {
		return desc
	}
	if key.goType.AssignableTo(fieldResolverGoType) {
		desc = &typeDescriptor{
			hasResolveField: true,
		}
		schema.goTypes[key] = desc
		return desc
	}
	desc = &typeDescriptor{
		fields: make(map[string]fieldDescriptor),
	}
	schema.goTypes[key] = desc

	var structType reflect.Type
	switch kind := key.goType.Kind(); {
	case kind == reflect.Struct:
		structType = key.goType
	case kind == reflect.Ptr && key.goType.Elem().Kind() == reflect.Struct:
		structType = key.goType.Elem()
	}
	for _, field := range key.gqlType.fields {
		numMatches := 0
		fdesc := fieldDescriptor{
			fieldIndex:  -1,
			methodIndex: -1,
		}
		lowerFieldName := toLower(field.name)
		passSel := field.typ.selectionSetType() != nil
		var fieldGoType reflect.Type
		for i, n := 0, key.goType.NumMethod(); i < n; i++ {
			meth := key.goType.Method(i)
			if meth.PkgPath != "" {
				// Don't consider unexported methods.
				continue
			}
			if toLower(meth.Name) == lowerFieldName {
				numMatches++
				fdesc.methodIndex = i
				if err := validateFieldMethodSignature(meth.Type, passSel); err != nil {
					*desc = typeDescriptor{
						err: xerrors.Errorf("can't use method %v.%s for field %s.%s: %w",
							key.goType, meth.Name, key.gqlType.name, field.name, err),
					}
					return desc
				}
				fieldGoType = meth.Type.Out(0)
			}
		}
		if structType != nil && len(field.args) == 0 {
			for i, n := 0, structType.NumField(); i < n; i++ {
				goField := structType.Field(i)
				if goField.PkgPath != "" {
					// Don't consider unexported fields.
					continue
				}
				if toLower(goField.Name) == lowerFieldName {
					numMatches++
					fdesc.fieldIndex = i
					fieldGoType = goField.Type
					if key.goType.Kind() == reflect.Ptr {
						// If the field is addressable, then use that as part of the type.
						// For example, a Bar field on a *Foo may be used as a *Bar if
						// needed. The pointer may get stripped below as part of
						// innermostPointerType.
						fieldGoType = reflect.PtrTo(fieldGoType)
					}
				}
			}
		}
		if numMatches == 0 {
			*desc = typeDescriptor{
				err: xerrors.Errorf("no method or field found on %v for %s.%s",
					key.goType, key.gqlType.name, field.name),
			}
			return desc
		}
		if numMatches > 1 {
			*desc = typeDescriptor{
				err: xerrors.Errorf("multiple methods and/or fields found on %v for %s.%s",
					key.goType, key.gqlType.name, field.name),
			}
			return desc
		}
		// TODO(someday): Check field type for scalars.
		if field.typ.isObject() {
			err := schema.typeDescriptorLocked(typeKey{
				goType:  innermostPointerType(fieldGoType),
				gqlType: field.typ.obj,
			}).err
			if err != nil {
				*desc = typeDescriptor{
					err: xerrors.Errorf("field %s: %w", field.name, err),
				}
				return desc
			}
		}
		desc.fields[field.name] = fdesc
	}
	return desc
}

// innermostPointerType returns the value's innermost pointer or interface type.
func innermostPointerType(t reflect.Type) reflect.Type {
	var tprev reflect.Type
	for t.Kind() == reflect.Ptr {
		tprev, t = t, t.Elem()
	}
	if tprev == nil || t.Kind() == reflect.Interface {
		return t
	}
	return tprev
}

func validateFieldMethodSignature(mtype reflect.Type, passSel bool) error {
	numIn := mtype.NumIn()
	argIdx := 1 // skip past receiver
	if argIdx < numIn && mtype.In(argIdx) == contextGoType {
		argIdx++
	}
	if argIdx < numIn && mtype.In(argIdx) == valueMapGoType {
		argIdx++
	}
	if passSel {
		if argIdx < numIn && mtype.In(argIdx) == selectionSetGoType {
			argIdx++
		}
	}
	if argIdx != numIn {
		return xerrors.New("wrong parameter signature")
	}
	switch mtype.NumOut() {
	case 1:
		if mtype.Out(0) == errorGoType {
			return xerrors.New("return type must not be error")
		}
	case 2:
		if mtype.Out(0) == errorGoType {
			return xerrors.New("first return type must not be error")
		}
		if got := mtype.Out(1); got != errorGoType {
			return xerrors.Errorf("second return type must be error (found %v)", got)
		}
	default:
		return xerrors.New("wrong return signature")
	}
	return nil
}

// typeKey is the key to the schema's Go type cache.
type typeKey struct {
	goType  reflect.Type
	gqlType *objectType
}

type typeDescriptor struct {
	fields          map[string]fieldDescriptor
	hasResolveField bool
	err             error
}

func (desc *typeDescriptor) read(ctx context.Context, recv reflect.Value, req FieldRequest) (reflect.Value, error) {
	if desc.err != nil {
		return reflect.Value{}, desc.err
	}
	if desc.hasResolveField {
		val, err := interfaceValueForAssertions(recv).(FieldResolver).ResolveField(ctx, req)
		if err != nil {
			// Intentionally making the returned error opaque to avoid interference in
			// toResponseError.
			return reflect.Value{}, xerrors.Errorf("server error: %v", err)
		}
		return reflect.ValueOf(val), nil
	}
	fdesc, ok := desc.fields[req.Name]
	if !ok {
		return reflect.Value{}, xerrors.Errorf("internal server error: no field descriptor for %q", req.Name)
	}
	return fdesc.read(ctx, recv, req)
}

type fieldDescriptor struct {
	fieldIndex  int
	methodIndex int
}

func (fdesc fieldDescriptor) read(ctx context.Context, recv reflect.Value, req FieldRequest) (reflect.Value, error) {
	if fdesc.fieldIndex != -1 {
		recv = unwrapPointer(recv)
		if !recv.IsValid() {
			return reflect.Value{}, xerrors.New("nil pointer")
		}
		return recv.Field(fdesc.fieldIndex), nil
	}
	method := recv.Method(fdesc.methodIndex)
	mtype := method.Type()
	numIn := mtype.NumIn()
	var callArgs []reflect.Value
	if len(callArgs) < numIn && mtype.In(len(callArgs)) == contextGoType {
		callArgs = append(callArgs, reflect.ValueOf(ctx))
	}
	if len(callArgs) < numIn && mtype.In(len(callArgs)) == valueMapGoType {
		callArgs = append(callArgs, reflect.ValueOf(req.Args))
	}
	if len(callArgs) < numIn && mtype.In(len(callArgs)) == selectionSetGoType {
		callArgs = append(callArgs, reflect.ValueOf(req.Selection))
	}
	if len(callArgs) != numIn {
		panic("unexpected method parameters; bug in validateFieldMethodSignature?")
	}
	switch mtype.NumOut() {
	case 1:
		out := method.Call(callArgs)
		return out[0], nil
	case 2:
		out := method.Call(callArgs)
		if !out[1].IsNil() {
			// Intentionally making the returned error opaque to avoid interference in
			// toResponseError.
			err := out[1].Interface().(error)
			return reflect.Value{}, xerrors.Errorf("server error: %v", err)
		}
		return out[0], nil
	default:
		panic("unexpected method return signature; bug in validateFieldMethodSignature?")
	}
}

var (
	contextGoType       = reflect.TypeOf(new(context.Context)).Elem()
	fieldResolverGoType = reflect.TypeOf(new(FieldResolver)).Elem()
	valueMapGoType      = reflect.TypeOf(new(map[string]Value)).Elem()
	selectionSetGoType  = reflect.TypeOf(new(*SelectionSet)).Elem()
	errorGoType         = reflect.TypeOf(new(error)).Elem()
)

func toLower(s string) string {
	sb := new(strings.Builder)
	for i := 0; i < len(s); i++ {
		if 'A' <= s[i] && s[i] <= 'Z' {
			sb.WriteByte(s[i] - 'A' + 'a')
		} else {
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}
