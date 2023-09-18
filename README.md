# truss-aws-tools

AWS tools that come in handy.

| Tool                    | Description                                                                                                                                                                         | AWS Lambda Support |
|-------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------|
| aws-remove-user         | Remove an AWS User's access keys and MFA devices.                                                                                                                                   | N/A                |
| ebs-delete              | snapshots an EBS volume before deleting, and won't delete volumes that belong to CloudFormation stacks.                                                                             | No                 |
| iam-keys-check          | checks users for old access keys and sends notification to a Slack webhook url                                                                                                      | Yes                |
| rds-snapshot-cleaner    | removes manual snapshot for a RDS instance that are older than X days or over a maximum snapshot count.                                                                             | Yes                |
| s3-bucket-size          | figures out how many bytes are in a given bucket as of the last CloudWatch metric update. Must faster and cheaper than iterating over all of the objects and usually "good enough". | No                 |
| trusted-advisor-refresh | triggers a refresh of Trusted Advisor because AWS doesn't do this for you.                                                                                                          | Yes                |
| aws-health-notifier     | Sends notifcations to a Slack webhook when AWS Health Events (read AWS outage) are triggered                                                                                        | Yes                |
| ami-cleaner             | Deregisters AMIs and deletes associated snapshots based on name/tag/age                                                                                                             | Yes                |
| packer-janitor          | Removes abandoned Packer instances and their associated keypairs and security groups.                                                                                               | Yes                |
| ebs-snapshot-cleaner    | Cleans ebs snapshots.                                                                                                                                                               | Yes                |

## Installation

``` shell
go get -u github.com/trussworks/truss-aws-tools/...
```

## Developer Setup

### Install dependencies (macOS)

``` shell
brew install pre-commit direnv
brew install golangci/tap/golangci-lint
```

Then run `./bin/prereqs` and follow any instructions that appear.

### Install dependencies (Debian Linux)

``` shell
sudo apt-get install direnv
pip install pre-commit
go get -u github.com/golangci/golangci-lint/cmd/golangci-lint
```

Then run `./bin/prereqs` and follow any instructions that appear.

### Build Local Binaries

``` shell
make all # Automatically setup pre-commit and Go dependencies before tests and build.
```

### Create Lambda

To build a zip for AWS Lambda to execute, run the following

``` shell
make S3_BUCKET=your-s3-bucket lambda_release
```

If using aws-vault:

```shell
aws-vault exec <aws_profile> -- make S3_BUCKET=your-s3-bucket lambda_release
```

## Release a new version

Partially complete automation has been put in place to release a new version of the tools. The following steps should be followed to release a new version:

* Add a tag to the desired commit i.e. `3.8`
* GitHub Actions will automatically build the binaries and upload them as artefacts on the repository
* Download the artefacts (zip file), extract the individual zip files and upload them to the S3 Bucket `us-east-1-dflt-ops-jenkins-lambdas-artifacts` in the `ops-default` account with the path `truss-aws-tools/<version>/`

## Tools wanted

* s3 deletion tool that purges a key AND all versions of that key.
* ebs volume snapshot deleter (all snaps older than x days, support keep tags)
* redshift snapshot cleaner
* automatic filesystem resizer (use case: you can make EBS volumes larger, but if you do, you still have to go in and run resize2fs (or whatever). Why not just do this at boot always?
* AWS id lookup (ie, figure out from the id which describe API to call, and do it).
* ebs snapshot creator (for all EBS volumes, trigger a snapshot).
* Something that will pull AWS Bucket Inventory data (AWS ships it as an Athena or Hive compatible format, so you need to read a manifest.json and then pull a set of CSV or ORC files).
