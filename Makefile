GOLIB ?= golib

.PHONY: check ci inventory repository-check workflows

check:
	$(GOLIB) check --all

ci: repository-check workflows check

inventory repository-check:
	$(GOLIB) repository check

workflows:
	$(GOLIB) workflows check
