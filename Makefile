.PHONY: fmt fmt-check vet test test-race coverage build validate-docs validate-book validate-site validate tools-install security security-history release-readiness validate-release validate-interview

# Prepare the pinned dependency cache with go -C labs mod download before invoking make.
export GOTOOLCHAIN := local
export GOPROXY := off
export GOSUMDB := off
export GOWORK := off

fmt:
	find labs -name '*.go' -type f -print0 | xargs -0 gofmt -w

fmt-check:
	@files="$$(find labs -name '*.go' -type f -print0 | xargs -0 gofmt -l)"; \
	if [ -n "$$files" ]; then printf '%s\n' "$$files"; exit 1; fi

vet:
	go -C labs vet ./...

test:
	go -C labs test ./... -count=1

test-race:
	go -C labs test ./... -race -count=1

coverage:
	go -C labs test ./... -cover

build:
	@set -e; output_dir="$$(mktemp -d /tmp/ai-engineer-build.XXXXXX)"; \
	go -C labs list -f '{{if eq .Name "main"}}{{.ImportPath}}{{end}}' ./... > "$$output_dir/packages"; \
	while IFS= read -r package; do \
		[ -z "$$package" ] || go -C labs build -o "$$output_dir/$$(printf '%s' "$$package" | tr '/' '_')" "$$package"; \
	done < "$$output_dir/packages"; \
	printf 'build output: %s\n' "$$output_dir"

validate-interview:
	node scripts/interview.mjs validate
	node scripts/interview.mjs check-navigation
	node scripts/check-links.mjs content/interview
	node --test tests/interview/*.test.mjs tests/interview-site-source.test.mjs

validate-docs: validate-interview
	./scripts/validate-chapter-map.sh
	./scripts/validate-topics.sh
	./scripts/check-links.sh
	ruby scripts/verify-chapter-merge.rb
	ruby scripts/verify-book-template.rb
	ruby scripts/verify-docs-layout.rb
	ruby scripts/verify-release-readiness.rb
	ruby scripts/verify-ci.rb
	./scripts/check-privacy-leaks.sh

validate-book:
	python3 tests/diagrams-test.py
	python3 scripts/verify-diagrams.py
	python3 labs/part3-agent/chapter24/strategy-comparison/language_reflection_test.py
	python3 labs/part3-agent/chapter29/context-builder/compression_model_test.py
	python3 labs/part3-agent/chapter24/strategy-comparison/self_ask_test.py
	python3 labs/part3-agent/chapter24/strategy-comparison/model_search_test.py
	python3 labs/part3-agent/chapter24/strategy-comparison/goal_policy_test.py
	python3 labs/part3-agent/chapter24/strategy-comparison/bounded_search.py --self-check
	python3 labs/part3-agent/chapter30/skills-loader/comparison_test.py
	python3 labs/part3-agent/chapter28/evidence-answer/citation_input_test.py
	python3 labs/part3-agent/chapter28/evidence-answer/diagnosis_input_test.py
	python3 labs/part3-agent/chapter28/evidence-answer/accounting_input_test.py
	python3 labs/part3-agent/chapter33/eval-harness/diagnosis_review_test.py
	python3 labs/part3-agent/chapter33/eval-harness/case_registry_test.py
	python3 tests/agent-snippets-test.py
	python3 scripts/verify-agent-snippets.py
	ruby tests/privacy-check-test.rb
	ruby tests/public-links-test.rb
	ruby tests/release-snapshot-test.rb
	ruby tests/security-command-test.rb
	ruby tests/release-readiness-test.rb
	ruby tests/ci-policy-test.rb
	ruby tests/chapter-evidence-test.rb
	ruby tests/topic-metadata-test.rb
	ruby tests/docs-layout-test.rb
	ruby tests/book-template-test.rb
	ruby tests/chapter-merge-test.rb
	ruby scripts/verify-book-snippets.rb
	ruby scripts/verify-book-pilots.rb

validate-site:
	npm run check:sites

validate: fmt-check vet test-race coverage build validate-docs validate-book validate-site

tools-install:
	ruby scripts/security.rb --install

security:
	ruby scripts/security.rb
	npm run audit

security-history:
	ruby scripts/security.rb --history

release-readiness:
	ruby scripts/verify-release-readiness.rb --strict

RELEASE_MODE ?= index
validate-release:
	ruby scripts/validate-release.rb --mode "$(RELEASE_MODE)" --checks release
