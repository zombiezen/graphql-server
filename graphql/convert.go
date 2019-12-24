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
	"encoding"
	"reflect"
	"strconv"

	"golang.org/x/xerrors"
)

// Convert converts a GraphQL value into a Go value. dst must be a non-nil
// pointer.
//
// For scalars, Convert will first attempt to use the encoding.TextUnmarshaler
// interface, if present. Next, Convert will try to convert the scalar to the
// Go type. A Go string will use the scalar's value verbatim. Numeric types will
// be converted by parsing the scalar as a number. Boolean types will be
// converted from the scalars "true" and "false".
//
// For enums, Convert behaves like scalars as described above, but if
// encoding.TextUnmarshaler is not implemented, then the enum may only be
// converted into a string.
//
// For objects and input objects, the Go value must either be a struct or a
// map[string]graphql.Value. During conversion to a struct, the GraphQL value's
// fields will be converted (as if by Convert) into the struct field with the
// same name, ignoring case. An error will be returned if a field in the GraphQL
// value does not match exactly one field in the Go struct. Conversion to a
// map[string]graphql.Value will use the field keys and copy the values.
//
// Lists will be converted into Go slices. Elements are converted using the same
// rules as Convert.
//
// If the Go value is of type graphql.Value, then the Value is copied verbatim.
//
// Null will be converted to the zero value of that type.
func (v Value) Convert(dst interface{}) error {
	dstValue := reflect.ValueOf(dst)
	if dstValue.Kind() != reflect.Ptr {
		return xerrors.Errorf("convert GraphQL value: argument not a pointer")
	}
	if err := v.convert(dstValue.Elem()); err != nil {
		return xerrors.Errorf("convert GraphQL value: %w", err)
	}
	return nil
}

func (v Value) convert(dst reflect.Value) error {
	if !dst.CanSet() {
		return xerrors.Errorf("cannot convert to unsettable value")
	}
	if v.IsNull() {
		dst.Set(reflect.Zero(dst.Type()))
		return nil
	}
	goType := dst.Type()
	kind := goType.Kind()
	for kind == reflect.Ptr {
		elemType := goType.Elem()
		if dst.IsNil() {
			// Create new values if pointers are nil.
			dst.Set(reflect.New(elemType))
		}
		dst = dst.Elem()
		goType = elemType
		kind = elemType.Kind()
	}
	convertErr := func() error {
		return xerrors.Errorf("cannot assign value of type %v to Go type %v", v.typ, goType)
	}
	switch val := v.val.(type) {
	case string:
		if u, ok := interfaceValueForAssertions(dst).(encoding.TextUnmarshaler); ok {
			return u.UnmarshalText([]byte(val))
		}
		if kind == reflect.String {
			dst.SetString(val)
			return nil
		}
		if v.typ.isEnum() {
			return convertErr()
		}
		valueErr := func() error {
			return xerrors.Errorf("cannot convert %q to %v", val, goType)
		}
		switch kind {
		case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64:
			i, err := strconv.ParseInt(val, 10, goType.Bits())
			if err != nil {
				return valueErr()
			}
			dst.SetInt(i)
		case reflect.Float32, reflect.Float64:
			f, err := strconv.ParseFloat(val, goType.Bits())
			if err != nil {
				return valueErr()
			}
			dst.SetFloat(f)
		case reflect.Bool:
			switch val {
			case "false":
				dst.SetBool(false)
			case "true":
				dst.SetBool(true)
			default:
				return valueErr()
			}
		default:
			return convertErr()
		}
	case []Value:
		if kind != reflect.Slice {
			return convertErr()
		}
		dst.Set(reflect.MakeSlice(goType, 0, len(val)))
		for dst.Len() < len(val) {
			i := dst.Len()
			next := dst.Slice(0, i+1)
			if err := val[i].convert(next.Index(i)); err != nil {
				return xerrors.Errorf("element[%d]: %w", i, err)
			}
			dst.Set(next)
		}
	case []Field:
		if goType == valueMapGoType {
			m := make(map[string]Value, len(val))
			for _, f := range val {
				m[f.Key] = f.Value
			}
			dst.Set(reflect.ValueOf(m))
			return nil
		}
		if kind != reflect.Struct {
			return convertErr()
		}
		for _, f := range val {
			fieldIndex, err := findConvertField(goType, f.Key)
			if err != nil {
				return err
			}
			if err := f.Value.convert(dst.Field(fieldIndex)); err != nil {
				return xerrors.Errorf("field %s: %w", f.Key)
			}
		}
	case map[string]Value:
		if goType == valueMapGoType {
			m := make(map[string]Value, len(val))
			for k, v := range val {
				m[k] = v
			}
			dst.Set(reflect.ValueOf(m))
			return nil
		}
		if kind != reflect.Struct {
			return convertErr()
		}
		for k, v := range val {
			fieldIndex, err := findConvertField(goType, k)
			if err != nil {
				return err
			}
			if err := v.convert(dst.Field(fieldIndex)); err != nil {
				return xerrors.Errorf("field %s: %w", k)
			}
		}
	default:
		panic("unknown type in Value")
	}
	return nil
}

// findConvertField returns the field index of a Go struct that's suitable for
// the given GraphQL field name.
func findConvertField(goType reflect.Type, name string) (int, error) {
	var index int
	numMatches := 0
	lowerFieldName := toLower(name)
	for i, n := 0, goType.NumField(); i < n; i++ {
		goField := goType.Field(i)
		if goField.PkgPath != "" {
			// Don't consider unexported fields.
			continue
		}
		if toLower(goField.Name) == lowerFieldName {
			index = i
			numMatches++
		}
	}
	if numMatches == 0 {
		return -1, xerrors.Errorf("field %s: %v has no matching field", name, goType)
	}
	if numMatches > 1 {
		return -1, xerrors.Errorf("field %s: %v has multiple matching fields", name, goType)
	}
	return index, nil
}
