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
	"encoding/json"
	"testing"
)

func BenchmarkExecute(b *testing.B) {
	schema, err := ParseSchema(`
		type Query {
			myString: String
		}
	`)
	if err != nil {
		b.Fatal(err)
	}
	server, err := NewServer(schema, &testQueryStruct{MyString: NullString{Valid: true, S: "Hello, World!"}}, nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()

	ctx := context.Background()
	b.Run("Simple", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			resp := server.Execute(ctx, Request{
				Query: "{ myString }",
			})
			if len(resp.Errors) > 0 {
				b.Fatal(err)
			}
		}
	})
	b.Run("ValidatedQuery", func(b *testing.B) {
		query, err := schema.Validate("{ myString }")
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resp := server.Execute(ctx, Request{
				ValidatedQuery: query,
			})
			if len(resp.Errors) > 0 {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkValidate(b *testing.B) {
	schema, err := ParseSchema(`
		type Query {
			myString: String
		}
	`)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := schema.Validate("{ myString }")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalRequestJSON(b *testing.B) {
	const source = `{"query": "query Foo { myString }", "operationName": "Foo"}`
	data := []byte(source)
	b.SetBytes(int64(len(source)))
	b.ResetTimer()
	var req Request
	for i := 0; i < b.N; i++ {
		json.Unmarshal(data, &req)
	}
}

func BenchmarkMarshalResponseJSON(b *testing.B) {
	val := testObjectValue()
	b.ResetTimer()
	data, err := json.Marshal(val)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N-1; i++ {
		_, err := json.Marshal(val)
		if err != nil {
			b.Fatal(err)
		}
	}
}
