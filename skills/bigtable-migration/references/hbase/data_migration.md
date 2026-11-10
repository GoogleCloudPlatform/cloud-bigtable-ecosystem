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

Use the standard Cloud Dataflow template **`SequenceFile_to_Cloud_Bigtable`**:

```bash
gcloud dataflow jobs run JOB_NAME \
    --gcs-location gs://dataflow-templates-<REGION>/latest/SequenceFile_to_Cloud_Bigtable \
    --region <REGION> \
    --parameters \
        bigtableProject=<PROJECT_ID>,\
        bigtableInstanceId=<INSTANCE_ID>,\
        bigtableTableId=<TABLE_ID>,\
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
    gsutil rsync -r /<local_path> gs://<bucket>/snapshots
    ```

*   **Exporting directly to GCS (if GCS Connector is installed)**:

    ```bash
    hbase org.apache.hadoop.hbase.snapshot.ExportSnapshot \
        -snapshot <snapshot_name> \
        -copy-to gs://<bucket>/snapshots/ \
        -mappers 16
    ```

#### 2. Importing to Bigtable

Because snapshots are raw HFiles, they cannot be imported via standard
SequenceFile templates. You must download and run the open-source
**`bigtable-beam-import`** tool on Cloud Dataflow:

```bash
# Download the tool
wget https://repo1.maven.org/maven2/com/google/cloud/bigtable/bigtable-beam-import/<VERSION>/bigtable-beam-import-<VERSION>-shaded.jar

# Run the import job
java -jar bigtable-beam-import-<VERSION>-shaded.jar importsnapshot \
    --runner=DataflowRunner \
    --project=<PROJECT_ID> \
    --bigtableInstanceId=<INSTANCE_ID> \
    --bigtableTableId=<TABLE_ID> \
    --hbaseSnapshotSourceDir=gs://<bucket>/snapshots \
    --snapshotName=<snapshot_name> \
    --stagingLocation=gs://<bucket>/staging \
    --tempLocation=gs://<bucket>/temp \
    --maxNumWorkers=<3 * Bigtable Nodes> \
    --region=<REGION>
```

### Pre-import Step: Pre-splitting Bigtable

To avoid hotspotting a single node during a bulk import, pre-split the Bigtable
table based on expected row key distribution (matching your source HBase region
boundaries).

*   Use `cbt createtable <table-id>` and then `cbt addsplits <table-id>
    <row-key>...`

## Online Migration: Bigtable HBase Replication Library

For zero-downtime migration, use the open-source Bigtable HBase replication
library.

### Steps:

1.  **Add Peer**: Enable `REPLICATION_SCOPE => '1'` on column families and
    register the Cloud Bigtable replication endpoint in HBase shell:

    ```hbase
    # For HBase 2.x (use hbase1_x for HBase 1.x)
    add_peer '1', ENDPOINT_CLASSNAME => 'com.google.cloud.bigtable.hbase2_x.replication.HbaseToCloudBigtableReplicationEndpoint'
    ```

2.  **Buffer Writes**: Disable the peer (`disable_peer '1'`) to let live writes
    buffer safely in the WAL while historical data is migrated.

3.  **Migrate Snapshot**: Use Pathway B (`ExportSnapshot` +
    `bigtable-beam-import`) to migrate a snapshot of the historical data.

4.  **Drain Buffer**: Re-enable the peer (`enable_peer '1'`) to let the buffered
    writes replay onto Bigtable.

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
*   **Timestamp Resolution**: Bigtable uses microsecond resolution for
    timestamps, while HBase typically uses milliseconds. The Bigtable HBase
    client usually handles this by multiplying by 1000, but verify this if you
    are doing custom imports.
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
