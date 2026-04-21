# https://just.systems

app := "pkgdoc2md"
pkg := "net"
out := "test.md"

default: build

fmt:
    goimports -w .

build: fmt
    go build -o {{ app }} .

run: build
    ./{{ app }} -pkg {{ pkg }} -output {{ out }} -debug
