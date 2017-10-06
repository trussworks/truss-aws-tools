# truss-aws-tools

AWS tools that come in handy.

We use the upcoming Go dependency manager `dep`. `brew install dep` will get it for you.

* ebs-delete exists, needs tests and refactoring.

## Tools wanted

* s3 deletion tool that purges a key AND all versions of that key.

* ami-deregister that doesn't touch AMIs that are currently active or have been recently.
* ebs volume snapshot deleter (all snaps older than x days, support keep tags)

* rds snapshot cleaner
* redshift snapshot cleaner
