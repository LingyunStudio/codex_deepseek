.PHONY: all cli gui dmg clean test cover cover-html cover-check
.DEFAULT_GOAL := all

all: cli

cli:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o codeseek ./cmd/codeseek/

gui:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o codeseek-gui ./cmd/codeseek-gui/

dmg:
	bash mac.sh dmg

clean:
	bash mac.sh clean

test:
	CGO_ENABLED=0 go test ./...

cover:
	CGO_ENABLED=0 go test -cover ./...

cover-html:
	CGO_ENABLED=0 go test -coverprofile=/tmp/codeseek-cover.out ./...
	go tool cover -html=/tmp/codeseek-cover.out

COVERAGE_THRESHOLD := 95

cover-check:
	@fail=0; \
	for pkg in $$(CGO_ENABLED=0 go test -cover ./... 2>&1 | grep 'coverage:' | grep -v '0.0%' | grep -v 'no statements'); do \
		echo "$$pkg"; \
	done; \
	echo ""; \
	echo "--- Enforced packages ---"; \
	for pkg in internal/extension/plugin; do \
		pct=$$(CGO_ENABLED=0 go test -cover ./$$pkg/ 2>&1 | grep -oP '[0-9]+\.[0-9]+(?=%)'); \
		echo "$$pkg: $${pct}%"; \
		if [ $$(echo "$${pct} < $(COVERAGE_THRESHOLD)" | bc -l) -eq 1 ]; then \
			echo "  FAIL: $${pct}% < $(COVERAGE_THRESHOLD)%"; \
			fail=1; \
		fi; \
	done; \
	if [ $$fail -eq 1 ]; then echo "Coverage check FAILED"; exit 1; fi; \
	echo "Coverage check PASSED"
