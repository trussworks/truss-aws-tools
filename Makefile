test:
	go test github.com/trussworks/truss-aws-tools/...
all:
	go install github.com/trussworks/truss-aws-tools/...
dep:
	dep ensure
pre-commit:
	pre-commit run --all-files
