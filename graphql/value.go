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
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"golang.org/x/xerrors"
	"zombiezen.com/go/graphql-server/internal/gqlang"
)

// A Value is a GraphQL value. The zero value is an untyped null.
//
// For more information on GraphQL types, see https://graphql.org/learn/schema/#type-system
type Value struct {
	typ *gqlType
	val interface{} // one of nil, string, []Value, []Field, or map[string]Value.
}

// Field is a field in an object or input object.
type Field struct {
	// Key is the response object key. This may not be the same as the field name
	// when aliases are used.
	Key string
	// Value is the field's value.
	Value Value
}

// valueFromGo converts a Go value into a GraphQL value. The selection set is
// ignored for scalars.
func (schema *Schema) valueFromGo(ctx context.Context, variables map[string]Value, goValue reflect.Value, typ *gqlType, sel *SelectionSet) (Value, []error) {
	// Since this function is recursive, caller must prepend error operation.

	goValue = unwrapPointer(goValue)
	if !goValue.IsValid() || isGraphQLNull(interfaceValueForAssertions(goValue)) {
		if !typ.isNullable() {
			return Value{typ: typ}, []error{xerrors.Errorf("cannot convert nil to %v", typ)}
		}
		return Value{typ: typ, val: nil}, nil
	}
	switch {
	case typ.isScalar() || typ.isEnum():
		v, err := scalarFromGo(goValue, typ)
		if err != nil {
			return Value{typ: typ}, []error{err}
		}
		return v, nil
	case typ.isList():
		if kind := goValue.Kind(); kind != reflect.Slice && kind != reflect.Array {
			return Value{typ: typ}, []error{xerrors.Errorf("cannot convert %v to %v", goValue.Type(), typ)}
		}
		gqlValues := make([]Value, goValue.Len())
		var errs []error
		for i := range gqlValues {
			var ierrs []error
			gqlValues[i], ierrs = schema.valueFromGo(ctx, variables, goValue.Index(i), typ.listElem, sel)
			for _, err := range ierrs {
				errs = append(errs, &listElementError{idx: i, err: err})
			}
			if len(errs) > 0 && !typ.listElem.isNullable() {
				return Value{typ: typ}, errs
			}
		}
		return Value{typ: typ, val: gqlValues}, errs
	case typ.isObject():
		if sel == nil {
			return Value{typ: typ, val: []Field(nil)}, nil
		}
		gqlFields := make([]Field, 0, len(sel.fields))
		goValue = valueForAssertions(goValue)
		desc := schema.typeDescriptor(typeKey{
			goType:  goValue.Type(),
			gqlType: typ.obj,
		})
		if desc.err != nil {
			return Value{typ: typ}, []error{desc.err}
		}
		var errs []error
		for _, f := range sel.fields {
			var fval Value
			var ferrs []error
			// Validation determines whether this is a valid reference to the
			// reserved fields.
			switch f.name {
			case typeNameFieldName:
				// TODO(someday): Check dynamic type of object.
				fval = Value{
					typ: stringType.toNonNullable(),
					val: typ.toNullable().String(),
				}
			case schemaFieldName:
				fval, ferrs = schema.introspectSchema(ctx, variables, f)
			case typeByNameFieldName:
				fval, ferrs = schema.introspectType(ctx, variables, f)
			default:
				fval, ferrs = schema.readField(ctx, variables, goValue, desc.fields[f.name], typ.obj.field(f.name).typ, f)
			}
			gqlFields = append(gqlFields, Field{Key: f.key, Value: fval})
			errs = append(errs, ferrs...)
		}
		return Value{typ: typ, val: gqlFields}, errs
	default:
		return Value{typ: typ}, []error{xerrors.Errorf("unhandled type: %v", typ)}
	}
}

// coerceArgumentValues uses the algorithm in
// https://graphql.github.io/graphql-spec/June2018/#sec-Coercing-Field-Arguments
// but assumes the arguments were validated.
func coerceArgumentValues(s *selectionSetScope, argDefns inputValueDefinitionList, args *gqlang.Arguments) (map[string]Value, []error) {
	argValues := make(map[string]Value)
	var errs []error
	for _, defn := range argDefns {
		arg := args.ByName(defn.name)
		if arg == nil {
			argValues[defn.name] = defn.defaultValue
			continue
		}
		if arg.Value.VariableRef != nil {
			if _, hasValue := s.variables[arg.Value.VariableRef.Name.Value]; !hasValue {
				argValues[defn.name] = defn.defaultValue
				continue
			}
		}
		var argErrs []error
		argValues[defn.name], argErrs = coerceInputValue(s, defn.Type(), arg.Value)
		for _, err := range argErrs {
			errs = append(errs, xerrors.Errorf("argument %s: %w", defn.name, err))
		}
	}
	return argValues, errs
}

// coerceConstantInputValue converts a input literal without variables into a
// value. It assumes that the input literal has been validated.
func coerceConstantInputValue(typ *gqlType, inputValue *gqlang.InputValue) Value {
	v, errs := coerceInputValue(nil, typ, inputValue)
	if len(errs) > 0 {
		// This condition should be impossible.
		panic(errs[0])
	}
	return v
}

// coerceInputValue converts an input expression possibly containing variables
// into a value. It assumes that the input expression has been validated.
func coerceInputValue(s *selectionSetScope, typ *gqlType, inputValue *gqlang.InputValue) (Value, []error) {
	switch {
	case inputValue.Null != nil:
		return Value{typ: typ}, nil
	case inputValue.VariableRef != nil:
		name := inputValue.VariableRef.Name.Value
		v := s.variables[name]
		if v.IsNull() && !typ.isNullable() {
			return Value{typ: typ}, []error{&ResponseError{
				Message: fmt.Sprintf("cannot use null variable $%s as %v", name, typ),
				Locations: []Location{
					astPositionToLocation(inputValue.VariableRef.Dollar.ToPosition(s.source)),
				},
			}}
		}
		return Value{
			typ: typ,
			val: v.val,
		}, nil
	case typ.isScalar() || typ.isEnum():
		return Value{
			typ: typ,
			val: inputValue.Scalar.Value(),
		}, nil
	case typ.isList():
		if inputValue.List == nil {
			// Attempt to coerce as single-element list.
			// Yes, I'm just as surprised as you are at this behavior,
			// see https://graphql.github.io/graphql-spec/June2018/#sec-Type-System.List
			value, errs := coerceInputValue(s, typ.listElem, inputValue)
			if len(errs) > 0 {
				return Value{typ: typ}, errs
			}
			return Value{
				typ: typ,
				val: []Value{value},
			}, nil
		}
		val := make([]Value, 0, len(inputValue.List.Values))
		var errs []error
		for i, elem := range inputValue.List.Values {
			elemValue, elemErrs := coerceInputValue(s, typ.listElem, elem)
			val = append(val, elemValue)
			for _, err := range elemErrs {
				errs = append(errs, xerrors.Errorf("list[%d]: %w", i, err))
			}
		}
		return Value{typ: typ, val: val}, errs
	case typ.isInputObject():
		val := make(map[string]Value)
		var errs []error
		for _, field := range inputValue.InputObject.Fields {
			fieldName := field.Name.Value
			fieldType := typ.input.fields.byName(fieldName).Type()
			var fieldErrs []error
			val[fieldName], fieldErrs = coerceInputValue(s, fieldType, field.Value)
			for _, err := range fieldErrs {
				errs = append(errs, xerrors.Errorf("input field %s: %w", fieldName, err))
			}
		}
		return Value{typ: typ, val: val}, errs
	default:
		panic("unhandled input type")
	}
}

func (schema *Schema) readField(ctx context.Context, variables map[string]Value, goValue reflect.Value, fdesc fieldDescriptor, typ *gqlType, f *SelectedField) (Value, []error) {
	result, err := fdesc.read(ctx, valueForAssertions(goValue), f.args, f.sub)
	if err != nil {
		return Value{typ: typ}, []error{wrapFieldError(f.key, f.loc, err)}
	}
	v, errs := schema.valueFromGo(ctx, variables, result, typ, f.sub)
	if len(errs) > 0 {
		for i := range errs {
			errs[i] = wrapFieldError(f.key, f.loc, errs[i])
		}
		return v, errs
	}
	return v, nil
}

var (
	contextGoType      = reflect.TypeOf(new(context.Context)).Elem()
	argsGoType         = reflect.TypeOf(new(map[string]Value)).Elem()
	selectionSetGoType = reflect.TypeOf(new(*SelectionSet)).Elem()
	errorGoType        = reflect.TypeOf(new(error)).Elem()
)

func scalarFromGo(goValue reflect.Value, typ *gqlType) (Value, error) {
	goValue = unwrapPointer(goValue)
	goIface := interfaceValueForAssertions(goValue)
	if !goValue.IsValid() || isGraphQLNull(goIface) {
		if !typ.isNullable() {
			return Value{}, xerrors.Errorf("cannot convert nil to %v", typ)
		}
		return Value{typ: typ, val: nil}, nil
	}
	if marshaler, ok := goIface.(encoding.TextMarshaler); ok {
		b, err := marshaler.MarshalText()
		if err != nil {
			return Value{}, err
		}
		// TODO(someday): Ensure marshaled value can be interpreted as the GraphQL type.
		val := string(b)
		if typ.isEnum() && !typ.enum.has(val) {
			return Value{typ: typ}, xerrors.Errorf("%q is not a valid value for %v", val, typ)
		}
		return Value{typ: typ, val: val}, nil
	}
	switch typ.toNullable() {
	case booleanType:
		if goValue.Kind() != reflect.Bool {
			return Value{}, xerrors.Errorf("cannot convert %v to %v", goValue.Type(), typ)
		}
		return Value{typ: typ, val: strconv.FormatBool(goValue.Bool())}, nil
	case intType:
		if goValue.Kind() != reflect.Int32 && goValue.Kind() != reflect.Int {
			return Value{}, xerrors.Errorf("cannot convert %v to %v", goValue.Type(), typ)
		}
		i := goValue.Int()
		const maxInt32 = 1 << 31
		const minInt32 = -maxInt32 - 1
		if i < minInt32 || maxInt32 < i {
			return Value{}, xerrors.New("integer out of GraphQL range")
		}
		return Value{typ: typ, val: strconv.FormatInt(i, 10)}, nil
	case floatType:
		var bitSize int
		switch goValue.Kind() {
		case reflect.Float32:
			bitSize = 32
		case reflect.Float64:
			bitSize = 64
		default:
			return Value{}, xerrors.Errorf("cannot convert %v to %v", goValue.Type(), typ)
		}
		val := strconv.FormatFloat(goValue.Float(), 'g', -1, bitSize)
		return Value{typ: typ, val: val}, nil
	case idType:
		if k := goValue.Kind(); k == reflect.Int32 || k == reflect.Int || k == reflect.Int64 {
			return Value{typ: typ, val: strconv.FormatInt(goValue.Int(), 10)}, nil
		}
		fallthrough
	default:
		if goValue.Kind() != reflect.String {
			return Value{typ: typ}, xerrors.Errorf("cannot convert %v to %v", goValue.Type(), typ)
		}
		val := goValue.String()
		if typ.isEnum() && !typ.enum.has(val) {
			return Value{typ: typ}, xerrors.Errorf("%q is not a valid value for %v", val, typ)
		}
		return Value{typ: typ, val: val}, nil
	}
}

func unwrapPointer(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

// valueForAssertions returns the value's innermost pointer or v itself
// if v does not represent a pointer.
func valueForAssertions(v reflect.Value) reflect.Value {
	v = unwrapPointer(v)
	if !v.IsValid() || v.Kind() == reflect.Interface || !v.CanAddr() {
		return v
	}
	return v.Addr()
}

// interfaceValueForAssertions returns the value's innermost pointer or v itself
// if v does not represent a pointer.
func interfaceValueForAssertions(v reflect.Value) interface{} {
	v = valueForAssertions(v)
	if !v.IsValid() {
		return nil
	}
	return v.Interface()
}

// GoValue dumps the value into one of the following Go types:
//
//   - nil interface{} for null
//   - string for scalars
//   - []interface{} for lists
//   - map[string]interface{} for objects
func (v Value) GoValue() interface{} {
	switch val := v.val.(type) {
	case nil:
		return nil
	case string:
		return val
	case []Value:
		goVal := make([]interface{}, len(val))
		for i, vv := range val {
			goVal[i] = vv.GoValue()
		}
		return goVal
	case []Field:
		goVal := make(map[string]interface{}, len(val))
		for _, f := range val {
			goVal[f.Key] = f.Value.GoValue()
		}
		return goVal
	case map[string]Value:
		goVal := make(map[string]interface{}, len(val))
		for k, vv := range val {
			goVal[k] = vv.GoValue()
		}
		return goVal
	default:
		panic("unknown type in Value.val")
	}
}

// IsNull reports whether v is null.
func (v Value) IsNull() bool {
	return v.val == nil
}

// Boolean reports if v is a scalar with the value "true".
func (v Value) Boolean() bool {
	return v.val == "true"
}

// Scalar returns the string value of v if it is a scalar or the empty
// string otherwise.
func (v Value) Scalar() string {
	s, _ := v.val.(string)
	return s
}

// Len returns the number of elements in v. Len panics if v is not a list or null.
func (v Value) Len() int {
	if v.val == nil {
		return 0
	}
	return len(v.val.([]Value))
}

// At returns v's i'th element. At panics if v is not a list or i is not in the
// range [0, v.Len()).
func (v Value) At(i int) Value {
	list := v.val.([]Value)
	return list[i]
}

// NumFields returns the number of fields in v. NumFields panics if v is not
// null, an object, or an input object.
func (v Value) NumFields() int {
	switch val := v.val.(type) {
	case nil:
		return 0
	case []Field:
		return len(val)
	case map[string]Field:
		return len(val)
	default:
		panic(fmt.Sprintf("invalid value for NumFields: %T", v.val))
	}
}

// Field returns v's i'th field. Field panics if v is not an object or i is not
// in the range [0, v.NumFields()).
func (v Value) Field(i int) Field {
	fields := v.val.([]Field)
	return fields[i]
}

// ValueFor returns the value of the field with the given key or the zero Value
// if v does not have the given key. ValueFor panics if v is not an object or
// input object.
func (v Value) ValueFor(key string) Value {
	switch val := v.val.(type) {
	case []Field:
		for _, f := range val {
			if f.Key == key {
				return f.Value
			}
		}
		return Value{}
	case map[string]Value:
		return val[key]
	default:
		panic(fmt.Sprintf("invalid value for ValueFor(): %T", v.val))
	}
}

// MarshalJSON converts the value to JSON.
func (v Value) MarshalJSON() ([]byte, error) {
	switch val := v.val.(type) {
	case nil:
		return []byte("null"), nil
	case string:
		if typ := v.typ.toNullable(); typ == booleanType || typ == intType || typ == floatType {
			// Can use as JSON literal.
			return []byte(val), nil
		}
		return json.Marshal(val)
	case []Value, map[string]Value:
		return json.Marshal(val)
	case []Field:
		var buf []byte
		buf = append(buf, '{')
		for i, f := range val {
			if i > 0 {
				buf = append(buf, ',')
			}
			key, err := json.Marshal(f.Key)
			if err != nil {
				return nil, err
			}
			buf = append(buf, key...)
			buf = append(buf, ':')
			fval, err := json.Marshal(f.Value)
			if err != nil {
				return nil, err
			}
			buf = append(buf, fval...)
		}
		buf = append(buf, '}')
		return buf, nil
	default:
		panic("unknown type in Value.typ")
	}
}
