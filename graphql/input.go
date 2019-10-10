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
	"bytes"
	"encoding/json"
	"fmt"

	"golang.org/x/xerrors"
	"zombiezen.com/go/graphql-server/internal/gqlang"
)

// Input is a typeless GraphQL value. The zero value is null.
type Input struct {
	val interface{} // one of nil, string, map[string]Input, or []Input
}

// ScalarInput returns a new input with the given scalar value.
func ScalarInput(s string) Input {
	return Input{val: s}
}

// InputObject returns a new input with the given scalar value.
func InputObject(obj map[string]Input) Input {
	return Input{val: obj}
}

// ListInput returns a new input with the given scalar value.
func ListInput(list []Input) Input {
	return Input{val: list}
}

// isNull reports whether the input represents the null value.
func (in Input) isNull() bool {
	return in.val == nil
}

// MarshalJSON converts the input into JSON. All scalars will be represented as
// strings.
func (in Input) MarshalJSON() ([]byte, error) {
	return json.Marshal(in.val)
}

// UnmarshalJSON converts JSON into an input.
func (in *Input) UnmarshalJSON(data []byte) error {
	// TODO(someday): Deal with deeply nested JSON.

	if len(data) == 0 {
		return xerrors.New("unmarshal input json: empty")
	}
	switch {
	case data[0] == '{':
		var m map[string]Input
		if err := json.Unmarshal(data, &m); err != nil {
			return err
		}
		in.val = m
	case data[0] == '[':
		var l []Input
		if err := json.Unmarshal(data, &l); err != nil {
			return err
		}
		in.val = l
	case data[0] == '"':
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		in.val = s
	case bytes.Equal(data, []byte("null")):
		in.val = nil
	default:
		// A literal of some sort: number or boolean.
		in.val = string(data)
	}
	return nil
}

// GoValue dumps the input into one of the following Go types:
//
//   - nil interface{} for null
//   - string for scalars
//   - []interface{} for lists
//   - map[string]interface{} for objects
func (in Input) GoValue() interface{} {
	switch val := in.val.(type) {
	case nil:
		return nil
	case string:
		return val
	case []Input:
		goVal := make([]interface{}, len(val))
		for i, vv := range val {
			goVal[i] = vv.GoValue()
		}
		return goVal
	case map[string]Input:
		goVal := make(map[string]interface{}, len(val))
		for k, vv := range val {
			goVal[k] = vv.GoValue()
		}
		return goVal
	default:
		panic("unknown type in Input.val")
	}
}

// coerceVariableValues converts inputs into values.
// The procedure is specified in https://graphql.github.io/graphql-spec/June2018/#CoerceVariableValues()
func coerceVariableValues(source string, typeMap map[string]*gqlType, vars map[string]Input, defns *gqlang.VariableDefinitions) (map[string]Value, []error) {
	if defns == nil {
		return nil, nil
	}
	coerced := make(map[string]Value)
	var errs []error
	for _, defn := range defns.Defs {
		name := defn.Var.Name.Value
		typ := resolveTypeRef(typeMap, defn.Type)
		input, hasValue := vars[name]
		switch {
		case !hasValue && defn.Default != nil:
			coerced[name] = coerceConstantInputValue(typ, defn.Default.Value)
		case !typ.isNullable() && input.isNull():
			errs = append(errs, &ResponseError{
				Message: fmt.Sprintf("variable $%s is required", name),
				Locations: []Location{
					astPositionToLocation(defn.Var.Dollar.ToPosition(source)),
				},
			})
		case hasValue && input.isNull():
			coerced[name] = Value{typ: typ}
		case hasValue && !input.isNull():
			var varErrs []error
			coerced[name], varErrs = coerceInput(typ, input)
			for _, err := range varErrs {
				errs = append(errs, xerrors.Errorf("variable $%s: %w", name, err))
			}
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return coerced, nil
}

func coerceInput(typ *gqlType, input Input) (Value, []error) {
	// This is distinct from coerceInputValue because there's no error position
	// information associated. Creating a unified structure to increase code
	// reuse would likely bring more confusion and a performance hit over the
	// current situation.

	if input.isNull() {
		if !typ.isNullable() {
			return Value{typ: typ}, []error{xerrors.Errorf("null not permitted for %v", typ)}
		}
		return Value{typ: typ}, nil
	}
	switch {
	case typ.isScalar():
		scalar, ok := input.val.(string)
		if !ok {
			return Value{typ: typ}, []error{xerrors.Errorf("non-scalar found for %v", typ)}
		}
		if err := validateScalar(typ, scalar, noAffinity); err != nil {
			return Value{typ: typ}, []error{err}
		}
		return Value{typ: typ, val: scalar}, nil
	case typ.isEnum():
		scalar, ok := input.val.(string)
		if !ok {
			return Value{typ: typ}, []error{xerrors.Errorf("non-scalar found for %v", typ)}
		}
		if !typ.enum.has(scalar) {
			return Value{typ: typ}, []error{xerrors.Errorf("%q is not a valid value for %v", scalar, typ)}
		}
		return Value{typ: typ, val: scalar}, nil
	case typ.isList():
		inputList, ok := input.val.([]Input)
		if !ok {
			// Attempt to coerce as single-element list.
			// Yes, I'm just as surprised as you are at this behavior,
			// see https://graphql.github.io/graphql-spec/June2018/#sec-Type-System.List
			value, errs := coerceInput(typ.listElem, input)
			if len(errs) > 0 {
				return Value{typ: typ}, errs
			}
			return Value{
				typ: typ,
				val: []Value{value},
			}, nil
		}
		valueList := make([]Value, 0, len(inputList))
		var errs []error
		for i, elem := range inputList {
			elemValue, elemErrs := coerceInput(typ.listElem, elem)
			valueList = append(valueList, elemValue)
			for _, err := range elemErrs {
				errs = append(errs, xerrors.Errorf("list[%d]: %w", i, err))
			}
		}
		return Value{typ: typ, val: valueList}, errs
	case typ.isInputObject():
		inputObj, ok := input.val.(map[string]Input)
		if !ok {
			return Value{typ: typ}, []error{xerrors.Errorf("non-object found for %v", typ)}
		}
		valueMap := make(map[string]Value)
		var errs []error
		for name := range inputObj {
			if _, exists := typ.input.fields[name]; !exists {
				// https://graphql.github.io/graphql-spec/June2018/#sec-Input-Object-Field-Names
				errs = append(errs, xerrors.Errorf("unknown input field %s for %v", name, typ))
			}
		}
		// https://graphql.github.io/graphql-spec/June2018/#sec-Input-Object-Required-Fields
		for name, defn := range typ.input.fields {
			if _, hasValue := inputObj[name]; !hasValue {
				if !defn.typ().isNullable() && defn.defaultValue.IsNull() {
					errs = append(errs, xerrors.Errorf("missing required input field for %v.%s", typ.toNullable(), name))
				}
				continue
			}
			field := inputObj[name]
			if !defn.typ().isNullable() && field.isNull() {
				errs = append(errs, xerrors.Errorf("required input field %v.%s is null", typ.toNullable(), name))
				continue
			}
			var fieldErrs []error
			// TODO(now): Write test
			valueMap[name], fieldErrs = coerceInput(defn.typ(), field)
			for _, err := range fieldErrs {
				errs = append(errs, xerrors.Errorf("input field %s: %w", name, err))
			}
		}
		if len(errs) > 0 {
			return Value{typ: typ}, errs
		}
		return Value{typ: typ, val: valueMap}, nil
	default:
		panic("unhandled input type")
	}
}
