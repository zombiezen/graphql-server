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

Field Resolution

When executing a request, the server will first check the object to see whether
it implements FieldResolver. If so, the ResolveField method will be called for
any field on the object.

Next, the server checks for a method with the same name as the field. Field
methods must have the following signature (with square brackets indicating
optional elements):

	func (foo *Foo) Bar([ctx context.Context,] [args ArgsType,] [sel *graphql.SelectionSet]) (ResultType[, error])

The ctx parameter will have a Context deriving from the one passed to Execute.
The args parameter can be of type map[string]graphql.Value, S, or *S, where S is
a struct type with fields for all of the arguments. See ConvertValueMap for a
description of how this parameter is derived from the field arguments. The sel
parameter is only passed to fields that return an object or list of objects type
and permits the method to peek into what fields will be evaluated on its return
value. This is useful for avoiding querying for data that won't be used in the
response. The method must be exported, but otherwise methods are matched with
fields ignoring case.

Lastly, if the object is a Go struct and the field takes no arguments, then the
server will read the value from an exported struct field with the same name
ignoring case.

Type Resolution

For abstract types, the server will first attempt to call a GraphQLType method
as documented in the Typer interface. If that's not present, the Go type name
will be matched with the GraphQL type with the same name ignoring case if
present. Otherwise, type resolution fails.

Scalars

Go values will be converted to scalars in the result by trying the following
in order:

	1) Call a method named IsGraphQLNull if present. If it returns true, then
	convert to null.

	2) Use the encoding.TextMarshaler interface if present.

	3) Examine the Go type and GraphQL types and attempt coercion.
*/
package graphql
