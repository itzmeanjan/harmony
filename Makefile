SHELL:=/bin/bash

build:
	go build -o harmony

run: build
	./harmony
