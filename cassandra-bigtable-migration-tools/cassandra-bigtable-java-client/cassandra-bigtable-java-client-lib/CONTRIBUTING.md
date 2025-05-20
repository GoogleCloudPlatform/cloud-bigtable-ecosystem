# How to contribute

We'd love to accept your patches and contributions to this project.

## Before you begin

### Sign our Contributor License Agreement

Contributions to this project must be accompanied by a
[Contributor License Agreement](https://cla.developers.google.com/about) (CLA).
You (or your employer) retain the copyright to your contribution; this simply
gives us permission to use and redistribute your contributions as part of the
project.

If you or your current employer have already signed the Google CLA (even if it
was for a different project), you probably don't need to do it again.

Visit <https://cla.developers.google.com/> to see your current agreements or to
sign a new one.

### Review our community guidelines

This project follows
[Google's Open Source Community Guidelines](https://opensource.google/conduct/).

## Contribution process

### Code reviews

All submissions, including submissions by project members, require review. We
use GitHub pull requests for this purpose. Consult
[GitHub Help](https://help.github.com/articles/about-pull-requests/) for more
information on using pull requests.

# Running tests

## Integration tests

Pre-requisites:
- A Google Cloud Project
- A Bigtable Instance
- Go v1.21
- Java 8+
- Bigtable cbt client
- gcloud client

First, obtain gcloud credentials if needed:

```shell
gcloud auth application-default login
```

Then, specify the following environment variables:

```shell
GCP_PROJECT_ID=<YOUR-PROJECT-ID-HERE>
BIGTABLE_INSTANCE_ID=<YOUR-BIGTABLE-INSTANCE-ID-HERE>
```

Then, modify the following _test_ dependency in the `pom.xml` file by replacing
the `classifier` with your target platform/architecture (see [README](README.md)
for the full list of classifiers):

```xml
<dependency>
  <groupId>com.google.cloud</groupId>
  <artifactId>cassandra-bigtable-java-client-core</artifactId>
  <version>0.1.0-SNAPSHOT</version>
  <classifier>linux-amd64</classifier>
  <scope>test</scope>
</dependency>
```

Finally, run this command to execute the integration tests:

```shell
mvn verify \
  -Dcassandra.bigtable.projectid=$GCP_PROJECT_ID \
  -Dcassandra.bigtable.instanceid=$BIGTABLE_INSTANCE_ID \
  -Djava.util.logging.config.file=src/test/resources/logging.properties
```

Optionally, to override the test and schema mapping table names, set these environment variables:

```shell
SMOKE_TEST_TABLE=<YOUR-SMOKE-TEST-TABLE-NAME-HERE>
SCHEMA_MAPPING_TABLE=<YOUR-SCHEMA-MAPPING-TABLE-NAME-HERE>
```

And run:

```shell
mvn verify \
  -Dcassandra.bigtable.projectid=$GCP_PROJECT_ID \
  -Dcassandra.bigtable.instanceid=$BIGTABLE_INSTANCE \
  -Dcassandra.bigtable.schemamappingtable=$SCHEMA_MAPPING_TABLE \
  -Dcassandra.bigtable.smoketesttable=$SMOKE_TEST_TABLE \
  -Djava.util.logging.config.file=src/test/resources/logging.properties
```
