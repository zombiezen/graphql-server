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

// Package graphql provides a GraphQL server type.
package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/xerrors"
	"zombiezen.com/go/graphql-server/internal/gqlang"
)

// Schema is a parsed set of type definitions.
type Schema struct {
	query    *gqlType
	mutation *gqlType
}

// ParseSchema parses a GraphQL document containing type definitions.
func ParseSchema(input string) (*Schema, error) {
	doc, errs := gqlang.Parse(input)
	if len(errs) > 0 {
		msgBuilder := new(strings.Builder)
		msgBuilder.WriteString("parse schema:")
		for _, err := range errs {
			msgBuilder.WriteByte('\n')
			if p, ok := gqlang.ErrorPosition(err); ok {
				msgBuilder.WriteString(p.String())
				msgBuilder.WriteString(": ")
			}
			msgBuilder.WriteString(err.Error())
		}
		return nil, xerrors.New(msgBuilder.String())
	}
	typeMap := make(map[string]*gqlType)
	builtins := []*gqlType{
		booleanType,
		floatType,
		intType,
		stringType,
		idType,
	}
	for _, b := range builtins {
		typeMap[b.String()] = b
	}
	// First pass: fill out lookup table.
	for _, defn := range doc.Definitions {
		t := defn.Type
		switch {
		case t == nil:
			continue
		case t.Scalar != nil:
			typeMap[t.Scalar.Name.Value] = newScalarType(t.Scalar.Name.Value)
		case t.Object != nil:
			typeMap[t.Object.Name.Value] = newObjectType(&objectType{
				name:   t.Object.Name.Value,
				fields: make(map[string]*gqlType),
			})
		}
	}
	// Second pass: fill in object definitions.
	for _, defn := range doc.Definitions {
		if defn.Type == nil || defn.Type.Object == nil {
			continue
		}
		obj := defn.Type.Object
		info := typeMap[obj.Name.Value].obj
		for _, fieldDefn := range obj.Fields.Defs {
			info.fields[fieldDefn.Name.Value] = resolveTypeRef(typeMap, fieldDefn.Type)
		}
	}
	schema := &Schema{
		query:    typeMap["Query"],
		mutation: typeMap["Mutation"],
	}
	if schema.query == nil {
		return nil, xerrors.New("parse schema: no query type specified")
	}
	return schema, nil
}

func resolveTypeRef(typeMap map[string]*gqlType, ref *gqlang.TypeRef) *gqlType {
	switch {
	case ref.Named != nil:
		return typeMap[ref.Named.Value]
	case ref.List != nil:
		return listOf(resolveTypeRef(typeMap, ref.List.Type))
	case ref.NonNull != nil && ref.NonNull.Named != nil:
		return typeMap[ref.NonNull.Named.Value].toNonNullable()
	case ref.NonNull != nil && ref.NonNull.List != nil:
		return listOf(resolveTypeRef(typeMap, ref.NonNull.List.Type)).toNonNullable()
	default:
		panic("unrecognized type reference form")
	}
}

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
	if !srv.query.IsValid() {
		return nil, xerrors.New("new server: query is required")
	}
	return srv, nil
}

// Execute runs a single GraphQL operation. It is safe to call Execute from
// multiple goroutines.
func (srv *Server) Execute(ctx context.Context, req Request) Response {
	doc, errs := gqlang.Parse(req.Query)
	if len(errs) > 0 {
		resp := Response{}
		for _, err := range errs {
			resp.Errors = append(resp.Errors, toResponseError(err))
		}
		return resp
	}
	errs = srv.schema.validateRequest(req.Query, doc)
	if len(errs) > 0 {
		resp := Response{}
		for _, err := range errs {
			resp.Errors = append(resp.Errors, toResponseError(err))
		}
		return resp
	}
	op := findOperation(doc, req.OperationName)
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
	var data Value
	var err error
	switch op.Type {
	case gqlang.Query:
		data, err = valueFromGo(ctx, srv.query, srv.schema.query, newSelectionSet(op.SelectionSet))
	case gqlang.Mutation:
		if !srv.mutation.IsValid() {
			pos := op.Start.ToPosition(req.Query)
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
		data, err = valueFromGo(ctx, srv.mutation, srv.schema.mutation, newSelectionSet(op.SelectionSet))
	default:
		pos := op.Start.ToPosition(req.Query)
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
	if err != nil {
		return Response{
			Errors: []*ResponseError{{
				Message: xerrors.Errorf("evaluate: %w", err).Error(),
			}},
		}
	}
	return Response{Data: data}
}

// findOperation finds the operation with the name or nil if not found.
// It assumes the document has been validated.
func findOperation(doc *gqlang.Document, operationName string) *gqlang.Operation {
	for _, defn := range doc.Definitions {
		if defn.Operation == nil {
			continue
		}
		if operationName == "" || operationName == defn.Operation.Name.Value {
			return defn.Operation
		}
	}
	return nil
}

// Request holds the inputs for a GraphQL operation.
type Request struct {
	Query         string           `json:"query"`
	OperationName string           `json:"operationName,omitempty"`
	Variables     map[string]Input `json:"variables,omitempty"`
}

// Response holds the output of a GraphQL operation.
type Response struct {
	Data   Value            `json:"data"`
	Errors []*ResponseError `json:"errors,omitempty"`
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
	if xerrors.As(e, &re) {
		// A *ResponseError is in the chain, but not the top-level.
		// Wrap the new message.
		return &ResponseError{
			Message:   e.Error(),
			Locations: re.Locations,
			Path:      re.Path,
		}
	}

	// Build a new response error.
	re = &ResponseError{
		Message: e.Error(),
	}
	if pos, ok := gqlang.ErrorPosition(e); ok {
		re.Locations = []Location{astPositionToLocation(pos)}
	}
	return re
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
