SHELL = /bin/sh
VERSION = 3.0

all: install

lambda_build: pre-commit-install test
	bin/make-lambda-build

lambda_release: lambda_build
	bin/make-lambda-release $(S3_BUCKET) $(VERSION)

install: pre-commit-install test
	go install github.com/trussworks/truss-aws-tools/...

.PHONY: test
test: pre-commit
	bin/make-test

pre-commit: pre-commit-install
	pre-commit run --all-files

pre-commit-install: .pre-commit-install.stamp

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

.PHONY: clean all test pre-commit pre-commit-install prereqs
