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
	val interface{} // one of nil, string, []Value, []field, or map[string]Value.
}

type field struct {
	name  string
	value Value
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
	case typ.scalar != "":
		v, err := scalarFromGo(goValue, typ)
		if err != nil {
			return Value{typ: typ}, []error{err}
		}
		return v, nil
	case typ.list != nil:
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
	case typ.obj != nil:
		gqlFields := make([]field, 0, len(sel.fields))
		for _, f := range sel.fields {
			fval, err := readField(ctx, goValue, f.name, f.args, typ.obj.fields[f.name], f.sub)
			if err != nil {
				return Value{}, err
			}
			gqlFields = append(gqlFields, field{name: f.name, value: fval})
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
	method := goValue.MethodByName(goName)
	if !method.IsValid() {
		return Value{typ: typ}, []error{xerrors.Errorf("field %s: no such method or field on %v", name, goValue.Type())}
	}
	// TODO(soon): Dynamically adapt to parameters available.
	callArgs := []reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(args),
	}
	if sel != nil {
		callArgs = append(callArgs, reflect.ValueOf(sel))
	}
	callReturns := method.Call(callArgs)
	if !callReturns[1].IsNil() {
		err := callReturns[1].Interface().(error)
		return Value{typ: typ}, []error{xerrors.Errorf("field %s: %w", name, err)}
	}
	ret, errs := valueFromGo(ctx, callReturns[0], typ, sel)
	if len(errs) > 0 {
		for i := range errs {
			errs[i] = xerrors.Errorf("field %s: %w", name, errs[i])
	}
		return Value{typ: typ}, errs
	}
	return ret, nil
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
		if goValue.Kind() != reflect.Int32 || goValue.Kind() != reflect.Int {
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
	case []field:
		goVal := make(map[string]interface{}, len(val))
		for _, f := range val {
			goVal[f.name] = f.value.GoValue()
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
	case []field:
		var buf []byte
		buf = append(buf, '{')
		for i, f := range val {
			if i > 0 {
				buf = append(buf, ',')
			}
			key, err := json.Marshal(f.name)
			if err != nil {
				return nil, err
			}
			buf = append(buf, key...)
			buf = append(buf, ':')
			fval, err := json.Marshal(f.value)
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
