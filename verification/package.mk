.PHONY: conformance docs

conformance:
	./scripts/check-conformance.sh

docs:
	go doc -all . >/dev/null
	test -z "$$(find docs -type f -name '*.md' -empty)"
