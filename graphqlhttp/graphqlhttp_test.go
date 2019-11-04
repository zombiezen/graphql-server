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

package graphqlhttp

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"zombiezen.com/go/graphql-server/graphql"
)

func TestParse(t *testing.T) {
	schema, err := graphql.ParseSchema(`
		type Query {
			me: User
		}

		type Mutation {
			me: User
		}

		type User {
			name: String!
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string

		method      string
		query       url.Values
		contentType string
		body        string

		want          graphql.Request
		wantErrStatus int
	}{
		{
			name:   "HEAD",
			method: http.MethodHead,
			query:  url.Values{"query": {"{me{name}}"}},
			want: graphql.Request{
				Query: "{me{name}}",
			},
		},
		{
			name:   "GET/JustQuery",
			method: http.MethodGet,
			query:  url.Values{"query": {"{me{name}}"}},
			want: graphql.Request{
				Query: "{me{name}}",
			},
		},
		{
			name:   "GET/AllFields",
			method: http.MethodGet,
			query: url.Values{
				"query":         {"query Baz{me{name}}"},
				"variables":     {`{"foo":"bar"}`},
				"operationName": {"Baz"},
			},
			want: graphql.Request{
				Query:         "query Baz{me{name}}",
				OperationName: "Baz",
				Variables: map[string]graphql.Input{
					"foo": graphql.ScalarInput("bar"),
				},
			},
		},
		{
			name:   "GET/Mutation",
			method: http.MethodGet,
			query: url.Values{
				"query":     {"mutation {me{name}}"},
				"variables": {`{"foo":"bar"}`},
			},
			wantErrStatus: http.StatusBadRequest,
		},
		{
			name:        "POST/JustQuery",
			method:      http.MethodPost,
			contentType: "application/json; charset=utf-8",
			body:        `{"query": "{me{name}}"}`,
			want: graphql.Request{
				Query: "{me{name}}",
			},
		},
		{
			name:        "POST/AllFields",
			method:      http.MethodPost,
			contentType: "application/json; charset=utf-8",
			body:        `{"query": "{me{name}}", "variables": {"foo":"bar"}, "operationName": "Baz"}`,
			want: graphql.Request{
				Query:         "{me{name}}",
				OperationName: "Baz",
				Variables: map[string]graphql.Input{
					"foo": graphql.ScalarInput("bar"),
				},
			},
		},
		{
			name:        "POST/QueryInURL",
			method:      http.MethodPost,
			query:       url.Values{"query": {"{me{name}}"}},
			contentType: "application/json; charset=utf-8",
			body:        `{"variables": {"foo":"bar"}, "operationName": "Baz"}`,
			want: graphql.Request{
				Query:         "{me{name}}",
				OperationName: "Baz",
				Variables: map[string]graphql.Input{
					"foo": graphql.ScalarInput("bar"),
				},
			},
		},
		{
			name:        "POST/QueryInBodyAndURL",
			method:      http.MethodPost,
			query:       url.Values{"query": {"{me{name}}"}},
			contentType: "application/json; charset=utf-8",
			body:        `{"query": "{your{face}}", "variables": {"foo":"bar"}, "operationName": "Baz"}`,
			want: graphql.Request{
				Query:         "{your{face}}",
				OperationName: "Baz",
				Variables: map[string]graphql.Input{
					"foo": graphql.ScalarInput("bar"),
				},
			},
		},
		{
			name:        "POST/GraphQLContentType",
			method:      http.MethodPost,
			contentType: "application/graphql; charset=utf-8",
			body:        "{me{name}}",
			want: graphql.Request{
				Query: "{me{name}}",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := &http.Request{
				Method: test.method,
				URL: &url.URL{
					RawQuery: test.query.Encode(),
				},
				Header: make(http.Header),
				Body:   ioutil.NopCloser(strings.NewReader(test.body)),
			}
			if test.contentType != "" {
				req.Header.Set("Content-Type", test.contentType)
			}
			got, err := Parse(schema, req)
			if err != nil {
				if test.wantErrStatus == 0 {
					t.Fatalf("Parse error = %v; want <nil>", err)
				}
				if StatusCode(err) != test.wantErrStatus {
					t.Fatalf("Parse error = %v, status code = %d; want status code = %d", err, StatusCode(err), test.wantErrStatus)
				}
				return
			}
			if test.wantErrStatus != 0 {
				t.Fatalf("Parse(...) = %+v, <nil>; want error status code = %d", got, test.wantErrStatus)
			}
			diff := cmp.Diff(test.want, got,
				cmp.Transformer("graphql.Input.GoValue", graphql.Input.GoValue),
				cmpopts.IgnoreFields(graphql.Request{}, "ValidatedQuery"))
			if diff != "" {
				t.Errorf("Parse(...) (-want +got):\n%s", diff)
			}
		})
	}
}
