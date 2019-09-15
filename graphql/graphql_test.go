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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestExecute(t *testing.T) {
	schema, err := ParseSchema(`
		type Query {
			user: User!
		}
		
		type User {
			name: String!
		}`)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(schema, Query{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	resp := srv.Execute(ctx, Request{
		Query: `{ user { name } }`,
	})
	if len(resp.Errors) > 0 {
		t.Fatal(resp.Errors)
	}
	got := resp.Data.GoValue()
	want := map[string]interface{}{
		"user": map[string]interface{}{
			"name": "Jane Doe",
		},
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("Execute result (-want +got):\n%s", diff)
	}
}

type User struct {
	Name string
}

type Query struct{}

func (Query) User(ctx context.Context, args map[string]Value, sel *SelectionSet) (User, error) {
	return User{Name: "Jane Doe"}, nil
}
