SHELL = /bin/sh
VERSION = 2.4

all: install

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

dep: .dep.stamp

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

.PHONY: clean dep all test pre-commit pre-commit-install prereqs
