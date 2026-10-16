.PHONY: test coverage-html

test:
	go test -timeout 10m -count=1 -coverprofile=coverage.out -coverpkg=./internal/... ./...

coverage-html:
	go tool cover -html=coverage.out
