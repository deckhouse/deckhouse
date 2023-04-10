.PHONY: deps
deps:
	pip3 install -r requirements.dev.txt

SHELL_OPERATOR_GET_DATA_FOR_MOCKS ?= false
.PHONY: test
test:
	SHELL_OPERATOR_GET_DATA_FOR_MOCKS=$(SHELL_OPERATOR_GET_DATA_FOR_MOCKS) python3 -m pytest hooks
