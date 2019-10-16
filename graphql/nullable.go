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
	"strconv"

	"golang.org/x/xerrors"
)

// Nullable defines the IsGraphQLNull method. IsGraphQLNull reports whether the
// receiver should be represented in GraphQL as null.
type Nullable interface {
	IsGraphQLNull() bool
}

func isGraphQLNull(x interface{}) bool {
	n, ok := x.(Nullable)
	return ok && n.IsGraphQLNull()
}

// NullInt represents an Int that may be null. The zero value is null.
type NullInt struct {
	Int   int32
	Valid bool
}

// IsGraphQLNull returns !n.Valid.
func (n NullInt) IsGraphQLNull() bool {
	return !n.Valid
}

// String returns the decimal representation or "null".
func (n NullInt) String() string {
	if !n.Valid {
		return "null"
	}
	return strconv.FormatInt(int64(n.Int), 10)
}

// MarshalText marshals the integer to a decimal representation. It returns an
// error if n.Valid is false.
func (n NullInt) MarshalText() ([]byte, error) {
	if !n.Valid {
		return nil, errMarshalNull
	}
	return strconv.AppendInt(nil, int64(n.Int), 10), nil
}

// UnmarshalText unmarshals a decimal integer.
func (n *NullInt) UnmarshalText(text []byte) error {
	i, err := strconv.ParseInt(string(text), 10, 32)
	if err != nil {
		return xerrors.Errorf("unmarshal: invalid int: %w", err)
	}
	*n = NullInt{Valid: true, Int: int32(i)}
	return nil
}

// NullFloat represents an Float that may be null. The zero value is null.
type NullFloat struct {
	Float float64
	Valid bool
}

// IsGraphQLNull returns !n.Valid.
func (n NullFloat) IsGraphQLNull() bool {
	return !n.Valid
}

// String returns the decimal representation (using scientific notation for
// large exponents) or "null".
func (n NullFloat) String() string {
	if !n.Valid {
		return "null"
	}
	return strconv.FormatFloat(n.Float, 'g', -1, 64)
}

// MarshalText marshals the floating point number to a decimal representation
// (or scientific notation for large exponents). It returns an error if n.Valid
// is false.
func (n NullFloat) MarshalText() ([]byte, error) {
	if !n.Valid {
		return nil, errMarshalNull
	}
	return strconv.AppendFloat(nil, n.Float, 'g', -1, 64), nil
}

// UnmarshalText unmarshals a floating point or integer literal.
func (n *NullFloat) UnmarshalText(text []byte) error {
	f, err := strconv.ParseFloat(string(text), 64)
	if err != nil {
		return xerrors.Errorf("unmarshal: invalid float: %w", err)
	}
	*n = NullFloat{Valid: true, Float: f}
	return nil
}

// NullString represents a String that may be null. The zero value is null.
type NullString struct {
	S     string
	Valid bool
}

// IsGraphQLNull returns !n.Valid.
func (n NullString) IsGraphQLNull() bool {
	return !n.Valid
}

// String returns n.S or "null".
func (n NullString) String() string {
	if !n.Valid {
		return "null"
	}
	return n.S
}

// MarshalText converts n.S to []byte. It returns an error if n.Valid is false.
func (n NullString) MarshalText() ([]byte, error) {
	if !n.Valid {
		return nil, errMarshalNull
	}
	return []byte(n.S), nil
}

// UnmarshalText converts the byte slice to a string.
func (n *NullString) UnmarshalText(text []byte) error {
	*n = NullString{Valid: true, S: string(text)}
	return nil
}

// NullBoolean represents a Boolean that may be null. The zero value is null.
type NullBoolean struct {
	Bool  bool
	Valid bool
}

// IsGraphQLNull returns !n.Valid.
func (n NullBoolean) IsGraphQLNull() bool {
	return !n.Valid
}

// String returns "true", "false", or "null".
func (n NullBoolean) String() string {
	switch {
	case n.Valid && n.Bool:
		return "true"
	case n.Valid && !n.Bool:
		return "false"
	default:
		return "null"
	}
}

// MarshalText marshals the boolean to "true" or "false". It returns an error
// if n.Valid is false.
func (n NullBoolean) MarshalText() ([]byte, error) {
	if !n.Valid {
		return nil, errMarshalNull
	}
	return []byte(strconv.FormatBool(n.Bool)), nil
}

// UnmarshalText unmarshals a "true" or "false" into the boolean.
func (n *NullBoolean) UnmarshalText(text []byte) error {
	switch string(text) {
	case "true":
		*n = NullBoolean{Valid: true, Bool: true}
	case "false":
		*n = NullBoolean{Valid: true, Bool: false}
	default:
		return xerrors.Errorf("unmarshal: invalid boolean %q", text)
	}
	return nil
}

var errMarshalNull = xerrors.New("marshal null")
