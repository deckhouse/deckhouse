DEPS_PATH := ./lib/python/dist/
WH_DEPS_PATH := ./webhooks/lib/python/dist/

.PHONY: all clean deps test
clean:
	rm -rf ${DEPS_PATH}
	rm -rf ${WH_DEPS_PATH}

deps:
	pip install deckhouse-sdk -t ${DEPS_PATH} --upgrade
	pip install pyyaml -t ${DEPS_PATH} --upgrade

	pip install deckhouse-sdk -t ${WH_DEPS_PATH} --upgrade

all: clean deps

test:
	PYTHONPATH=${DEPS_PATH} pytest hooks
	PYTHONPATH=${WH_DEPS_PATH} pytest webhooks/conversion
