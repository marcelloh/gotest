# gotest

Like `go test` but with colors.

## What is gotest

gotest will do the same as go test, but with colors.

it will show:
- a clickable file & line number on a fail for easy fixing
- the total of fails
- the busy time

## Installation

```
$ go install github.com/marcelloh/gotest@latest
or
$ go get -u github.com/marcelloh/gotest

```

or get a specific version, like this:

```
$ go install github.com/marcelloh/gotest@v1.19.1
$ go get -u github.com/marcelloh/gotest@v1.19.0
```

## Usage

Accepts all the arguments and flags `go test` works with.

Example:

```
$ gotest -v ./...
```
## Extra

If "loop" is the last argument, it will keep running and watch file changes.
(not for new files!)

Example:

```
$ gotest ./... loop
```