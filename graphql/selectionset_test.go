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
	"context"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSelectionSet_HasAny(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		request    Request
		fieldNames []string
		want       bool
	}{
		{
			name: "Present",
			request: Request{
				Query: `{ object { foo }}`,
			},
			fieldNames: []string{"foo"},
			want:       true,
		},
		{
			name: "PartialPresent",
			request: Request{
				Query: `{ object { foo }}`,
			},
			fieldNames: []string{"foo", "bar"},
			want:       true,
		},
		{
			name: "Absent",
			request: Request{
				Query: `{ object { foo }}`,
			},
			fieldNames: []string{"bar"},
			want:       false,
		},
		{
			name: "EmptyName",
			request: Request{
				Query: `{ object { foo }}`,
			},
			fieldNames: []string{""},
			want:       false,
		},
		{
			name: "Empty",
			request: Request{
				Query: `{ object { foo }}`,
			},
			fieldNames: []string{},
			want:       false,
		},
		{
			name: "ThroughFragment",
			request: Request{
				Query: `
				{ object {
					... frag
				}}

				fragment frag on Object {
					foo
				}
				`,
			},
			fieldNames: []string{"foo"},
			want:       true,
		},
		{
			name: "Typename",
			request: Request{
				Query: `{ object { __typename }}`,
			},
			fieldNames: []string{"__typename"},
			want:       true,
		},
		{
			name: "Dotted/Present",
			request: Request{
				Query: `{ object { baz { quux } }}`,
			},
			fieldNames: []string{"baz.quux"},
			want:       true,
		},
		{
			name: "Dotted/OuterAbsent",
			request: Request{
				Query: `{ object { foo }}`,
			},
			fieldNames: []string{"baz.quux"},
			want:       false,
		},
		{
			name: "Dotted/InnerAbsent",
			request: Request{
				Query: `{ object { baz { snafu } }}`,
			},
			fieldNames: []string{"baz.quux"},
			want:       false,
		},
	}
	schema, err := ParseSchema(selectionSetTestSchema, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := new(selectionSetQuery)
			srv, err := NewServer(schema, q, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp := srv.Execute(context.Background(), test.request)
			if len(resp.Errors) > 0 {
				t.Fatal(resp.Errors)
			}
			got := q.readSet().HasAny(test.fieldNames...)
			if got != test.want {
				t.Errorf("HasAny(%q) = %t; want %t. Query:\n%s", test.fieldNames, got, test.want, test.request.Query)
			}
		})
		if len(test.fieldNames) == 1 {
			t.Run("Has/"+test.name, func(t *testing.T) {
				q := new(selectionSetQuery)
				srv, err := NewServer(schema, q, nil)
				if err != nil {
					t.Fatal(err)
				}
				resp := srv.Execute(context.Background(), test.request)
				if len(resp.Errors) > 0 {
					t.Fatal(resp.Errors)
				}
				got := q.readSet().Has(test.fieldNames[0])
				if got != test.want {
					t.Errorf("Has(%q) = %t; want %t. Query:\n%s", test.fieldNames[0], got, test.want, test.request.Query)
				}
			})
		}
	}
}

func TestSelectionSet_OnlyUses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		request Request
		fields  []string
		want    bool
	}{
		{
			name: "EmptySet",
			request: Request{
				Query: `{ object { foo }}`,
			},
			fields: nil,
			want:   false,
		},
		{
			name: "SameSet",
			request: Request{
				Query: `{ object { foo }}`,
			},
			fields: []string{"foo"},
			want:   true,
		},
		{
			name: "DistinctSet",
			request: Request{
				Query: `{ object { foo }}`,
			},
			fields: []string{"bar"},
			want:   false,
		},
		{
			name: "Intersection",
			request: Request{
				Query: `{ object { foo, bar }}`,
			},
			fields: []string{"foo", "baz"},
			want:   false,
		},
		{
			name: "Superset",
			request: Request{
				Query: `{ object { foo }}`,
			},
			fields: []string{"foo", "bar"},
			want:   true,
		},
		{
			name: "IgnoresTypename",
			request: Request{
				Query: `{ object { __typename, foo }}`,
			},
			fields: []string{"foo"},
			want:   true,
		},
		{
			name: "Composite",
			request: Request{
				Query: `{ object { baz { quux } }}`,
			},
			fields: []string{"baz"},
			want:   true,
		},
		{
			name: "Dotted/SameSet",
			request: Request{
				Query: `{ object { baz { quux } }}`,
			},
			fields: []string{"baz.quux"},
			want:   true,
		},
		{
			name: "Dotted/OuterAbsent",
			request: Request{
				Query: `{ object { foo }}`,
			},
			fields: []string{"foo", "baz.quux"},
			want:   true,
		},
		{
			name: "Dotted/InnerAbsent",
			request: Request{
				Query: `{ object { baz { snafu } }}`,
			},
			fields: []string{"baz.quux", "baz.snafu"},
			want:   true,
		},
		{
			name: "Dotted/InnerDistinct",
			request: Request{
				Query: `{ object { baz { snafu } }}`,
			},
			fields: []string{"baz.quux"},
			want:   false,
		},
		{
			name: "Dotted/WithParent",
			request: Request{
				Query: `{ object { baz { snafu } }}`,
			},
			fields: []string{"baz", "baz.quux"},
			want:   true,
		},
	}
	schema, err := ParseSchema(selectionSetTestSchema, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := new(selectionSetQuery)
			srv, err := NewServer(schema, q, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp := srv.Execute(context.Background(), test.request)
			if len(resp.Errors) > 0 {
				t.Fatal(resp.Errors)
			}
			got := q.readSet().OnlyUses(test.fields...)
			if got != test.want {
				t.Errorf("OnlyUses(%q) = %t; want %t. Query:\n%s", test.fields, got, test.want, test.request.Query)
			}
		})
	}
}

type selectionSetQuery struct {
	mu  sync.Mutex
	set *SelectionSet
}

func (q *selectionSetQuery) Object(set *SelectionSet) *selectionSetQueryObject {
	// TODO(maybe): This retains the selection set past the end of the resolution.
	// We might want to forbid this later.
	q.mu.Lock()
	q.set = set
	q.mu.Unlock()
	return new(selectionSetQueryObject)
}

func (q *selectionSetQuery) readSet() *SelectionSet {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.set
}

type selectionSetQueryObject struct {
	Foo string
	Bar string
	Baz struct {
		Quux  string
		Snafu bool
	}
}

const selectionSetTestSchema = `
type Query {
	object: Object!
}

type Object {
	foo: String!
	bar: String!
	baz: Baz!
}

type Baz {
	quux: String!
	snafu: Boolean!
}
`

func TestNewFieldTree(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		names []string
		want  fieldTree
	}{
		{
			name:  "Empty",
			names: []string{},
			want:  fieldTree{},
		},
		{
			name:  "SinglePart",
			names: []string{"foo"},
			want: fieldTree{
				"foo": {selected: true},
			},
		},
		{
			name:  "TwoSingleParts",
			names: []string{"foo", "bar"},
			want: fieldTree{
				"foo": {selected: true},
				"bar": {selected: true},
			},
		},
		{
			name:  "TwoParts",
			names: []string{"foo.bar"},
			want: fieldTree{
				"foo": {
					subtree: fieldTree{
						"bar": {selected: true},
					},
				},
			},
		},
		{
			name:  "TwoPartsWithParent",
			names: []string{"foo.bar", "foo"},
			want: fieldTree{
				"foo": {
					selected: true,
					subtree: fieldTree{
						"bar": {selected: true},
					},
				},
			},
		},
		{
			name:  "ExtraDots",
			names: []string{".foo..bar."},
			want: fieldTree{
				"foo": {
					subtree: fieldTree{
						"bar": {selected: true},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := newFieldTree(test.names)
			if diff := cmp.Diff(test.want, got, cmp.AllowUnexported(fieldTreeNode{})); diff != "" {
				t.Errorf("newFieldTree(%#v) (-want +got):\n%s", test.names, diff)
			}
		})
	}
}
