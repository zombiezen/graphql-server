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

package graphqlhttp_test

import (
	"log"
	"net/http"

	"zombiezen.com/go/graphql-server/graphql"
	"zombiezen.com/go/graphql-server/graphqlhttp"
)

// Query is the GraphQL object read from the server.
type Query struct {
	Greeting string
}

func Example() {
	// Set up the server.
	schema, err := graphql.ParseSchema(`
		type Query {
			greeting: String!
		}
	`)
	if err != nil {
		log.Fatal(err)
	}
	queryObject := &Query{Greeting: "Hello, World!"}
	server, err := graphql.NewServer(schema, queryObject, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Serve over HTTP using NewHandler.
	http.Handle("/graphql", graphqlhttp.NewHandler(server))
	http.ListenAndServe(":8080", nil)
}