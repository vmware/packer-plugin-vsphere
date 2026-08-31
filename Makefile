NAME=vsphere
BINARY=packer-plugin-${NAME}
PLUGIN_FQN="$(shell grep -E '^module' <go.mod | sed -E 's/module *//')"

COUNT?=1
TEST?=$(shell go list ./...)
HASHICORP_PACKER_PLUGIN_SDK_VERSION?=$(shell go list -m github.com/hashicorp/packer-plugin-sdk | cut -d " " -f2)

.PHONY: dev build test install-packer-sdc plugin-check testacc \
	testacc-datasource \
	testacc-builder testacc-builder-iso testacc-builder-clone \
	testacc-post-processor testacc-post-processor-vsphere testacc-post-processor-vsphere-template \
	generate docs-deps docs-prepare docs-test docs-test-links docs-test-internal-links docs-test-admonitions \
	docs-test-example-labels docs-test-group-example-tabs docs-test-normalize docs-test-github-alerts \
	docs-build docs-serve docs-serve-version docs-serve-mike docs-serve-mike-only docs-backfill

build:
	@go build -o ${BINARY}

dev:
	@go build -ldflags="-X '${PLUGIN_FQN}/version.VersionPrerelease=dev'" -o '${BINARY}'
	packer plugins install --path ${BINARY} "$(shell echo "${PLUGIN_FQN}" | sed 's/packer-plugin-//')"

test:
	@go test -race -count $(COUNT) $(TEST) -timeout=3m

install-packer-sdc: ## Install packer sofware development command
	@go install github.com/hashicorp/packer-plugin-sdk/cmd/packer-sdc@${HASHICORP_PACKER_PLUGIN_SDK_VERSION}

plugin-check: install-packer-sdc build
	@packer-sdc plugin-check ${BINARY}

testacc: dev
	@PACKER_ACC=1 go test -count $(COUNT) -v $(TEST) -timeout=180m

TESTACC_DATASOURCE_RUN?=TestAccDatasource
testacc-datasource:
	@PACKER_ACC=1 go test ./datasource/... -count $(COUNT) -v -timeout=15m -run '$(TESTACC_DATASOURCE_RUN)'

TESTACC_BUILDER_ISO_RUN?=TestAccISOBuilder_Matrix
testacc-builder-iso: dev
	@PACKER_ACC=1 go test ./builder/vsphere/iso -count $(COUNT) -v -timeout=180m -run '$(TESTACC_BUILDER_ISO_RUN)'

TESTACC_BUILDER_CLONE_RUN?=TestAccCloneBuilder_Matrix
testacc-builder-clone: dev
	@PACKER_ACC=1 go test ./builder/vsphere/clone -count $(COUNT) -v -timeout=30m -run '$(TESTACC_BUILDER_CLONE_RUN)'

testacc-builder: testacc-builder-iso testacc-builder-clone

TESTACC_POST_PROCESSOR_VSPHERE_RUN?=TestAccPostProcessorVSphere_Matrix
testacc-post-processor-vsphere: dev
	@PACKER_ACC=1 go test ./post-processor/vsphere -count $(COUNT) -v -timeout=30m -run '$(TESTACC_POST_PROCESSOR_VSPHERE_RUN)'

TESTACC_POST_PROCESSOR_VSPHERE_TEMPLATE_RUN?=TestAccPostProcessorVSphereTemplate_Matrix
testacc-post-processor-vsphere-template: dev
	@PACKER_ACC=1 go test ./post-processor/vsphere-template -count $(COUNT) -v -timeout=30m -run '$(TESTACC_POST_PROCESSOR_VSPHERE_TEMPLATE_RUN)'

testacc-post-processor: testacc-post-processor-vsphere testacc-post-processor-vsphere-template

generate: install-packer-sdc
	@go generate ./...
	# Patch the generated files to use *os.FileMode so null values are handled correctly.
	@for f in builder/vsphere/common/output_config.hcl2spec.go builder/vsphere/common/step_export.hcl2spec.go; do \
		awk '/DirPerm/{gsub(/ os\.FileMode/, " *os.FileMode")} {print}' "$$f" > "$$f.tmp" && mv "$$f.tmp" "$$f"; \
	done
	@go fmt ./...
	@rm -rf .docs
	@packer-sdc renderdocs -src "docs" -partials docs-partials/ -dst ".docs/"
	@./.web-docs/scripts/compile-to-webdocs.sh "." ".docs" ".web-docs" "hashicorp"
	@rm -r ".docs"

DOCS_VENV?=$(CURDIR)/docs-site/.venv
DOCS_PYTHON=$(DOCS_VENV)/bin/python
DOCS_PIP=$(DOCS_VENV)/bin/pip
DOCS_BOOTSTRAP_PYTHON ?= $(shell command -v python3.14 || command -v python3.13 || command -v python3.12 || command -v python3.11 || command -v python3.10 || true)

docs-deps:
	@bootstrap="$(DOCS_BOOTSTRAP_PYTHON)"; \
	if [ -z "$$bootstrap" ]; then \
		echo "error: Python 3.10+ is required for zensical (python3 is $$(python3 -V 2>/dev/null || echo missing))" >&2; \
		echo "hint: brew install python && make docs-deps" >&2; \
		exit 1; \
	fi; \
	if [ -x "$(DOCS_PYTHON)" ] && ! "$(DOCS_PYTHON)" -c 'import sys; raise SystemExit(0 if sys.version_info >= (3, 10) else 1)'; then \
		echo "Recreating docs-site/.venv with $$("$$bootstrap" -V) (zensical requires Python 3.10+)"; \
		rm -rf "$(DOCS_VENV)"; \
	fi; \
	test -d "$(DOCS_VENV)" || "$$bootstrap" -m venv "$(DOCS_VENV)"; \
	"$(DOCS_PYTHON)" -m pip install --upgrade pip; \
	"$(DOCS_PIP)" install -r docs-site/requirements.txt

docs-prepare:
	@./docs-site/scripts/prepare-docs.sh

docs-test:
	@./docs-site/scripts/test/test-all.sh

docs-test-links:
	@./docs-site/scripts/test/test-rewrite-integration-links.sh

docs-test-internal-links:
	@./docs-site/scripts/test/test-fix-internal-links.sh

docs-test-admonitions:
	@./docs-site/scripts/test/test-convert-admonitions.sh

docs-test-example-labels:
	@./docs-site/scripts/test/test-format-example-labels.sh

docs-test-group-example-tabs:
	@./docs-site/scripts/test/test-group-example-tabs.sh

docs-test-normalize:
	@./docs-site/scripts/test/test-normalize-list-spacing.sh

docs-test-strip-codegen:
	@./docs-site/scripts/test/test-strip-codegen-comments.sh

docs-test-repair-fences:
	@./docs-site/scripts/test/test-repair-code-fences.sh

docs-test-stage-markdown:
	@./docs-site/scripts/test/test-stage-markdown.sh

docs-test-github-alerts:
	@./docs-site/scripts/test/test-convert-github-alerts.sh

docs-build: generate docs-deps docs-prepare
	@cd docs-site && "$(DOCS_VENV)/bin/zensical" build --config-file zensical.build.toml

docs-serve: generate docs-deps docs-prepare
	@cd docs-site && "$(DOCS_VENV)/bin/zensical" serve --config-file zensical.build.toml

docs-serve-version: docs-deps
	@test -n "$(VERSION)" || (echo "VERSION is required, e.g. make docs-serve-version VERSION=2.1.0" && exit 1)
	@rm -rf .web-docs
	@git checkout "v$(VERSION)" -- .web-docs
	@INCLUDE_EXTRA=true ./docs-site/scripts/prepare-docs.sh
	@cd docs-site && "$(DOCS_VENV)/bin/zensical" serve --config-file zensical.build.toml

docs-serve-mike: generate docs-deps
	@[ -z "$(VERSIONS)" ] || export MIKE_PREVIEW_VERSIONS="$(VERSIONS)"; ./docs-site/scripts/mike-preview.sh

docs-serve-mike-only: docs-deps
	@./docs-site/scripts/mike-preview.sh --serve-only

docs-backfill: docs-deps
	@./docs-site/scripts/mike-backfill.sh $(VERSIONS)
