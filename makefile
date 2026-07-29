SHELL := /usr/bin/env zsh
MODULE := $(shell go list -m)

.PHONY: publish
publish:
	@if [ -z "$(VERSION)" ]; then \
		echo "FATAL: VERSION is required. Usage: make publish VERSION=v1.0.0"; \
		exit 1; \
	fi
	@if [[ ! "$(VERSION)" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then \
		echo "FATAL: VERSION must be a valid SemVer string starting with 'v'"; \
		exit 1; \
	fi
	git tag $(VERSION)
	git push origin $(VERSION)
	GOPROXY=https://proxy.golang.org go list -m $(MODULE)@$(VERSION)