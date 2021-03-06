SHELL:=/bin/bash

graphql_gen:
	pushd app; gqlgen generate; popd

build:
	go build -o harmony

run: build
	./harmony
