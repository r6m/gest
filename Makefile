.PHONY: test lint check

test:
	rtk go test ./...

lint:
	rtk proxy golangci-lint run ./...

check: test lint
