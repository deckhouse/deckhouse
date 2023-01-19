DEPS_PATH := lib/python/dist/

.PHONY: all clean deps test
clean:
	rm -rf ${DEPS_PATH}

deps:
	pip install deckhouse-sdk -t ${DEPS_PATH} --upgrade

all: clean deps

test:
	D8_MODULE_ROOT=$$(pwd) pytest hooks
