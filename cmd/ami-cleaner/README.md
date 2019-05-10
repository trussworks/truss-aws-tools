# AMI Cleaner

This tool is designed to remove AMIs and their associated snapshots (if
EBS-based AMIs) from AWS. The tool offers a number of possible filtering
techniques for determining which AMIs to remove:

* Days of retention
* Name prefix
* Tag key/value pair
* Unused by instances

## Usage

Here are the flags that this tool can take:

| Short | Long | Env | Type | Description |
| ----- | ---- | --- | ---- | ----------- |
| -D | --delete | DELETE | bool | Actually purge AMIs (runs in dryrun mode by default) |
| | --prefix | NAME_PREFIX | string | Name prefix to filter on (not affected by --invert) |
| | --days | RETENTION_DAYS | integer | Age of AMI in days before it is a candidate for removal (default 30) |
| | --tag-key | TAG_KEY | string | Key of tag to operate on (if set, value must also be set) |
| | --tag-value | TAG_VALUE | string | Value of tag to operate on (if set, key must also be set) |
| -i | --invert | INVERT | string | Operate in tag inverted mode -- only purge AMIs that do NOT match the tag provided |
| | --unused | UNUSED | bool | Only purge AMIs for which no running instances were built from |
| -p | --profile | AWS_PROFILE | AWS profile to use |
| -r | --region | AWS_REGION | AWS region to use |
| | --lambda | LAMBDA | bool | Run as an AWS Lambda function |

## Examples

Here are some examples of how you can use this tool from the command line:

```bash
ami-cleaner --prefix="my_ami" --tag-key="Branch" --tag-value="master"
```

This invocation will check AWS for AMIs in your account which have names
which begin with "my_ami" and the tag "Branch: master" which are older
than 30 days (the default retention). It will *not* actually purge them,
since we did not set the -D flag.

```bash
ami-cleaner --tag-key="Branch" --tag-value="master" -i --days=7 -D
```

This invocation will look for all AMIs which do *not* have the tag
"Branch: master" (because we have the -i flag set), which are older than
7 days, and then it will deregister them and delete their snapshots
(because we *do* have the -D flag set here).

```bash
ami-cleaner --prefix="bad_ami" --tag-key="Branch" --tag-value="master" -i -D
```

This invocation will look for AMIs with names that begin with "bad_ami",
which *do not* have the tag "Branch: master" set, which are older than 30
days, and purge them. Note that invert does *not* operate on the prefix
argument, only on the tags.
