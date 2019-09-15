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

	"golang.org/x/xerrors"
)

// Input is a typeless GraphQL value. The zero value is null.
type Input struct {
	val interface{} // one of nil, string, map[string]Input, or []Input
}

// IsNull reports whether the input represents the null value.
func (in Input) IsNull() bool {
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
