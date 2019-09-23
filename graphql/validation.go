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
	"fmt"

	"zombiezen.com/go/graphql-server/internal/gqlang"
)

// validateRequest validates a parsed GraphQL request according to the procedure
// defined in https://graphql.github.io/graphql-spec/June2018/#sec-Validation.
func (schema *Schema) validateRequest(input string, doc *gqlang.Document) []error {
	var errs []error
	var anonPosList []gqlang.Pos
	operationsByName := make(map[string][]gqlang.Pos)
	for _, defn := range doc.Definitions {
		if defn.Operation == nil {
			// https://graphql.github.io/graphql-spec/June2018/#sec-Executable-Definitions
			errs = append(errs, &ResponseError{
				Message: "not an operation nor a fragment",
				Locations: []Location{
					astPositionToLocation(defn.Start().ToPosition(input)),
				},
			})
			continue
		}
		if defn.Operation.Name == nil {
			anonPosList = append(anonPosList, defn.Operation.Start)
			continue
		}
		name := defn.Operation.Name.Value
		operationsByName[name] = append(operationsByName[name], defn.Operation.Name.Start)
	}
	if len(anonPosList) > 1 {
		// https://graphql.github.io/graphql-spec/June2018/#sec-Lone-Anonymous-Operation
		errs = append(errs, &ResponseError{
			Message:   "multiple anonymous operations",
			Locations: posListToLocationList(input, anonPosList),
		})
	}
	if len(anonPosList) > 0 && len(operationsByName) > 0 {
		// https://graphql.github.io/graphql-spec/June2018/#sec-Lone-Anonymous-Operation
		errs = append(errs, &ResponseError{
			Message:   "anonymous operations mixed with named operations",
			Locations: posListToLocationList(input, anonPosList),
		})
	}
	for name, posList := range operationsByName {
		if len(posList) > 1 {
			// https://graphql.github.io/graphql-spec/June2018/#sec-Operation-Name-Uniqueness
			errs = append(errs, &ResponseError{
				Message:   fmt.Sprintf("multiple operations with name %q", name),
				Locations: posListToLocationList(input, posList),
			})
		}
	}
	if len(errs) > 0 {
		return errs
	}
	// TODO(soon): Validate the rest.
	return nil
}

func posListToLocationList(input string, posList []gqlang.Pos) []Location {
	locList := make([]Location, len(posList))
	for i := range locList {
		locList[i] = astPositionToLocation(posList[i].ToPosition(input))
	}
	return locList
}