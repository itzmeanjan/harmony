SHELL:=/bin/bash

graphql_init:
	pushd app/server; gqlgen init; popd

graphql_gen:
	pushd app/server; gqlgen generate; popd

build:
	go build -o harmony

run: build
	./harmony
