# @zombiezen's GraphQL Go Server

[![GoDoc](https://godoc.org/zombiezen.com/go/graphql-server/graphql?status.svg)](https://godoc.org/zombiezen.com/go/graphql-server/graphql)

This repository contains Go packages for creating a [GraphQL][] server. The
primary focus is on simplicity and efficiency of the server endpoints.

**This library is pre-1.0, so there may still be API changes.** You can find
known issues about [compliance to the specification][spec-compliance] or
[production-readiness][productionization] in the issue tracker.

[GraphQL]: https://graphql.org/
[productionization]: https://github.com/zombiezen/graphql-server/labels/productionization
[spec-compliance]: https://github.com/zombiezen/graphql-server/labels/spec-compliance

## Getting Started

The easiest way to get started with this library is to follow the directions in
the [graphql-go-app README][] to create a project. This will set you up with a
GraphQL server with a [Single-Page Application][] using [TypeScript][] and
[React][].

If you want to integrate the library into an existing project, then run:

```
go get zombiezen.com/go/graphql-server/graphql
```

Then, look at the [main package example][] for how to write a server type and
the [`graphqlhttp` package example][] for how to start serving over HTTP.

[graphql-go-app README]: https://github.com/zombiezen/graphql-go-app/blob/master/README.md#getting-started
[`graphqlhttp` package example]: https://godoc.org/zombiezen.com/go/graphql-server/graphqlhttp#example-package
[main package example]: https://godoc.org/zombiezen.com/go/graphql-server/graphql#example-package
[React]: https://reactjs.org/
[Single-Page Application]: https://en.wikipedia.org/wiki/Single-page_application
[TypeScript]: https://www.typescriptlang.org/

## Comparison With Other Libraries

A quick look at the [official GraphQL libraries for Go][] may leave you
wondering, "why another server library?" Simply, @zombiezen hit roadblocks to
writing apps with other libraries and wanted to try a different API approach.

This library intentionally focuses on:

-  using the [GraphQL interface definition language (IDL)][GraphQL IDL] to
   define types
-  allowing resolver functions to avoid unnecessary work by allowing inspection
   of their [selection set][]
-  simplifying testing by exposing a rich [value API][]
-  permitting any serialization format, but [supporting JSON over
   HTTP][graphqlhttp] out-of-the-box

[GraphQL IDL]: https://graphql.org/learn/schema/
[graphqlhttp]: https://godoc.org/zombiezen.com/go/graphql-server/graphqlhttp
[official GraphQL libraries for Go]: https://graphql.org/code/#go
[selection set]: https://godoc.org/zombiezen.com/go/graphql-server/graphql#SelectionSet
[value API]: https://godoc.org/zombiezen.com/go/graphql-server/graphql#Value

### `github.com/graphql-go/graphql`

[`github.com/graphql-go/graphql`][] follows the [`graphql-js`][] reference
implementation, basically replicating its API verbatim. Unfortunately, this
leads to a fairly verbose approach to defining the server's schema in code
rather than the GraphQL IDL. Further, while you can do limited look-ahead of
output selection sets via [`ResolveInfo`][], this only returns the ASTs and
requires the caller to interpret the fragments themselves
([graphql-go/graphql#157][]). The library also doesn't provide any
out-of-the-box utilities for serving over HTTP.

[`github.com/graphql-go/graphql`]: https://github.com/graphql-go/graphql
[graphql-go/graphql#157]: https://github.com/graphql-go/graphql/issues/157
[`graphql-js`]: https://github.com/graphql/graphql-js
[`ResolveInfo`]: https://godoc.org/github.com/graphql-go/graphql#ResolveInfo

### `github.com/graph-gophers/graphql-go`

@zombiezen very much liked the approach that
[`github.com/graph-gophers/graphql-go`][] took toward the problem: it uses the
IDL and the Go type system rather than a large package-specific data structure.
This results in a small ramp-up time and application code that is fairly
straightforward to follow.

However, the library has a few issues. It does not support look-ahead at all
([graph-gophers/graphql-go#17][]). The API conflates schema parsing with server
object binding (i.e. [`graphql.ParseSchema`][] takes a resolver), so many
servers end up passing dependencies through the `Context`. The library makes it
difficult to test servers that use it, since its responses are always
JSON-formatted, which makes it hard to compare specific fields. While JSON is a
common serialization format used with GraphQL, the spec permits any
serialization.

[`github.com/graph-gophers/graphql-go`]: https://github.com/graph-gophers/graphql-go
[graph-gophers/graphql-go#17]: https://github.com/graph-gophers/graphql-go/issues/17
[`graphql.ParseSchema`]: https://godoc.org/github.com/graph-gophers/graphql-go#ParseSchema

### `github.com/99designs/gqlgen`

[gqlgen][] is another common GraphQL solution for Go. Its primary selling point
is that rather than using reflection, it generates Go code based on a GraphQL
schema and a YAML configuration file.

While this seems productive and helpful, @zombiezen has a [great][zombiezen protobuf]
[deal][zombiezen wire] of [experience][go-capnproto2] with code generators and
knows their strengths and limitations. Notably, code generators are difficult to
reason about when generated code is mixed with user-written code. Names can
conflict and special configuration directives are required. gqlgen is no
exception, and while gqlgen may be productive for smaller projects, code
generation provides unacceptable complexity for larger projects.

Of all the other Go libraries for GraphQL @zombiezen has surveyed, gqlgen is
definitely the most [feature-complete][gqlgen features], but @zombiezen found
these features hard to use or find due to the large API surface area dedicated
to supporting the generated code. Further, critical features like
[examining the selection set][gqlgen field collection] are limited to a single
level of depth, whereas this library permits checking arbitrary depth.

In practice, the code someone would write for this library is largely similar to
`gqlgen`, but with far less glue and build complexity. By performing up-front
type structure checks, this library is able to get many of the same guarantees
without nearly as much intrusion on Go developer workflow.

[go-capnproto2]: https://github.com/capnproto/go-capnproto2
[gqlgen]: https://gqlgen.com/
[gqlgen features]: https://gqlgen.com/feature-comparison/
[gqlgen field collection]: https://gqlgen.com/reference/field-collection/
[zombiezen protobuf]: https://github.com/golang/protobuf/commits?author=light@google.com
[zombiezen wire]: https://github.com/google/wire/commits?author=zombiezen

## License

[Apache 2.0](https://github.com/zombiezen/graphql-server/blob/master/LICENSE)
