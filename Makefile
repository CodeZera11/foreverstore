build:
	@go build -o bin/foreverstore

run: build
	@./bin/foreverstore

test:
	@go test -v ./...