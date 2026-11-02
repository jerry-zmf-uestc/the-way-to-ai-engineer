# Makefile for labs/part2-backend/chapter13/hertz-kitex-real-chain.
#
# gen  : regenerate kitex_gen/ from idl/commerce.thrift.
# demo : run the bounded end-to-end demo (task 2.7).
# test : run gofmt / vet / race tests / cover (writes binaries to /tmp only).

MODULE ?= github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs
KITEX  ?= $(shell go env GOPATH)/bin/kitex
TMPBIN ?= /tmp/hertz-kitex-real-chain

.PHONY: gen test demo build clean

gen:
	$(KITEX) -module $(MODULE) -no-fast-api idl/commerce.thrift

build:
	go build -o $(TMPBIN) ./

demo: build
	$(TMPBIN)

test:
	gofmt -l .
	go vet ./...
	go test ./... -race -count=1
	go test ./... -cover -count=1

clean:
	rm -f $(TMPBIN)
