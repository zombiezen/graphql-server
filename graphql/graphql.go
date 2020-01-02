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
	query    operation
	mutation operation
}

// NewServer returns a new server that is backed by the given query object and
// optional mutation object. These must either be objects that follow the rules
// laid out in Field Resolution in the package documentation, or functions that
// return such objects. Functions may have up to two parameters: an optional
// context.Context followed by an optional *SelectionSet. The function may also
// have an error return.
//
// Top-level objects may also implement the OperationFinisher interface. See the
// interface documentation for details.
func NewServer(schema *Schema, query, mutation interface{}) (*Server, error) {
	// Check for missing or extra arguments first.
	if query == nil {
		return nil, xerrors.New("new server: query is required")
	}
	if mutation == nil && schema.mutation != nil {
		return nil, xerrors.New("new server: schema specified mutation type, but no mutation object given")
	}
	if mutation != nil && schema.mutation == nil {
		return nil, xerrors.New("new server: mutation object given, but no mutation type")
	}

	// Next check for type errors with the arguments provided.
	srv := &Server{
		schema: schema,
	}
	var err error
	srv.query, err = newOperation(schema, schema.query, query)
	if err != nil {
		return nil, xerrors.Errorf("new server: %w", err)
	}
	srv.mutation, err = newOperation(schema, schema.mutation, mutation)
	if err != nil {
		return nil, xerrors.Errorf("new server: %w", err)
	}
	return srv, nil
}

// Schema returns the schema passed to NewServer. It is safe to call from
// multiple goroutines.
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
	scope := &selectionSetScope{
		source:    req.ValidatedQuery.source,
		doc:       req.ValidatedQuery.doc,
		types:     srv.schema.types,
		variables: varValues,
	}
	data, errs := srv.resolve(ctx, scope, op)
	resp := Response{
		Data: data,
	}
	for _, err := range errs {
		resp.Errors = append(resp.Errors, toResponseError(err))
	}
	return resp
}

func (srv *Server) resolve(ctx context.Context, scope *selectionSetScope, op *gqlang.Operation) (Value, []error) {
	gt, obj, err := srv.operationFor(op.Type)
	if err != nil {
		pos := op.Start.ToPosition(scope.source)
		return Value{}, []error{&ResponseError{
			Message: err.Error(),
			Locations: []Location{{
				Line:   pos.Line,
				Column: pos.Column,
			}},
		}}
	}
	sel, errs := newSelectionSet(scope, gt, op.SelectionSet)
	if len(errs) > 0 {
		return Value{}, errs
	}
	value := obj.value
	if obj.flags&operationFunc != 0 {
		var args []reflect.Value
		if obj.flags&operationContextParam != 0 {
			args = append(args, reflect.ValueOf(ctx))
		}
		if obj.flags&operationSelectionSetParam != 0 {
			args = append(args, reflect.ValueOf(sel))
		}
		ret := value.Call(args)
		if len(ret) == 2 {
			if err, _ := ret[1].Interface().(error); err != nil {
				// Intentionally making the returned error opaque to avoid interference in
				// toResponseError.
				return Value{}, []error{xerrors.Errorf("server error: %v", err)}
			}
		}
		value = ret[0]
	}
	result, resultErrs := srv.schema.valueFromGo(ctx, scope.variables, value, gt, sel)
	if finisher, ok := interfaceValueForAssertions(value).(OperationFinisher); ok {
		err := finisher.FinishOperation(ctx, &OperationDetails{
			SelectionSet: sel,
			HasErrors: len(resultErrs) > 0,
		})
		if err != nil {
			// Intentionally making the returned error opaque to avoid interference in
			// toResponseError.
			resultErrs = append(resultErrs, xerrors.Errorf("server error: finish request: %v", err))
		}
	}
	return result, resultErrs
}

func (srv *Server) operationFor(opType gqlang.OperationType) (*gqlType, operation, error) {
	switch opType {
	case gqlang.Query:
		return srv.schema.query, srv.query, nil
	case gqlang.Mutation:
		if !srv.mutation.value.IsValid() {
			return nil, operation{}, xerrors.New("unsupported operation type")
		}
		return srv.schema.mutation, srv.mutation, nil
	default:
		return nil, operation{}, xerrors.New("unsupported operation type")
	}
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

// If a top-level object (like query or mutation) implements OperationFinisher,
// then its FinishOperation method will be called after all its fields are
// resolved. The details struct must not be modified or retained past the end
// of the call to FinishOperation. If an error is returned, then it will be
// added to the response's errors, but the data will still be returned to the
// client.
//
// If the top-level object is shared between requests, then FinishOperation must
// be safe to call concurrently from multiple goroutines.
type OperationFinisher interface {
	FinishOperation(ctx context.Context, details *OperationDetails) error
}

// OperationDetails holds information about a nearly completed request.
type OperationDetails struct {
	// SelectionSet is the top-level selection set.
	SelectionSet *SelectionSet

	// HasErrors will be true if the operation will return at least one error.
	HasErrors bool
}

type operation struct {
	value reflect.Value
	flags operationFlags
}

type operationFlags uint8

const (
	operationFunc operationFlags = 1 << iota
	operationContextParam
	operationSelectionSetParam
)

func newOperation(schema *Schema, gt *gqlType, v interface{}) (operation, error) {
	if v == nil {
		return operation{}, nil
	}
	value := reflect.ValueOf(v)
	if _, ok := interfaceValueForAssertions(value).(FieldResolver); ok {
		return operation{value: value}, nil
	}
	if value.Kind() == reflect.Func {
		return newOperationFunc(schema, gt, value)
	}
	typ := value.Type()
	err := schema.typeDescriptor(typeKey{
		goType:  typ,
		gqlType: gt.obj,
	}).err
	if err != nil {
		return operation{}, xerrors.Errorf("cannot use %v to provide %v: %w", typ, gt, err)
	}
	return operation{value: value}, nil
}

func newOperationFunc(schema *Schema, gt *gqlType, value reflect.Value) (operation, error) {
	flags := operationFunc
	typ := value.Type()
	numIn := typ.NumIn()
	argIdx := 0
	if argIdx < numIn && typ.In(argIdx) == contextGoType {
		flags |= operationContextParam
		argIdx++
	}
	if argIdx < numIn && typ.In(argIdx) == selectionSetGoType {
		flags |= operationSelectionSetParam
		argIdx++
	}
	if argIdx < numIn {
		return operation{}, xerrors.Errorf("cannot use %v to provide %v: incorrect parameters", typ, gt)
	}
	switch typ.NumOut() {
	case 1:
		if typ.Out(0) == errorGoType {
			return operation{}, xerrors.Errorf("cannot use %v to provide %v: first return value must be non-error", typ, gt)
		}
	case 2:
		if typ.Out(0) == errorGoType {
			return operation{}, xerrors.Errorf("cannot use %v to provide %v: first return value must be non-error", typ, gt)
		}
		if typ.Out(1) != errorGoType {
			return operation{}, xerrors.Errorf("cannot use %v to provide %v: second return value must be error", typ, gt)
		}
	default:
		return operation{}, xerrors.Errorf("cannot use %v to provide %v: must have 1-2 return values", typ, gt)
	}
	err := schema.typeDescriptor(typeKey{
		goType:  typ.Out(0),
		gqlType: gt.obj,
	}).err
	if err != nil {
		return operation{}, xerrors.Errorf("cannot use %v to provide %v: %w", typ, gt, err)
	}
	return operation{
		value: value,
		flags: flags,
	}, nil
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
