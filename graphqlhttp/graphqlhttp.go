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

// Package graphqlhttp provides functions for serving GraphQL over HTTP as
// described in https://graphql.org/learn/serving-over-http/.
package graphqlhttp

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"strconv"

	"golang.org/x/xerrors"
	"zombiezen.com/go/graphql-server/graphql"
)

// Handler serves GraphQL HTTP requests by executing them on its server.
type Handler struct {
	server *graphql.Server
}

// NewHandler returns a new handler that sends requests to the given server.
func NewHandler(server *graphql.Server) *Handler {
	return &Handler{server: server}
}

// ServeHTTP executes a GraphQL request.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	gqlRequest, err := Parse(r)
	if err != nil {
		code := StatusCode(err)
		if code == http.StatusMethodNotAllowed {
			w.Header().Set("Allow", "GET, HEAD, POST")
		}
		http.Error(w, err.Error(), code)
	}
	gqlResponse := h.server.Execute(r.Context(), gqlRequest)
	WriteResponse(w, gqlResponse)
}

// Parse parses a GraphQL HTTP request. If an error is returned, StatusCode
// will return the proper HTTP status code to use.
//
// Request methods may be GET, HEAD, or POST. If the method is not one of these,
// then an error is returned that will make StatusCode return
// http.StatusMethodNotAllowed.
func Parse(r *http.Request) (graphql.Request, error) {
	request := graphql.Request{
		Query: r.URL.Query().Get("query"),
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		if v := r.FormValue("variables"); v != "" {
			if err := json.Unmarshal([]byte(v), &request.Variables); err != nil {
				return graphql.Request{}, &httpError{
					msg:   "parse graphql request: ",
					code:  http.StatusBadRequest,
					cause: err,
				}
			}
		}
		request.OperationName = r.FormValue("operationName")
		if !request.IsQuery() {
			return graphql.Request{}, &httpError{
				msg:  "parse graphql request: GET requests must be queries",
				code: http.StatusBadRequest,
			}
		}
	case http.MethodPost:
		rawContentType := r.Header.Get("Content-Type")
		contentType, _, err := mime.ParseMediaType(rawContentType)
		if err != nil {
			return graphql.Request{}, &httpError{
				msg:  "parse graphql request: invalid content type: " + rawContentType,
				code: http.StatusUnsupportedMediaType,
			}
		}
		switch contentType {
		case "application/json":
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				return graphql.Request{}, &httpError{
					msg:   "parse graphql request: ",
					code:  http.StatusBadRequest,
					cause: err,
				}
			}
		case "application/x-www-form-urlencoded":
			request.Query = r.FormValue("query")
		case "application/graphql":
			data, err := ioutil.ReadAll(r.Body)
			if err != nil {
				return graphql.Request{}, &httpError{
					msg:   "parse graphql request: ",
					code:  http.StatusBadRequest,
					cause: err,
				}
			}
			if len(data) > 0 {
				request.Query = string(data)
			}
		default:
			return graphql.Request{}, &httpError{
				msg:  "parse graphql request: unrecognized content type: " + contentType,
				code: http.StatusUnsupportedMediaType,
			}
		}
	default:
		return graphql.Request{}, &httpError{
			msg:  fmt.Sprintf("parse graphql request: method %s not allowed", r.Method),
			code: http.StatusMethodNotAllowed,
		}
	}
	return request, nil
}

type httpError struct {
	msg   string
	code  int
	cause error
}

func (e *httpError) Error() string {
	if e.cause == nil {
		return e.msg
	}
	return e.msg + e.cause.Error()
}

func (e *httpError) Unwrap() error {
	return e.cause
}

// StatusCode returns the HTTP status code an error indicates.
func StatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}
	var e *httpError
	if !xerrors.As(err, &e) {
		return http.StatusInternalServerError
	}
	return e.code
}

// WriteResponse writes a GraphQL result as an HTTP response.
func WriteResponse(w http.ResponseWriter, response graphql.Response) {
	payload, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "GraphQL marshal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	if _, err := w.Write(payload); err != nil {
		return
	}
}
