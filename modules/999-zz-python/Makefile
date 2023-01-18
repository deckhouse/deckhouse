DEPS_PATH=lib/python/dist/

.PHONY: all clean deps
clean:
	rm -rf ${DEPS_PATH}

deps:
	pip install shell-operator -t ${DEPS_PATH} --upgrade

all: clean deps
