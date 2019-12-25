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
	"fmt"
	"log"

	"zombiezen.com/go/graphql-server/graphql"
)

// Query is the GraphQL object read from the server.
type Query struct {
	// GenericGreeting is a no-arguments field that is read directly.
	GenericGreeting string
}

// Greet is a field that takes arguments.
func (q *Query) Greet(args *GreetArgs) (string, error) {
	message := fmt.Sprintf("Hello, %s!", args.Subject)
	return message, nil
}

// GreetArgs are arguments passed to the Query.greet field. The arguments are
// validated through GraphQL's type system and converted into this struct before
// the Greet method is called.
type GreetArgs struct {
	Subject string
}

func Example() {
	// Parse the GraphQL schema to establish type information. The schema
	// is usually a string constant in your Go server or loaded from your
	// server's filesystem.
	schema, err := graphql.ParseSchema(`
		type Query {
			genericGreeting: String!
			greet(subject: String!): String!
		}
	`, nil)
	if err != nil {
		log.Fatal(err)
	}

	// A *graphql.Server binds a schema to a Go value. The structure of
	// the Go type should reflect the GraphQL query type.
	queryObject := &Query{GenericGreeting: "Hiya!"}
	server, err := graphql.NewServer(schema, queryObject, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Once created, a *graphql.Server can execute requests.
	response := server.Execute(context.Background(), graphql.Request{
		Query: `
			query($subject: String!) {
				genericGreeting
				greet(subject: $subject)
			}
		`,
		Variables: map[string]graphql.Input{
			"subject": graphql.ScalarInput("World"),
		},
	})

	// GraphQL responses can be serialized however you want. Typically,
	// you would use JSON, but this example displays the results directly.
	if len(response.Errors) > 0 {
		log.Fatal(response.Errors)
	}
	fmt.Println(response.Data.ValueFor("genericGreeting").Scalar())
	fmt.Println(response.Data.ValueFor("greet").Scalar())
	// Output:
	// Hiya!
	// Hello, World!
}
