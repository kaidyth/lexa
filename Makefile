SHELL := /bin/bash
GIT_VERSION := $(shell git describe --dirty --always --tags)

help:	## Lists all available commands and a brief description.
	@grep -E '^[a-zA-Z/_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'


.DEFAULT_GOAL := default

default: ## Builds Lexa
	go build -ldflags="-X 'github.com/kaidyth/lexa/command.version=\"$(GIT_VERSION)\"' -X 'github.com/kaidyth/lexa/command.architecture=\"$(shell uname)/$(shell arch)\"'" \