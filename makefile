MAKEFLAGS += --silent
all=build vet fmt fmtcheck test
.PHONY: all

build:
	CGO_ENABLED=0 govendor build main.go

vet:
	go vet ./...

fmt:
	gofmt -w -s $$(find . -name '*.go' -not -path './vendor/*')

fmtcheck:
	lineCount=$(shell gofmt -l -s $$(find . -name '*.go' -not -path './vendor/*') | wc -l | tr -d ' ') && exit $$lineCount

test: build vet fmtcheck
