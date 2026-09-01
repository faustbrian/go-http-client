SHELL := /usr/bin/env bash

GOLIB ?= golib

.PHONY: check ci inventory repository-check

check:
	$(GOLIB) check --all

ci: repository-check check

inventory:
	$(GOLIB) inventory

repository-check:
	$(GOLIB) repository check
