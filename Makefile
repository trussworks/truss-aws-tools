SHELL = /bin/sh
VERSION = 2.5

all: install

go_version: .go_version.stamp
.go_version.stamp: bin/check_go_version
	bin/check_go_version
	touch .go_version.stamp

lambda_build: dep pre-commit-install test
	bin/make-lambda-build

lambda_release: lambda_build
	bin/make-lambda-release $(S3_BUCKET) $(VERSION)

install: dep pre-commit-install test
	go install github.com/trussworks/truss-aws-tools/...

test: dep pre-commit
	bin/make-test

pre-commit: pre-commit-install
	pre-commit run --all-files

dep: go_version .dep.stamp

.dep.stamp: Gopkg.lock .prereqs.stamp
	bin/make-dep
	touch .dep.stamp

pre-commit-install: .pre-commit-install.stamp dep

.pre-commit-install.stamp: .git/hooks/pre-commit
	touch .pre-commit-install.stamp

.git/hooks/pre-commit:
	pre-commit install

prereqs: .prereqs.stamp

.prereqs.stamp: bin/prereqs
	bin/prereqs
	touch .prereqs.stamp

clean:
	rm -f .*.stamp *.zip

.PHONY: clean go_version dep all test pre-commit pre-commit-install prereqs
