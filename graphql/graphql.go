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
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"golang.org/x/xerrors"
	"zombiezen.com/go/graphql-server/internal/gqlang"
)

// Server manages execution of GraphQL operations.
type Server struct {
	schema   *Schema
	query    reflect.Value
	mutation reflect.Value
}

// NewServer returns a new server that is backed by the given query and
// mutation objects. The mutation object is optional.
func NewServer(schema *Schema, query, mutation interface{}) (*Server, error) {
	srv := &Server{
		schema:   schema,
		query:    reflect.ValueOf(query),
		mutation: reflect.ValueOf(mutation),
	}
	// Check for missing or extra arguments first.
	if !srv.query.IsValid() {
		return nil, xerrors.New("new server: query is required")
	}
	if schema.mutation != nil && !srv.mutation.IsValid() {
		return nil, xerrors.New("new server: schema specified mutation type, but no mutation object given")
	}
	if schema.mutation == nil && srv.mutation.IsValid() {
		return nil, xerrors.New("new server: mutation object given, but no mutation type")
	}
	// Next check for type errors with the values provided.
	err := schema.typeDescriptor(typeKey{
		goType:  srv.query.Type(),
		gqlType: schema.query.obj,
	}).err
	if err != nil {
		return nil, xerrors.Errorf("new server: can't use %v for query: %w", srv.query.Type(), err)
	}
	if schema.mutation != nil {
		err := schema.typeDescriptor(typeKey{
			goType:  srv.mutation.Type(),
			gqlType: schema.mutation.obj,
		}).err
		if err != nil {
			return nil, xerrors.Errorf("new server: can't use %v for mutation: %w", srv.mutation.Type(), err)
		}
	}
	return srv, nil
}

// Schema returns the schema passed to NewServer.
func (srv *Server) Schema() *Schema {
	return srv.schema
}

// Execute runs a single GraphQL operation. It is safe to call Execute from
// multiple goroutines.
func (srv *Server) Execute(ctx context.Context, req Request) Response {
	query := req.ValidatedQuery
	if query == nil {
		var errs []*ResponseError
		query, errs = srv.schema.Validate(req.Query)
		if len(errs) > 0 {
			return Response{Errors: errs}
		}
	} else if query.schema != srv.schema {
		return Response{
			Errors: []*ResponseError{
				{Message: "query validated with a schema different from the server"},
			},
		}
	}
	return srv.executeValidated(ctx, Request{
		ValidatedQuery: query,
		OperationName:  req.OperationName,
		Variables:      req.Variables,
	})
}

func (srv *Server) executeValidated(ctx context.Context, req Request) Response {
	op := req.ValidatedQuery.doc.FindOperation(req.OperationName)
	if op == nil {
		if req.OperationName == "" {
			return Response{
				Errors: []*ResponseError{
					{Message: "multiple operations; must specify operation name"},
				},
			}
		}
		return Response{
			Errors: []*ResponseError{
				{Message: fmt.Sprintf("no such operation %q", req.OperationName)},
			},
		}
	}
	varValues, errs := coerceVariableValues(req.ValidatedQuery.source, srv.schema.types, req.Variables, op.VariableDefinitions)
	if len(errs) > 0 {
		resp := Response{}
		for _, err := range errs {
			resp.Errors = append(resp.Errors, toResponseError(err))
		}
		return resp
	}
	var data Value
	switch op.Type {
	case gqlang.Query:
		var sel *SelectionSet
		sel, errs = newSelectionSet(req.ValidatedQuery.source, varValues, srv.schema.query.obj, op.SelectionSet)
		if len(errs) == 0 {
			data, errs = srv.schema.valueFromGo(ctx, varValues, srv.query, srv.schema.query, sel)
		}
	case gqlang.Mutation:
		if !srv.mutation.IsValid() {
			pos := op.Start.ToPosition(req.ValidatedQuery.source)
			return Response{
				Errors: []*ResponseError{{
					Message: "unsupported operation type",
					Locations: []Location{{
						Line:   pos.Line,
						Column: pos.Column,
					}},
				}},
			}
		}
		var sel *SelectionSet
		sel, errs = newSelectionSet(req.ValidatedQuery.source, varValues, srv.schema.mutation.obj, op.SelectionSet)
		if len(errs) == 0 {
			data, errs = srv.schema.valueFromGo(ctx, varValues, srv.mutation, srv.schema.mutation, sel)
		}
	default:
		pos := op.Start.ToPosition(req.ValidatedQuery.source)
		return Response{
			Errors: []*ResponseError{{
				Message: "unsupported operation type",
				Locations: []Location{{
					Line:   pos.Line,
					Column: pos.Column,
				}},
			}},
		}
	}
	resp := Response{
		Data: data,
	}
	for _, err := range errs {
		resp.Errors = append(resp.Errors, toResponseError(err))
	}
	return resp
}

// ValidatedQuery is a query that has been parsed and type-checked.
type ValidatedQuery struct {
	schema *Schema
	source string
	doc    *gqlang.Document
}

// TypeOf returns the type of the operation with the given name or zero if no
// such operation exists. If the operation name is empty and there is only one
// operation in the query, then TypeOf returns the type of that operation.
func (query *ValidatedQuery) TypeOf(operationName string) OperationType {
	op := query.doc.FindOperation(operationName)
	if op == nil {
		return 0
	}
	return operationTypeFromAST(op.Type)
}

// OperationType represents the keywords used to declare operations.
type OperationType int

// Types of operations.
const (
	QueryOperation OperationType = 1 + iota
	MutationOperation
	SubscriptionOperation
)

func operationTypeFromAST(typ gqlang.OperationType) OperationType {
	switch typ {
	case gqlang.Query:
		return QueryOperation
	case gqlang.Mutation:
		return MutationOperation
	case gqlang.Subscription:
		return SubscriptionOperation
	default:
		panic("unknown operation type")
	}
}

// String returns the keyword corresponding to the operation type.
func (typ OperationType) String() string {
	switch typ {
	case QueryOperation:
		return "query"
	case MutationOperation:
		return "mutation"
	case SubscriptionOperation:
		return "subscription"
	default:
		return fmt.Sprintf("OperationType(%d)", int(typ))
	}
}

// Request holds the inputs for a GraphQL execution.
type Request struct {
	// Query is the GraphQL document text.
	Query string `json:"query"`
	// If ValidatedQuery is not nil, then it will be used instead of the Query field.
	ValidatedQuery *ValidatedQuery `json:"-"`
	// If OperationName is not empty, then the operation with the given name will
	// be executed. Otherwise, the query must only include a single operation.
	OperationName string `json:"operationName,omitempty"`
	// Variables specifies the values of the operation's variables.
	Variables map[string]Input `json:"variables,omitempty"`
}

// Response holds the output of a GraphQL operation.
type Response struct {
	Data   Value            `json:"data"`
	Errors []*ResponseError `json:"errors,omitempty"`
}

// MarshalJSON converts the response to JSON format.
func (resp Response) MarshalJSON() ([]byte, error) {
	var buf []byte
	buf = append(buf, '{')
	if len(resp.Errors) > 0 {
		buf = append(buf, `"errors":`...)
		errorsData, err := json.Marshal(resp.Errors)
		if err != nil {
			return buf, xerrors.Errorf("marshal response: %w", err)
		}
		buf = append(buf, errorsData...)
		if !resp.Data.IsNull() {
			buf = append(buf, ',')
		}
	}
	if !resp.Data.IsNull() {
		buf = append(buf, `"data":`...)
		data, err := json.Marshal(resp.Data)
		if err != nil {
			return buf, xerrors.Errorf("marshal response: %w", err)
		}
		buf = append(buf, data...)
	}
	buf = append(buf, '}')
	return buf, nil
}

// ResponseError describes an error that occurred during the processing of a
// GraphQL operation.
type ResponseError struct {
	Message   string        `json:"message"`
	Locations []Location    `json:"locations,omitempty"`
	Path      []PathSegment `json:"path,omitempty"`
}

// Error returns e.Message.
func (e *ResponseError) Error() string {
	return e.Message
}

func toResponseError(e error) *ResponseError {
	re, ok := e.(*ResponseError)
	if ok {
		// e is a *ResponseError.
		return re
	}
	// Build a new response error.
	re = &ResponseError{
		Message: e.Error(),
	}
	unknownChain := e
	for ; e != nil; e = xerrors.Unwrap(e) {
		// TODO(someday): Call As in addition to type assignment.
		switch e := e.(type) {
		case *ResponseError:
			re.Locations = append(re.Locations, e.Locations...)
			re.Path = append(re.Path, e.Path...)
			unknownChain = nil // leaf
		case *fieldError:
			re.Path = append(re.Path, PathSegment{Field: e.key})
			re.Locations = append(re.Locations, e.locs...)
			unknownChain = e.Unwrap()
		case *listElementError:
			re.Path = append(re.Path, PathSegment{ListIndex: e.idx})
			unknownChain = e.Unwrap()
		}
	}
	if pos, ok := gqlang.ErrorPosition(unknownChain); ok {
		re.Locations = []Location{astPositionToLocation(pos)}
	}
	return re
}

func hasLocation(e error) bool {
	var re *ResponseError
	if xerrors.As(e, &re) && len(re.Locations) > 0 {
		return true
	}
	var fe *fieldError
	if xerrors.As(e, &fe) {
		return true
	}
	_, ok := gqlang.ErrorPos(e)
	return ok
}

type fieldError struct {
	key  string
	locs []Location
	err  error
}

func wrapFieldError(key string, loc Location, err error) error {
	if key == "" {
		panic("empty key")
	}
	if loc.Line < 1 || loc.Column < 1 {
		panic("invalid location")
	}
	var locs []Location
	if !hasLocation(err) {
		locs = []Location{loc}
	}
	return &fieldError{
		key:  key,
		locs: locs,
		err:  err,
	}
}

func (e *fieldError) Error() string {
	return fmt.Sprintf("field %s: %v", e.key, e.err)
}

func (e *fieldError) Unwrap() error {
	return e.err
}

type listElementError struct {
	idx int
	err error
}

func (e *listElementError) Error() string {
	return fmt.Sprintf("list[%d]: %v", e.idx, e.err)
}

func (e *listElementError) Unwrap() error {
	return e.err
}

// Location identifies a position in a GraphQL document. Line and column
// are 1-based.
type Location struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

func astPositionToLocation(pos gqlang.Position) Location {
	return Location{
		Line:   pos.Line,
		Column: pos.Column,
	}
}

// String returns the location in the form "line:col".
func (loc Location) String() string {
	return fmt.Sprintf("%d:%d", loc.Line, loc.Column)
}

// PathSegment identifies a field or array index in an output object.
type PathSegment struct {
	Field     string
	ListIndex int
}

// String returns the segment's index or field name as a string.
func (seg PathSegment) String() string {
	if seg.Field == "" {
		return strconv.Itoa(seg.ListIndex)
	}
	return seg.Field
}

// MarshalJSON converts the segment to a JSON integer or a JSON string.
func (seg PathSegment) MarshalJSON() ([]byte, error) {
	if seg.Field == "" {
		return strconv.AppendInt(nil, int64(seg.ListIndex), 10), nil
	}
	return json.Marshal(seg.Field)
}

// UnmarshalJSON converts JSON strings into field segments and JSON numbers into
// list index segments.
func (seg *PathSegment) UnmarshalJSON(data []byte) error {
	if !bytes.HasPrefix(data, []byte(`"`)) {
		i, err := json.Number(string(data)).Int64()
		if err != nil {
			return err
		}
		seg.ListIndex = int(i)
		return nil
	}
	err := json.Unmarshal(data, &seg.Field)
	return err
}
