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

/*
Package graphql provides a GraphQL execution engine. During execution, a GraphQL
server transforms requests into Go method calls and struct field accesses. This
package follows the specification laid out at https://graphql.github.io/graphql-spec/June2018/

For the common case where you are serving GraphQL over HTTP, see the graphqlhttp
package in this module.

Methods

Field methods must have the following signature (with square brackets
indicating optional elements):

	func (foo *Foo) Bar([ctx context.Context,] [args map[string]graphql.Value,] [sel *graphql.SelectionSet]) (ResultType[, error])

The ctx parameter will have a Context deriving from the one passed to Execute.
The args parameter will be a map filled with the arguments passed to the field.
The sel parameter is only passed to fields that return an object or list of
objects type and permits the method to peek into what fields will be evaluated
on its return value. This is useful for avoiding querying for data that won't
be used in the response.

Scalars

Go values will be converted to scalars in the result by trying the following
in order:

	1) Call a method named IsGraphQLNull if present. If it returns true, then
	convert to null.

	2) Use the encoding.TextMarshaler interface if present.

	3) Examine the Go type and GraphQL types and attempt coercion.
*/
package graphql
