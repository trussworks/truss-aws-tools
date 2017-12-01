# truss-aws-tools

AWS tools that come in handy.

* ebs-delete snapshots an EBS volume before deleting, and won't delete volumes that belong to CloudFormation stacks.

## Developer Setup

### Install dependencies (macOS)

``` shell
brew install dep
brew install pre-commit
brew install gometalinter
gometalinter --install
```

### Install dependencies (Linux)

``` shell
go get -u github.com/golang/dep/cmd/dep
pip install pre-commit
go get -u github.com/alecthomas/gometalinter
gometalinter --install
```

### Build

``` shell
make all # Automatically setup pre-commit and Go dependencies before tests and build.
```

## Tools wanted

* s3 deletion tool that purges a key AND all versions of that key.

* ami-deregister that doesn't touch AMIs that are currently active or have been recently.
* ebs volume snapshot deleter (all snaps older than x days, support keep tags)

* rds snapshot cleaner
* redshift snapshot cleaner
* automatic filesystem resizer (use case: you can make EBS volumes larger, but if you do, you still have to go in and run resize2fs (or whatever). Why not just do this at boot always?
* Packer debris cleaner (old instances, security groups, etc)
