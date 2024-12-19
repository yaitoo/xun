.PHONY: lint unit-tests
lint:
	golangci-lint run

unit-tests:
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...