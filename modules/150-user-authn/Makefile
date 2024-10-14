WH_DEPS_PATH := ./webhooks/lib/python/dist/

.PHONY: all clean deps
clean:
	rm -rf ${WH_DEPS_PATH}

deps:
	pip install deckhouse -t ${WH_DEPS_PATH} --upgrade
	pip install dotmap -t ${WH_DEPS_PATH} --upgrade


all: clean deps
