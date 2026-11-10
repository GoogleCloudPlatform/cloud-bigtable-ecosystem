# HBase to Bigtable Migration Validation

## Overview

This skill covers techniques for validating data integrity after migrating data
from Apache HBase to Google Cloud Bigtable, focusing on specific commands and
handling discrepancies.

## Validation Methods

### 1. Row Count Comparison

Compare the total number of rows in both databases. This is a quick sanity
check.

**Bigtable Command:**

```bash
cbt count <table-id>
```

**HBase Command:** Use the `RowCounter` MapReduce job for large tables, as
`count` in HBase shell can be slow and timeout.

```bash
hbase org.apache.hadoop.hbase.mapreduce.RowCounter <tablename>
```

### 2. Data Integrity Validation (HashTable & SyncTable) - RECOMMENDED

For byte-level data integrity validation, Google Cloud recommends using the
HBase native **`HashTable`** tool and the Bigtable **`sync-table`** Beam
pipeline on Cloud Dataflow.

#### Asynchronous Replication Gotcha (The "Why"):

If live writes continue to be applied to HBase while you run validation, the
asynchronous replication lag and in-flight mutations will cause **false-positive
mismatches** in `sync-table` hashes. To prevent this, you must choose one of the
following operational patterns:

--------------------------------------------------------------------------------

### Operational Pattern B: Live Sync Validation via "Quiet Period" (RECOMMENDED / DEFAULT)

This is the most robust validation method. It validates the absolute latest
state of your synchronized data (including all replayed WAL buffer writes) by
briefly pausing application writes.

#### Interactive Execution Protocol (HITL):

1.  **Explain the trade-off**: The agent **MUST** explain to the user that this
    method provides the highest validation confidence but requires a short
    "Quiet Period" (read-only mode) where application writes to HBase are
    paused.
2.  **Ask for explicit authorization**:
    *   *“We are ready to validate the fully synchronized live state. This
        requires briefly pausing all live writes to the HBase source table. Do
        you authorize pausing writes and starting the HashTable generation?
        Please reply with YES or NO.”*
3.  **Wait for confirmation**: Do not execute the `HashTable` or `sync-table`
    commands until the user explicitly replies with `YES`.

#### Execution Steps:

1.  Ensure the replication peer is active (`enable_peer '1'`) and the queue is
    fully drained (replication delay is `0` in `status 'replication'`).
2.  **Pause live writes** to the HBase source table (application side).
3.  **Compute hashes on HBase (HashTable)**:

    *   *Local Disk Fallback (when GCS Connector is not installed)*:

        ```bash
        mkdir -p /home/<user>/validation/hashtable
        hbase org.apache.hadoop.hbase.mapreduce.HashTable --batchsize=32000 --numhashfiles=20 \
            <tablename> file:///home/<user>/validation/hashtable/<tablename>
        gsutil rsync -r /home/<user>/validation/hashtable gs://<bucket>/validation/hashtable
        ```

    *   *Direct to GCS (Clusters with GCS Connector configured)*:

        ```bash
        hbase org.apache.hadoop.hbase.mapreduce.HashTable --batchsize=32000 --numhashfiles=20 \
            <tablename> gs://<bucket>/validation/hashtable/<tablename>
        ```

4.  **Execute SyncTable on Cloud Dataflow**:

    ```bash
    java -jar bigtable-beam-import-<VERSION>-shaded.jar sync-table \
        --runner=DataflowRunner \
        --project=<PROJECT_ID> \
        --bigtableInstanceId=<INSTANCE_ID> \
        --bigtableTableId=<TABLE_ID> \
        --hashTableOutputDir=gs://<bucket>/validation/hashtable/<tablename> \
        --outputPrefix=gs://<bucket>/validation/sync-table/output-<tablename>-$(date +"%s") \
        --stagingLocation=gs://<bucket>/staging \
        --tempLocation=gs://<bucket>/validation/sync-table/temp \
        --region=<REGION>
    ```

5.  **Evaluate Results**: Verify that `ranges_not_matched = 0` in the Dataflow
    custom counters (or in GCS output files).

6.  **Resume writes** and cut over immediately.

--------------------------------------------------------------------------------

### Operational Pattern A: Static Bootstrap Validation (Zero-Downtime Alternative)

If your application cannot tolerate a write pause, you must validate the static
historical snapshot *before* replaying the WAL buffer (while the target Bigtable
is still in a static, offline state).

#### Interactive Execution Protocol (HITL):

1.  **Explain the trade-off**: The agent must explain that this method validates
    the historical bootstrap import with zero downtime, but does not validate
    subsequent WAL buffer catch-up.
2.  **Ask for explicit authorization**:
    *   *“We are ready to validate the historical bootstrap data. This will
        clone your snapshot to a temp table and run validation before
        re-enabling live replication. Do you authorize this step? Please reply
        with YES or NO.”*

#### Execution Steps:

1.  Ensure the replication peer is **disabled** (`disable_peer '1'`) so live
    writes continue to buffer safely in the WAL.
2.  **Clone the snapshot** to a temporary table in HBase shell:

    ```hbase
    clone_snapshot '<snapshot_name>', '<tablename>_val_temp'
    ```

3.  **Run HashTable on the temporary table**:

    *   *Local Disk Fallback*:

        ```bash
        mkdir -p /home/<user>/validation/hashtable
        hbase org.apache.hadoop.hbase.mapreduce.HashTable --batchsize=32000 --numhashfiles=20 \
            <tablename>_val_temp file:///home/<user>/validation/hashtable/<tablename>
        gsutil rsync -r /home/<user>/validation/hashtable gs://<bucket>/validation/hashtable
        ```

4.  **Run SyncTable on Dataflow** against Bigtable using the `sync-table`
    command from Operational Pattern B (Step 4) above.

5.  Once validation is complete, **delete the temporary table** in HBase:

    ```hbase
    disable '<tablename>_val_temp'
    drop '<tablename>_val_temp'
    ```

6.  **Re-enable the replication peer** (`enable_peer '1'`) to replay the WAL
    backlog.

### 3. Sampling Validation

If full validation is too expensive, sample random row keys and compare full row
content.

*   Use `cbt read <table-id> count=100` to get a sample.

## Handling Discrepancies

If row counts or data do not match:

1.  **Identify Missing Rows**: Use a join or diff on the exported row keys to
    find which keys are missing in Bigtable.
2.  **Check Logs**: Look for Dataflow worker logs for failed mutations (e.g.,
    due to cell size limits).
3.  **Re-migrate Specific Ranges**: If a specific range failed, re-export and
    re-import that specific row key range to save time.

## Validation Challenges due to Mismatches

*   **Expired Data**: Because Bigtable does not automatically filter out expired
    data marked for deletion in an upcoming garbage collection cycle during a
    read, simple row counts or hash comparisons might show discrepancies if
    HBase has already garbage collected that data or filters it differently.
    Ensure validation scans use explicit filters to exclude expired data if
    needed.
*   **Timestamp Resolution**: Ensure that validation tools account for the
    millisecond-to-microsecond translation if comparing raw timestamps.

## References

*   Refer to standard HBase documentation for `RowCounter`.
*   [Bigtable Documentation](https://cloud.google.com/bigtable/docs)
