# HBase to Bigtable Data Migration Techniques

## Overview

This skill covers methods for exporting data from Apache HBase and importing it
into Google Cloud Bigtable, with a focus on technical precision and specific
commands.

## Offline Migration: Export and Import

You have two primary pathways to export historical data from HBase and import it
to Bigtable: **Pathway A (HBase Export Tool)** which produces SequenceFiles, and
**Pathway B (HBase ExportSnapshot Tool)** which produces raw HFiles. Pathway B
is highly recommended for large tables and online migrations as it has zero
impact on the active RegionServers during data transfer.

### Pathway A: HBase Export Tool (SequenceFiles)

Use this for smaller tables or when you want to filter data during export.

#### 1. Exporting from HBase

```bash
hbase org.apache.hadoop.hbase.mapreduce.Export <tablename> <output_directory_uri>
```

*   **Tip**: Use a GCS URI (`gs://my-bucket/path`) if the GCS connector is
    configured.
*   **Impact**: Scans the live table, which can impact HBase cluster
    performance.

#### 2. Importing to Bigtable

Use the Cloud Dataflow **`GCS_SequenceFile_to_Cloud_Bigtable`** template:

```bash
gcloud dataflow jobs run <JOB_NAME> \
    --gcs-location gs://dataflow-templates-<REGION>/latest/GCS_SequenceFile_to_Cloud_Bigtable \
    --region <REGION> \
    --parameters \
bigtableProject=<PROJECT_ID>,\
bigtableInstanceId=<INSTANCE_ID>,\
bigtableTableId=<TABLE_ID>,\
bigtableAppProfileId=<APPLICATION_PROFILE_ID>,\
sourcePattern=gs://<BUCKET>/<PATH>/part-*
```

--------------------------------------------------------------------------------

### Pathway B: HBase ExportSnapshot Tool (HFiles) - RECOMMENDED

Use this for large tables to avoid impacting live performance.

#### 1. Taking and Exporting Snapshot

First, take a snapshot in HBase shell:

```hbase
snapshot 'table_name', 'snapshot_name'
```

Then, export the snapshot.

> [!IMPORTANT]
> **GCS Connector Gotcha**: If your standalone HBase host or self-managed Hadoop
> cluster does not have the Google Cloud Storage connector configured in its
> Java classpath, exporting directly to `gs://` will fail. **Remediation**:
> Export the snapshot to a local directory first, then sync it to GCS using
> `gsutil rsync -r` so hidden `.hbase-snapshot` metadata directories are
> preserved.

*   **Exporting Locally (if no GCS Connector)**:

    ```bash
    hbase org.apache.hadoop.hbase.snapshot.ExportSnapshot \
        -snapshot <snapshot_name> \
        -copy-to file:///<local_path>/ \
        -mappers 1
    ```

    Then copy to GCS using `gsutil rsync` to safely replicate the local
    directory and hidden files (like `.hbase-snapshot`):

    ```bash
    gsutil rsync -r /<local_path> gs://<BUCKET>/hbase-migration-snap/data
    ```

*   **Exporting directly to GCS (if GCS Connector is installed)**:

    ```bash
    hbase org.apache.hadoop.hbase.snapshot.ExportSnapshot \
        -Dhbase.zookeeper.quorum=<ZOOKEEPER_QUORUM>:<ZOOKEEPER_PORT> \
        -snapshot <SNAPSHOT_NAME> \
        -copy-from <MIGRATION_SOURCE_DIRECTORY> \
        -copy-to gs://<BUCKET>/hbase-migration-snap/data
    ```

    *Tip*: To limit production network and I/O impact, pass `-mappers <N>` and
    `-bandwidth <MB_PER_SEC>` (for example, `-mappers 20 -bandwidth 50` caps
    total export throughput at 1,000 MB/sec).

#### 2. Importing to Bigtable

Because snapshots are raw HFiles, they cannot be imported via standard
SequenceFile templates. Download and run the open-source
**`bigtable-beam-import`** shaded JAR on Cloud Dataflow:

```bash
# Download the shaded JAR from Maven Central (e.g., version 2.20.2)
wget https://repo1.maven.org/maven2/com/google/cloud/bigtable/bigtable-beam-import/<VERSION>/bigtable-beam-import-<VERSION>-shaded.jar

# Run the import job
java -jar bigtable-beam-import-<VERSION>-shaded.jar importsnapshot \
    --runner=DataflowRunner \
    --project=<PROJECT_ID> \
    --bigtableInstanceId=<INSTANCE_ID> \
    --bigtableTableId=<TABLE_ID> \
    --hbaseSnapshotSourceDir=gs://<BUCKET>/hbase-migration-snap/data \
    --snapshotName=<SNAPSHOT_NAME> \
    --stagingLocation=gs://<BUCKET>/hbase-migration-snap/staging \
    --tempLocation=gs://<BUCKET>/hbase-migration-snap/temp \
    --maxNumWorkers=$((3 * <CLUSTER_NUM_NODES>)) \
    --region=<REGION>
```

*   **Snappy-compressed tables**: If the source HBase table uses Snappy
    compression, append `--enableSnappy=true` (requires Apache Beam SDK 2.30.0+
    on Dataflow Runner v2 and HBase client for Java 2.3.0+).

### Pre-import Step: Creating Destination Tables and Splits

Before running the import job, create the destination table in Bigtable and
pre-split it based on your source HBase region boundaries to avoid hotspotting a
single node during bulk load. Note that Bigtable only accepts manual split keys
**at table creation time** (manual splits cannot be added after a table is
created, and the Google Cloud Console does not support pre-splitting):

*   **Using the Cloud Bigtable Schema Translation tool (when keeping matching
    schema and table name)**: Automatically copies the HBase table name, column
    families, garbage collection policies, and region splits to Bigtable:

    ```bash
    # Download JAR with dependencies from Maven Central (e.g., version 2.20.2)
    wget https://repo1.maven.org/maven2/com/google/cloud/bigtable/bigtable-hbase-1.x-tools/<VERSION>/bigtable-hbase-1.x-tools-<VERSION>-jar-with-dependencies.jar

    java \
        -Dgoogle.bigtable.project.id=<PROJECT_ID> \
        -Dgoogle.bigtable.instance.id=<INSTANCE_ID> \
        -Dgoogle.bigtable.table.filter=<TABLE_NAME> \
        -Dhbase.zookeeper.quorum=<ZOOKEEPER_QUORUM> \
        -Dhbase.zookeeper.property.clientPort=<ZOOKEEPER_PORT> \
        -jar bigtable-hbase-1.x-tools-<VERSION>-jar-with-dependencies.jar
    ```

*   **Using `cbt` CLI (when customizing the schema or renaming tables)**:

    ```bash
    cbt createtable <TABLE_ID> "families=<FAMILY_ID>:maxage=<AGE>||maxversions=<N>" splits=<SPLIT_KEY_1>,<SPLIT_KEY_2>
    ```

*   **Using `gcloud` CLI**:

    ```bash
    gcloud bigtable instances tables create <TABLE_ID> \
        --instance=<INSTANCE_ID> \
        --project=<PROJECT_ID> \
        --column-families="<FAMILY_ID>:maxage=<AGE>" \
        --splits=<SPLIT_KEY_1>,<SPLIT_KEY_2>
    ```

## Online Migration: Bigtable HBase Replication Library

For online migration with almost no downtime, use the open-source Cloud Bigtable
HBase replication library (`bigtable-hbase-1.x-replication` or
`bigtable-hbase-2.x-replication`).

### Steps:

1.  **Configure `hbase-site.xml` and Install the Replication JAR**:
    *   Create a Bigtable app profile configured with **single-cluster routing**
        (required to preserve write ordering) and set
        `google.bigtable.project.id`, `google.bigtable.instance.id`,
        `google.bigtable.app_profile.id`, and
        `google.bigtable.auth.json.keyfile` in `hbase-site.xml` across all HBase
        servers.
    *   Copy `bigtable-hbase-<1.x|2.x>-replication-<VERSION>-jar-with-dependencies.jar`
        onto the HBase classpath (for example, `/usr/lib/hbase/lib/`) on every
        HBase master and RegionServer, then restart HBase servers.
2.  **Add Peer**: Register the Cloud Bigtable replication endpoint in HBase
    shell:

    ```hbase
    # Use hbase1_x for HBase 1.x or hbase2_x for HBase 2.x (HBase 3.x is not supported)
    add_peer '1', ENDPOINT_CLASSNAME => 'com.google.cloud.bigtable.hbase2_x.replication.HbaseToCloudBigtableReplicationEndpoint'
    ```

3.  **Buffer Writes & Enable Table Replication**: Disable the peer
    (`disable_peer '1'`) so new writes buffer safely in the HBase WAL, then
    enable table replication for each table:

    ```hbase
    disable_peer '1'
    enable_table_replication '<TABLE_NAME>'
    ```

4.  **Migrate Snapshot**: Follow Pathway B (`ExportSnapshot` +
    `bigtable-beam-import`) to migrate a snapshot of the historical data.

5.  **Drain Buffer**: Re-enable the peer (`enable_peer '1'`) to let buffered WAL
    writes replay onto Bigtable, validate parity, and switch application traffic
    to Bigtable.

## Semi-Autonomous Execution with Human-in-the-Loop (HITL)

For agents operating with command execution capabilities (e.g., SSH access to
HBase hosts, gcloud CLI, `cbt` CLI, or Terraform control), you **MUST** execute
the migration plan using a strict **Human-in-the-Loop (HITL)** protocol. This
ensures safe, controlled, and authorized data movement.

### 1. Pre-Execution: Network & Scope Verification

Before executing any migration steps, the agent must check and verify
environment sanity:

*   **GCE Access Scopes**: If running from a GCE VM, run diagnostics (`curl -H
    "Metadata-Flavor: Google"
    http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/scopes`)
    to ensure storage write scopes (`cloud-platform` or `devstorage.read_write`)
    are active when using the default VM service account.
*   **Firewall Rules**: Verify network configurations. If running Dataflow batch
    imports without Dataflow Shuffle, advise the user regarding internal TCP
    ports `12345-12346`. Prefer appending `--experiments=use_runner_v2` to
    utilize GCS-based Shuffle and bypass firewall needs entirely.
*   **IAM verification**: Verify that the active CLI account has GCS write/read
    permissions and Bigtable Admin/User roles.

### 2. Interactive Execution Protocol (HITL)

For **every single command** or configuration change (such as downloading jars,
editing `hbase-site.xml`, modifying schemas, taking snapshots, syncing
directories, or launching Dataflow jobs), the agent **MUST** follow this strict
interaction pattern:

1.  **Explain the Action**: Write a clear, plain-English summary explaining:
    *   What command is about to be run.
    *   Why it is necessary at this stage of the migration.
    *   What resources it will touch or modify (e.g., "Taking a snapshot of
        table X will freeze region metadata briefly but has zero live write
        impact").
2.  **Display the Command**: Render the exact, fully-parameterized command
    block.
3.  **Request Explicit Authorization**: Ask the user for confirmation:
    *   *“Do you authorize executing this step? Please reply with YES to proceed
        or specify any modifications.”*
4.  **Wait for User Confirmation**: **CRITICAL**: Do not invoke any terminal
    tools or execute any backend scripts for this step until the user has
    explicitly responded with `YES` (or a positive equivalent) in their next
    turn.

### 3. Post-Step Monitoring & Error Handling

After the user authorizes a step and the agent executes it:

*   **Report Output**: Show the relevant stdout/stderr logs or job URLs.
*   **Confirm Success**: Verify success parameters (e.g., checking that
    `ExportSnapshot` logs show `completed successfully`, or that Dataflow job
    status is `RUNNING` / `SUCCESSFUL`).
*   **Handle Errors Interactively**: If a command fails (such as a `GcsApiError`
    or `ClassNotFoundException`), do not try alternative destructive actions
    blindly. Summarize the failure, present remediation options (e.g., VM scope
    reboot vs. SA key upload), and ask the user how they wish to proceed.

## Technical Gotchas and Corner Cases

*   **Cell Size Limits**: Bigtable has a hard limit of 100 MB per cell. If your
    HBase data has cells larger than this, the import will fail. You must filter
    or truncate these before importing.
*   **Timestamp Resolution**: Bigtable stores timestamps in microseconds, while
    HBase stores timestamps in milliseconds. The Cloud Bigtable HBase client
    converts between milliseconds and microseconds automatically, but because
    the largest HBase timestamp Bigtable can store is `Long.MAX_VALUE / 1000`,
    any larger value (such as large reversed timestamps) is capped at
    `Long.MAX_VALUE / 1000` and may not convert as expected.
*   **Mutations per Row**: Bigtable has a limit of 100,000 mutations per row in
    a single transaction. If you have extremely wide rows with many versions,
    you might hit this limit during import.
*   **Deletes mask Puts**: In HBase, deletes mask puts. In Bigtable, a write
    request sent to a cell is **not affected** by a previously sent delete
    request to the same cell if the write happens after. This can lead to data
    discrepancies if mutations are replayed out of order or if applications rely
    on HBase delete-masking behavior.

## References

*   [Migrate data from HBase to Bigtable offline](https://cloud.google.com/bigtable/docs/migrate-hbase-data-to-bigtable)
*   [Replicate from HBase to Bigtable](https://cloud.google.com/bigtable/docs/hbase-replication)
