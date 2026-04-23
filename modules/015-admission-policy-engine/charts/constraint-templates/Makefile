SHELL := /bin/bash

CHART_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
TESTS_DIR := $(CHART_DIR)/tests
TEST_CASES_DIR := $(TESTS_DIR)/test_cases
CONSTRAINTS_ROOT := $(TEST_CASES_DIR)/constraints
CONSTRAINT_TESTGEN_SRC := $(TESTS_DIR)/tools/constraint_testgen
RUN_ALL_SCRIPT := $(TEST_CASES_DIR)/run_all_tests.sh
REPO_ROOT := $(abspath $(CHART_DIR)/../../../../)
CONSTRAINT_TESTGEN_BIN := $(REPO_ROOT)/.tmp/constraint-templates/constraint_testgen
CHART_REL := $(patsubst $(REPO_ROOT)/%,%,$(CHART_DIR))
CONTAINER_REPO_DIR := /work/deckhouse
CONTAINER_TESTS_DIR := $(CONTAINER_REPO_DIR)/$(CHART_REL)/tests

DOCKER_IMAGE ?= constraint-templates-tests:local
DOCKERFILE := $(TESTS_DIR)/images/test-container/dockerfile
ROOT_MAKEFILE := $(REPO_ROOT)/Makefile
ROOT_OPA_VERSION := $(strip $(shell awk -F '=' '/^OPA_VERSION[[:space:]]*=/{gsub(/[[:space:]]/,"",$$2); print $$2; exit}' "$(ROOT_MAKEFILE)"))
ROOT_GATOR_VERSION := $(strip $(shell awk -F '=' '/^GATOR_VERSION[[:space:]]*=/{gsub(/[[:space:]]/,"",$$2); print $$2; exit}' "$(ROOT_MAKEFILE)"))
OPA_VERSION ?= $(if $(filter v%,$(ROOT_OPA_VERSION)),$(ROOT_OPA_VERSION),v$(ROOT_OPA_VERSION))
GATOR_VERSION ?= $(if $(filter v%,$(ROOT_GATOR_VERSION)),$(ROOT_GATOR_VERSION),v$(ROOT_GATOR_VERSION))

.PHONY: help test all coverage constraint --docker --name docker-image constraint-testgen __run

help:
	@echo "Constraint templates test entrypoint"
	@echo ""
	@echo "Supported commands:"
	@echo "  make test all"
	@echo "  make test all --docker"
	@echo "  make test coverage all"
	@echo "  make test coverage all --docker"
	@echo "  make test constraint --name <name>"
	@echo "  make test constraint --name <name> --docker"
	@echo "  make test coverage constraint --name <name>"
	@echo "  make test coverage constraint --name <name> --docker"
	@echo ""
	@echo "NOTE: GNU make parses --name/--docker as options. For strict flag-style syntax, run with '--' separator:"
	@echo "  make test constraint -- --name allow-host-network"
	@echo "  make test all -- --docker"

# no-op goals used only as positional tokens for `make test ...` command-style invocations
all coverage constraint --docker --name:
	@:

# absorb extra positional token after --name <value>
%:
	@:

test:
	@set -euo pipefail; \
	goals=( $(filter-out $@,$(MAKECMDGOALS)) ); \
	mode="local"; \
	name=""; \
	action=""; \
	scope=""; \
	i=0; \
	while [ $$i -lt $${#goals[@]} ]; do \
	  g="$${goals[$$i]}"; \
	  case "$$g" in \
	    all|coverage|constraint) \
	      if [ -z "$$action" ]; then \
	        action="$$g"; \
	      elif [ -z "$$scope" ]; then \
	        scope="$$g"; \
	      fi; \
	      ;; \
	    --docker) \
	      mode="docker"; \
	      ;; \
	    --name) \
	      i=$$((i+1)); \
	      if [ $$i -ge $${#goals[@]} ]; then \
	        echo "error: --name requires value"; \
	        exit 2; \
	      fi; \
	      name="$${goals[$$i]}"; \
	      ;; \
	    --) \
	      ;; \
	    *) \
	      ;; \
	  esac; \
	  i=$$((i+1)); \
	done; \
	cmd=""; \
	if [ "$$action" = "all" ]; then \
	  cmd="all"; \
	elif [ "$$action" = "coverage" ] && [ "$$scope" = "all" ]; then \
	  cmd="coverage-all"; \
	elif [ "$$action" = "constraint" ]; then \
	  cmd="constraint"; \
	elif [ "$$action" = "coverage" ] && [ "$$scope" = "constraint" ]; then \
	  cmd="coverage-constraint"; \
	else \
	  echo "error: unsupported command syntax"; \
	  $(MAKE) --no-print-directory help; \
	  exit 2; \
	fi; \
	$(MAKE) --no-print-directory __run CMD="$$cmd" MODE="$$mode" NAME="$$name"

docker-image:
	@docker build \
	  --build-arg OPA_VERSION="$(OPA_VERSION)" \
	  --build-arg GATOR_VERSION="$(GATOR_VERSION)" \
	  -f "$(DOCKERFILE)" \
	  -t "$(DOCKER_IMAGE)" \
	  "$(REPO_ROOT)"

constraint-testgen:
	@mkdir -p "$(dir $(CONSTRAINT_TESTGEN_BIN))"
	@go build -o "$(CONSTRAINT_TESTGEN_BIN)" "$(CONSTRAINT_TESTGEN_SRC)"

__run:
	@set -euo pipefail; \
	resolve_dir() { \
	  local n="$$1"; \
	  local d=""; \
	  for g in security operation; do \
	    d="$$(find "$(CONSTRAINTS_ROOT)/$$g" -mindepth 1 -maxdepth 1 -type d -name "$$n" | head -n1 || true)"; \
	    if [ -n "$$d" ]; then \
	      echo "$$d"; \
	      return 0; \
	    fi; \
	  done; \
	  return 1; \
	}; \
	if [ "$(MODE)" = "docker" ]; then \
	  if ! docker image inspect "$(DOCKER_IMAGE)" >/dev/null 2>&1; then \
	    printf '\033[33m%s\033[0m\n' "============================================================"; \
	    printf '\033[33m%s\033[0m\n' "Docker image $(DOCKER_IMAGE) is not built yet. First build may take up to 5 minutes, please wait."; \
	    printf '\033[33m%s\033[0m\n' "============================================================"; \
	  fi; \
	  $(MAKE) --no-print-directory docker-image; \
	fi; \
	case "$(CMD)" in \
	  all) \
	    if [ "$(MODE)" = "docker" ]; then \
	      docker run --rm \
	        -v "$(REPO_ROOT):$(CONTAINER_REPO_DIR)" \
	        -w "$(CONTAINER_TESTS_DIR)" \
	        "$(DOCKER_IMAGE)" \
	        bash -c "./test_cases/run_all_tests.sh"; \
	    else \
	      bash "$(RUN_ALL_SCRIPT)"; \
	    fi; \
	    ;; \
	  coverage-all) \
	    if [ "$(MODE)" = "docker" ]; then \
	      docker run --rm \
	        -v "$(REPO_ROOT):$(CONTAINER_REPO_DIR)" \
	        -w "$(CONTAINER_TESTS_DIR)" \
	        "$(DOCKER_IMAGE)" \
	        bash -c "constraint_testgen coverage -tests-root ./test_cases/constraints -format table"; \
	    else \
	      $(MAKE) --no-print-directory constraint-testgen; \
	      "$(CONSTRAINT_TESTGEN_BIN)" coverage -tests-root "$(CONSTRAINTS_ROOT)" -format table; \
	    fi; \
	    ;; \
	  constraint) \
	    if [ -z "$(NAME)" ]; then \
	      echo "error: constraint name is required (--name <name>)"; \
	      exit 2; \
	    fi; \
	    if [ "$(MODE)" = "docker" ]; then \
	      docker run --rm \
	        -v "$(REPO_ROOT):$(CONTAINER_REPO_DIR)" \
	        -w "$(CONTAINER_TESTS_DIR)" \
	        "$(DOCKER_IMAGE)" \
	        bash -c 'set -euo pipefail; \
	          d=""; \
	          for g in security operation; do \
	            d="$$(find ./test_cases/constraints/$$g -mindepth 1 -maxdepth 1 -type d -name "$(NAME)" | head -n1 || true)"; \
	            [ -n "$$d" ] && break; \
	          done; \
	          [ -n "$$d" ] || { echo "constraint not found: $(NAME)"; exit 1; }; \
	          cd "$$d"; \
	          constraint_testgen generate -bundle ./test-matrix.yaml; \
	          gator verify -v ./rendered'; \
	    else \
	      $(MAKE) --no-print-directory constraint-testgen; \
	      d="$$(resolve_dir "$(NAME)")" || { echo "constraint not found: $(NAME)"; exit 1; }; \
	      (cd "$$d" && "$(CONSTRAINT_TESTGEN_BIN)" generate -bundle ./test-matrix.yaml && gator verify -v ./rendered); \
	    fi; \
	    ;; \
	  coverage-constraint) \
	    if [ -z "$(NAME)" ]; then \
	      echo "error: constraint name is required (--name <name>)"; \
	      exit 2; \
	    fi; \
	    if [ "$(MODE)" = "docker" ]; then \
	      docker run --rm \
	        -v "$(REPO_ROOT):$(CONTAINER_REPO_DIR)" \
	        -w "$(CONTAINER_TESTS_DIR)" \
	        "$(DOCKER_IMAGE)" \
	        bash -c 'set -euo pipefail; \
	          d=""; \
	          for g in security operation; do \
	            d="$$(find ./test_cases/constraints/$$g -mindepth 1 -maxdepth 1 -type d -name "$(NAME)" | head -n1 || true)"; \
	            [ -n "$$d" ] && break; \
	          done; \
	          [ -n "$$d" ] || { echo "constraint not found: $(NAME)"; exit 1; }; \
	          constraint_testgen coverage -tests-root "$$d" -format table'; \
	    else \
	      $(MAKE) --no-print-directory constraint-testgen; \
	      d="$$(resolve_dir "$(NAME)")" || { echo "constraint not found: $(NAME)"; exit 1; }; \
	      "$(CONSTRAINT_TESTGEN_BIN)" coverage -tests-root "$$d" -format table; \
	    fi; \
	    ;; \
	  *) \
	    echo "error: unknown command $(CMD)"; \
	    exit 2; \
	    ;; \
	esac
