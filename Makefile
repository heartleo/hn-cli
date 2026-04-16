.PHONY: build fmt tidy vet

build:
	go build github.com/heartleo/hn-cli/cmd/hn

fmt:
	go fmt ./...

tidy:
	go mod tidy

vet:
	go vet ./...
