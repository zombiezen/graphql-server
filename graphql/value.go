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
)

// A Value is a typed GraphQL datum.
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
func valueFromGo(ctx context.Context, goValue reflect.Value, typ *gqlType, sel *SelectionSet) (Value, []error) {
	// Since this function is recursive, caller must prepend error operation.

	goValue = unwrapPointer(goValue)
	if !goValue.IsValid() {
		if !typ.isNullable() {
			return Value{typ: typ}, []error{xerrors.Errorf("cannot convert nil to %v", typ)}
		}
		return Value{typ: typ, val: nil}, nil
	}
	switch {
	case typ.isScalar():
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
		for i := range gqlValues {
			var errs []error
			gqlValues[i], errs = valueFromGo(ctx, goValue.Index(i), typ.list, sel)
			if len(errs) > 0 {
				// TODO(soon): Wrap with path segment.
				for j := range errs {
					errs[j] = xerrors.Errorf("list value [%d]: %w", i, errs[j])
				}
				// TODO(soon): Only return if element types are non-nullable.
				return Value{typ: typ}, errs
			}
		}
		return Value{typ: typ, val: gqlValues}, nil
	case typ.isObject():
		if sel == nil {
			return Value{typ: typ, val: []Field(nil)}, nil
		}
		gqlFields := make([]Field, 0, len(sel.fields))
		for _, f := range sel.fields {
			fval, err := readField(ctx, goValue, f.name, f.args, typ.obj.fields[f.name], f.sub)
			if err != nil {
				return Value{}, err
			}
			gqlFields = append(gqlFields, Field{Key: f.key, Value: fval})
		}
		return Value{typ: typ, val: gqlFields}, nil
	default:
		return Value{typ: typ}, []error{xerrors.Errorf("unhandled type: %v", typ)}
	}
}

func readField(ctx context.Context, goValue reflect.Value, name string, args map[string]Value, typ *gqlType, sel *SelectionSet) (Value, []error) {
	// TODO(soon): Wrap any error in this function with a field path segment.

	// TODO(soon): Search over all fields and/or methods to find case-insensitive match.
	goName := graphQLToGoFieldName(name)

	if len(args) == 0 && goValue.Kind() == reflect.Struct {
		if fieldValue := goValue.FieldByName(goName); fieldValue.IsValid() {
			v, err := valueFromGo(ctx, fieldValue, typ, sel)
			if err != nil {
				return Value{typ: typ}, []error{xerrors.Errorf("field %s: %w", name, err)}
			}
			return v, nil
		}
	}
	method := findFieldMethod(goValue, name)
	if !method.IsValid() {
		return Value{typ: typ}, []error{xerrors.Errorf("field %s: no such method or field on %v", name, goValue.Type())}
	}
	methodResult, err := callFieldMethod(ctx, method, args, sel, typ.selectionSetType() != nil)
	if err != nil {
		return Value{typ: typ}, []error{xerrors.Errorf("field %s: %w", name, err)}
	}
	ret, errs := valueFromGo(ctx, methodResult, typ, sel)
	if len(errs) > 0 {
		for i := range errs {
			errs[i] = xerrors.Errorf("field %s: %w", name, errs[i])
		}
	}
	return ret, errs
}

func findFieldMethod(v reflect.Value, name string) reflect.Value {
	// TODO(soon): Search over all fields and/or methods to find case-insensitive match.

	v = unwrapPointer(v)
	if v.Kind() != reflect.Interface && v.CanAddr() {
		v = v.Addr()
	}
	return v.MethodByName(graphQLToGoFieldName(name))
}

var (
	contextGoType      = reflect.TypeOf(new(context.Context)).Elem()
	argsGoType         = reflect.TypeOf(new(map[string]Value)).Elem()
	selectionSetGoType = reflect.TypeOf(new(*SelectionSet)).Elem()
	errorGoType        = reflect.TypeOf(new(error)).Elem()
)

func callFieldMethod(ctx context.Context, method reflect.Value, args map[string]Value, sel *SelectionSet, passSel bool) (reflect.Value, error) {
	mtype := method.Type()
	numIn := mtype.NumIn()
	var callArgs []reflect.Value
	if len(callArgs) < numIn && mtype.In(len(callArgs)) == contextGoType {
		callArgs = append(callArgs, reflect.ValueOf(ctx))
	}
	if len(callArgs) < numIn && mtype.In(len(callArgs)) == argsGoType {
		callArgs = append(callArgs, reflect.ValueOf(args))
	}
	if passSel {
		if len(callArgs) < numIn && mtype.In(len(callArgs)) == selectionSetGoType {
			callArgs = append(callArgs, reflect.ValueOf(sel))
		}
	}
	if len(callArgs) != numIn {
		return reflect.Value{}, xerrors.New("call field method: wrong parameter signature")
	}

	switch mtype.NumOut() {
	case 1:
		if mtype.Out(0) == errorGoType {
			return reflect.Value{}, xerrors.New("call field method: return type must not be error")
		}
		out := method.Call(callArgs)
		return out[0], nil
	case 2:
		if mtype.Out(0) == errorGoType {
			return reflect.Value{}, xerrors.New("call field method: first return type must not be error")
		}
		if got := mtype.Out(1); got != errorGoType {
			return reflect.Value{}, xerrors.Errorf("call field method: second return type must be error (found %v)", got)
		}
		out := method.Call(callArgs)
		if !out[1].IsNil() {
			err := out[1].Interface().(error)
			return reflect.Value{}, xerrors.Errorf("call field method: %w", err)
		}
		return out[0], nil
	default:
		return reflect.Value{}, xerrors.New("call field method: wrong return signature")
	}
}

func scalarFromGo(goValue reflect.Value, typ *gqlType) (Value, error) {
	goValue = unwrapPointer(goValue)
	if !goValue.IsValid() {
		if !typ.isNullable() {
			return Value{}, xerrors.Errorf("cannot convert nil to %v", typ)
		}
		return Value{typ: typ, val: nil}, nil
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
	default:
		switch goIface := interfaceValueForAssertions(goValue).(type) {
		case encoding.TextMarshaler:
			text, err := goIface.MarshalText()
			if err != nil {
				return Value{}, err
			}
			return Value{typ: typ, val: string(text)}, nil
		case fmt.Stringer:
			return Value{typ: typ, val: goIface.String()}, nil
		}
		if goValue.Kind() != reflect.String {
			return Value{}, xerrors.Errorf("cannot convert %v to %v", goValue.Type(), typ)
		}
		return Value{typ: typ, val: goValue.String()}, nil
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

// interfaceValueForAssertions returns the value's innermost pointer or v itself
// if v does not represent a pointer.
func interfaceValueForAssertions(v reflect.Value) interface{} {
	v = unwrapPointer(v)
	if v.Kind() == reflect.Interface || !v.CanAddr() {
		return v.Interface()
	}
	return v.Addr().Interface()
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

// Len returns the number of elements or fields in v. Len panics if v is not a
// list, object, or input object.
func (v Value) Len() int {
	switch val := v.val.(type) {
	case []Value:
		return len(val)
	case []Field:
		return len(val)
	case map[string]Value:
		return len(val)
	default:
		panic(fmt.Sprintf("invalid value for Len(): %T", v.val))
	}
}

// Field returns v's i'th field. Field panics if v is not an object or i is not
// in the range [0, Len()).
func (v Value) Field(i int) Field {
	fields := v.val.([]Field)
	return fields[i]
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

func graphQLToGoFieldName(name string) string {
	if c := name[0]; 'a' <= c && c <= 'z' {
		return string(c-'a'+'A') + name[1:]
	}
	return name
}
