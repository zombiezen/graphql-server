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

package graphql_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"zombiezen.com/go/graphql-server/graphql"
)

// GraphQL requests and response can be converted to JSON using the
// standard encoding/json package.
func Example_json() {
	// Create a schema and a server.
	server := newServer()

	// Use json.Unmarshal to parse a GraphQL request from JSON.
	var request graphql.Request
	err := json.Unmarshal([]byte(`{
		"query": "{ genericGreeting }"
	}`), &request)
	if err != nil {
		log.Fatal(err)
	}

	// Use json.Marshal to serialize a GraphQL server response to JSON.
	// We use json.MarshalIndent here for easier display.
	response := server.Execute(context.Background(), request)
	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(responseJSON))
	// Output:
	// {
	//   "data": {
	//     "genericGreeting": "Hiya!"
	//   }
	// }
}

func ExampleValidatedQuery() {
	server := newServer()

	// You can use your server's schema to validate a query.
	query, errs := server.Schema().Validate(`{ genericGreeting }`)
	if len(errs) > 0 {
		log.Fatal(errs)
	}

	// You can pass the query to the server and it will execute it directly.
	response := server.Execute(context.Background(), graphql.Request{
		ValidatedQuery: query,
	})
	if len(response.Errors) > 0 {
		log.Fatal(response.Errors)
	}
	fmt.Println(response.Data.ValueFor("genericGreeting").Scalar())
	// Output:
	// Hiya!
}

func newServer() *graphql.Server {
	schema, err := graphql.ParseSchema(`
		type Query {
			genericGreeting: String!
			greet(subject: String!): String!
		}
	`, nil)
	if err != nil {
		panic(err)
	}
	queryObject := &Query{GenericGreeting: "Hiya!"}
	server, err := graphql.NewServer(schema, queryObject, nil)
	if err != nil {
		panic(err)
	}
	return server
}
