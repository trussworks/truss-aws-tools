all: dep pre-commit-install test
	go install github.com/trussworks/truss-aws-tools/...
test: dep pre-commit
	go test github.com/trussworks/truss-aws-tools/...
pre-commit: pre-commit-install
	pre-commit run --all-files
dep: .dep.stamp
.dep.stamp: Gopkg.lock
	dep ensure
	touch .dep.stamp
pre-commit-install: .pre-commit-install.stamp dep
.pre-commit-install.stamp: .git/hooks/pre-commit
	touch .pre-commit-install.stamp
.git/hooks/pre-commit:
	pre-commit install
clean:
	rm -f .*.stamp
.PHONY: clean dep all test pre-commit pre-commit-install
