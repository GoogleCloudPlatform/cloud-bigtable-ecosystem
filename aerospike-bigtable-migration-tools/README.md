# Aerospike Migration Tools
This project provides tools for migrating data from Aerospike to Cloud Bigtable.

## High level overview of the process
The process consists of three parts:
- Backup migration - import an Aerospike backup into Cloud Bigtable (see [Dataflow template section](#dataflow-template-aerospikebackuptobigtable) for details),
- Streaming changes - replicate ongoing updates from Aerospike to Cloud Bigtable (see [Kafka Connect tools section](#kafka-connect-tools) for details),
- Cutover - switch the application to use Cloud Bigtable as the database.

## Submodules
The primary tool used for managing this project is Maven.

The project consists of the following Maven submodules:

### Adapter
Defines the transformation of Aerospike entities into Cloud Bigtable entities and provides utilities for performing it.
It is intended to be used by all the migration tools (including `replicator` and `backup-loader` described below) to ensure that the data mapping is consistent between them.

The important classes are:
- [`RowBuilder`](adapter/src/main/java/com/google/cloud/aerospike/RowBuilder.java): The definition of the transformation of Aerospike entities into Cloud Bigtable ones.
- [`BigtableMutationBuilder`](adapter/src/main/java/com/google/cloud/aerospike/BigtableMutationBuilder.java): A builder of Cloud Bigtable mutations creating Cloud Bigtable rows from Aerospike records.
- [`AerospikeRecord`](adapter/src/main/java/com/google/cloud/aerospike/AerospikeRecord.java): A utility for transforming Cloud Bigtable rows into Aerospike-like records.

### Replicator
A Kafka Connect Single Message Transformation responsible for converting Aerospike
[XDR JSON Kafka messages](https://aerospike.com/docs/connectors/streaming/common/formats/json-serialization-format/)
into messages ingestible by [Kafka Connect Bigtable Sink](https://github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/tree/main/kafka-connect-bigtable-sink).

It's meant to be used for streaming Aerospike changes into Cloud Bigtable.

### Backup Loader
Implements [`BackupReader`](backup-loader/src/main/java/com/google/cloud/aerospike/BackupReader.java), a reader of Aerospike backups.

Note that it is implemented by using the official [`aerospike-tools-backup`](https://github.com/aerospike/aerospike-tools-backup) library via Java Native Interface.

#### Dependencies
The Java `BackupReader` class requires a compiled native shared library to be present on the host (see [Dockerfile](Dockerfile)'s `dataflow-worker` target for an example how to satisfy this requirement).

`backup-loader` should be built in the Docker container of the project, for details see [Dockerfile](Dockerfile) and [backup-loader's pom.xml](backup-loader/pom.xml).

All the other modules are pure Java and can be built in any environment with supported Java and Maven versions.

### Backup Loader Examples
Contains runnable examples of BackupReader usage, along with scripts and config files for generating test backups.

## [Docker container](Dockerfile)
Provides:
- All necessary dependencies for building and running the project,
- Pre-installed tools like `maven`, `gcc`, `/app/bin/asbackup`

It also documents how to build both the project and the dependencies.

## Workflow
[Justfile](./Justfile) is an index of interesting commands and shortcuts for running them.

Note that if you want to execute `mvn` commands directly, you should do so from the top directory.

### Example commands
Building the project:
```bash
just build
```

Building just the backup reader (and its dependencies):
```bash
just run-mvn backup-loader compile
```

Running the backup loader example (note that it's only likely to run within the [container](#docker-container) due to [backup-loader's requirements](#dependencies)):
```bash
just run-emulator & # On the host, within the container there's no `docker`
just run-backup-loader
```

Installing `backup-loader` and `adapter` modules to the local Maven repository (so they can be added as dependencies in other locally built projects):
```bash
just install
```

### Generating A Backup
To generate a backup, start an Aerospike server:
```bash
just run-aerospike # On the host, within the container there's no `docker`
```

Next run the seeding script.
It will populate the database with some rows.
```bash
pip install aerospike==16.0.1
python seed_aerospike.py
```

Lastly, create the backup using the executable `asbackup` like so:
```bash
asbackup --host 127.0.0.0 --port 3000 --namespace dinosaurs --output-file backup1.asb
```

### Important Considerations:
Aerospike does not store keys by default - only digests.
It has to be configured to do so, the [config](backup-loader-example/aerospike/config) used by `just run-aerospike` has that option enabled.
That is why example backups made with this configuration located in the `backup-loader-example/aerospike/example_files` folder contain keys.

# Process of migration from Aerospike to Cloud Bigtable
This section documents how to obtain the binary artifacts needed for the process of migration from Aerospike to Cloud Bigtable.

## Dataflow template `AerospikeBackupToBigtable`

<!-- TODO: link to template -->
[A fork of DataflowTemplates contains `AerospikeBackupToBigtable`](TODO), a Dataflow template that can be used to load data from Aerospike backups into Cloud Bigtable.

Note that it uses `backup-loader` module for reading these files, so it uses `adapter` module for mapping Aerospike values into Cloud Bigtable ones.

### How to use it

#### Build and push the worker image
In this step we build OCI image used by Dataflow worker nodes to run the actual work on.

**CAUTION:** Use the tag matching the `beam.version` property of the root pom.xml from the Dataflow repository!
```bash
just build-worker-image $REGISTRY/$IMAGE_NAME-worker $VERSION
docker push $REGISTRY/$IMAGE_NAME-worker:$VERSION
```

#### Build and stage the template
In this step we build and publish (into buckets and registries configured by the arguments):
- Uber-jar of the Dataflow template
- OCI image of Dataflow template's job manager (which coordinates the worker nodes)
- JSON descriptor of the Dataflow template (it points to the job manager image and contains some metadata such as the template's arguments and their description)

##### Authentication
Note that this action needs to upload data to GCP using Application Default credentials.
It is also to be run in a Docker container (due to `backup-loader`'s dependencies), so you need to ensure that the process in the container is authenticated.

If you're running on GCP, you can just pass `--dns=169.254.169.254` argument to `docker run` command - see the command below to verify that the IP is correct and that Application Default Credentials use the expected service account:
```bash
docker run --dns=169.254.169.254 --rm curlimages/curl curl -sH "Metadata-Flavor: Google" http://169.254.169.254/computeMetadata/v1/instance/service-accounts/default/email
```

If you're running outside of it, follow the [official README](https://docs.cloud.google.com/docs/authentication/provide-credentials-adc).

##### Build
Build the container:
```bash
docker build . --target compiled -t aerospike-bigtable-migration-tools
```

Note that the build process of Dataflow pulls a large number of dependencies, so you might want to add flag such as `-v ./.m2:/root/.m2` to avoid redownloading them all if you end up needing to build it more than once.

Also mind the `--dns` flag described in [Authentication](#authentication) section.

Clone the `DataflowTemplates` repo and start the container with:
```bash
docker run --rm -it -v PATH_TO_DATAFLOW_TEMPLATES_REPO:/dataflow aerospike-migration-tools
```
Then within it run:
```bash
# Install Java packages of `aerospike-bigtable-migration-tools` into local maven repository.
just install
# Now do the operations on the Dataflow template.
cd /dataflow
mvn package -PtemplatesStage -DskipTests -DprojectId="$PROJECT_ID" -DbucketName=$BUCKET_NAME -DstagePrefix="templates" -DtemplateName="AerospikeBackupToBigtable" -DartifactRegistry=$REGISTRY/$IMAGE_NAME -pl v2/aerospike-backup-to-bigtable -am
```

#### Run the template
After executing the steps described, there should be:
- `$REGISTRY/templates/$IMAGE_NAME` - job manager image
- `$REGISTRY/$IMAGE_NAME-worker:$VERSION` - worker image
- `gs://$BUCKET_NAME/templates/flex/Aerospike_Backup_To_Bigtable` - Dataflow template's descriptor

To run it:
- go to https://console.cloud.google.com/dataflow/createjob
- pick `Custom template` as Dataflow template
- paste or pick the path to the Dataflow template descriptor
- fill in the template's parameters
- **[IMPORTANT]** Fill "SDK Container Image" field under "Optional parameters" with `$REGISTRY/$IMAGE_NAME-worker:$VERSION`

Alternatively you can do the same using `gcloud dataflow flex-template run` command or Terraform provider.
In any case, remember to use the worker image!

## Kafka Connect tools

You can set up a Kafka Connect pipeline consisting of:
- [`org.apache.kafka.connect.json.JsonConverter`](https://github.com/apache/kafka/blob/trunk/connect/json/src/main/java/org/apache/kafka/connect/json/JsonConverter.java),
- [`MapAerospikeConnectJsonToBigtableSinkInput`](replicator/src/main/java/com/google/cloud/aerospike/MapAerospikeConnectJsonToBigtableSinkInput.java),
- [`BigtableSinkConnector`](../kafka-connect-bigtable-sink/sink/src/main/java/com/google/cloud/kafka/connect/bigtable/BigtableSinkConnector.java).

It will respectively:
- deserialize [JSON-formatted Aerospike Outbound Connector's messages](https://aerospike.com/docs/connectors/streaming/kafka/outbound/formats/json-serialization-format),
- filter records older than some threshold and transform their Aerospike values into Cloud Bigtable ones,
- write the transformed Cloud Bigtable values into Cloud Bigtable.

### `replicator.jar` containing `MapAerospikeConnectJsonToBigtableSinkInput`
Run:
```bash
mvn clean package -pl adapter,replicator -DskipUnitTests
```
Copy the .jar from `replicator/target`.

### `sink.jar` containing `BigtableSinkConnector`
See the README from [../kafka-connect-bigtable-sink/README.md](../kafka-connect-bigtable-sink/README.md).
