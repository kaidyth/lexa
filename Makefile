SHELL := /bin/bash
GIT_VERSION := $(shell git describe --dirty --always --tags)

BUILDTIME=$(shell date +%s)
SCRIPTPATH=$(shell pwd -P)
DRONE_TAG?=$(GIT_VERSION)

help:	## Lists all available commands and a brief description.
	@grep -E '^[a-zA-Z/_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := default

default: ## Builds Lexa
	go build -ldflags="-X 'github.com/kaidyth/lexa/command.version=\"$(GIT_VERSION)\"' -X 'github.com/kaidyth/lexa/command.architecture=\"$(shell uname)/$(shell arch)\"'" \

clean: ## Cleans up directories
	rm lexa
	rm -rf package
	rm -rf dist

fpm_debian: ## Creates Debian Package
	mkdir -p package
	mkdir -p dist
	mkdir -p package/etc/systemd/system
	mkdir -p package/etc/lexa
	mkdir -p package/usr/local/bin
	cp lexa package/usr/local/bin/lexa
	cp server/lexa.service package/etc/systemd/system/lexa-server.service
	cp agent/lexa.service package/etc/systemd/system/lexa-agent.service
	cp cluster/lexa.service package/etc/systemd/system/lexa-cluster.service

	fpm -s dir \
		-t deb \
		-n lexa \
		-v $(DRONE_TAG) \
		-C $(shell pwd)/package \
		-p $(shell pwd)/dist/lexa-$(DRONE_TAG)-$(shell uname -m).deb \
		-m "charlesportwoodii@erianna.com" \
		--license "proprietary" \
		--url "https://github.com/kaidyth/lexa" \
		--description "Lexa - instance and service discovery for LXD containers" \
		--vendor "Kaidyth" \
		--deb-systemd-restart-after-upgrade \
		--template-scripts \
		--force \
		--after-install "$(shell pwd)/.debian/postinstall-pak" \
		--before-remove "$(shell pwd)/.debian/preremove-pak" \
		--no-deb-auto-config-files \
		--deb-compression=gz

fpm_alpine: ## Creates an Alpine Package
	mkdir -p package
	mkdir -p dist
	mkdir -p package/etc/lexa
	mkdir -p package/usr/local/bin
	mkdir -p package/usr/local/etc/init.d

	cp .alpine/*.rc package/usr/local/etc/init.d/

	fpm -s dir \
		-t apk \
		-n lexa \
		-v $(DRONE_TAG) \
		-C $(shell pwd)/package \
		-p $(shell pwd)/dist/lexa-$(DRONE_TAG)-$(shell uname -m).apk \
		-m "charlesportwoodii@erianna.com" \
		--license "proprietary" \
		--url "https://github.com/kaidyth/lexa" \
		--description "Lexa - instance and service discovery for LXD containers" \
		--vendor "Kaidyth" \
		--force