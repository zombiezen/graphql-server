# @zombiezen's GraphQL Go Server

[![GoDoc](https://godoc.org/zombiezen.com/go/graphql-server/graphql?status.svg)](https://godoc.org/zombiezen.com/go/graphql-server/graphql)

This repository contains Go packages for creating a [GraphQL][] server. The
primary focus is on simplicity and efficiency of the server endpoints.

**This library is still under development and should not be used in production.**
You can see known issues about [compliance to the specification][spec-compliance]
or [production-readiness][productionization] in the issue tracker.

[GraphQL]: https://graphql.org/
[productionization]: https://github.com/zombiezen/graphql-server/labels/productionization
[spec-compliance]: https://github.com/zombiezen/graphql-server/labels/spec-compliance

## Getting Started

```
go get zombiezen.com/go/graphql-server/graphql
```

Then, look at the [main package example][] for how to write a server type and
the [`graphqlhttp` package example][] for how to start serving over HTTP.

[main package example]: https://godoc.org/zombiezen.com/go/graphql-server/graphql#example-package
[`graphqlhttp` package example]: https://godoc.org/zombiezen.com/go/graphql-server/graphqlhttp#example-package

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

## License

[Apache 2.0](https://github.com/zombiezen/graphql-server/blob/master/LICENSE)
