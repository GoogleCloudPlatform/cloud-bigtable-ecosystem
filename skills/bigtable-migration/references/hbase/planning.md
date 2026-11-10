# HBase to Bigtable Migration Planning

## Overview

This skill guides you through the planning phase of migrating from Apache HBase
to Google Cloud Bigtable. It is designed to generate data-driven **Migration
Plan Artifacts** covering technical differences, schema design considerations,
and choosing the right migration strategy.

## Key Technical Differences

*   **API Compatibility**: The Cloud Bigtable HBase client for Java targets
    versions 1.x and 2.x of the Apache HBase API. If your application targets
    older versions (e.g., 0.98), you must update it to use `ConnectionFactory`
    instead of the deprecated `HConnectionManager`.
*   **Unsupported Features**:
    *   **Coprocessors**: Bigtable does not support coprocessors. Any
        server-side logic must be moved to the application layer.
    *   **Custom Filters**: Custom filters are not supported. Only standard
        HBase filters are available.
    *   **Namespaces**: Bigtable does not have explicit namespaces. Table names
        can include prefixes (e.g., `namespace:tablename`) to simulate them.
*   **Mutations and Deletes Mismatches**:
    *   Deleting all values with a specific timestamp in a given column family
        or row is **not supported**.
    *   The following `Delete` methods are not supported: `Delete(byte[] row,
        long timestamp)`, `addFamily(byte[] family, long timestamp)`,
        `addFamilyVersion(byte[] family, long timestamp)`.
*   **Gets and Scans Mismatches**:
    *   `Scan#setMaxResultsPerColumnFamily(int limit)` is **not supported**.
    *   `setBatch(int batch)` and `setCaching(int caching)` are ignored.
    *   Bigtable doesn't automatically filter out expired data marked for GC
        during reads. You must use a filter in the read request to avoid reading
        expired data.
*   **Connection Management**: Creating a `Connection` object is expensive.
    Create one per process and share it. They are thread-safe.

## Schema Design Considerations

*   **Row Key Design**: Avoid monotonically increasing keys (like timestamps
    alone) to prevent hotspotting. While Bigtable handles splits automatically,
    good row key design is still crucial. Avoid row keys with high cardinality
    at the start if possible, or use salting (though Bigtable handles sequential
    writes better than HBase, it's still a best practice to distribute load).
*   **Column Families**: Keep the number of column families small (typically <
    10). Name them with short strings as they are stored with every cell.
*   **Column Family Names**: Must follow the regular expression
    `[_a-zA-Z0-9][-_.a-zA-Z0-9]*`. You may need to rename families during
    migration.
*   **Cell Size Limits**: Bigtable has a hard limit of 100 MB per cell and 256
    MB per row. Recommended size is < 10 MB. HBase can sometimes handle larger
    cells, so this is a critical corner case to check.

## Consuming Assessment Outputs

To formulate a data-driven, optimized Migration Plan, this skill consumes the
output generated in Phase 1 (specifically a JSON extraction payload provided by
the user, which must follow the schema of the example file
[scripts/hbase/hbase_assessment_input_example.json](../../scripts/hbase/hbase_assessment_input_example.json)).

### 1. Automated JSON Parsing

To safely parse and summarize large JSON extraction payloads, you **MUST**
utilize the helper parser script located at:

*   **Script Path**:
    [scripts/hbase/hbase_json_parser.py](../../scripts/hbase/hbase_json_parser.py)

**Operational Directives**:

*   If you have Python execution capabilities in your runtime environment,
    execute `python3 scripts/hbase/hbase_json_parser.py <file>` (or invoke
    `parse_hbase_assessment_json`) to summarize the payload.
*   Analyze the parsed dictionary returned by the script: pay close attention to
    `hosting`, `total_tables`, `replication_peers`, `unsupported_features`, and
    the details in `tables_analyzed` (such as column family naming warnings,
    region counts, split points, compression types, and coprocessor lists).

### 2. Infrastructure Mapping Rules

When parsing the assessment payload, the agent must execute the following
advanced mappings for the final `migration_plan.md` artifact:

*   **Coprocessor Action Plan**:
    *   Map detected Coprocessors from the DB Assessment JSON directly to
        technical Remediation steps in the plan.
*   **Custom Filter Action Plan (IDE Execution Trigger)**:
    *   Mark status as `Optional / Action Required`.
    *   The plan **MUST** include a dedicated "Custom Filter Analysis" section.
        This section must instruct the user to **invoke the Bigtable Migration
        Agent inside their application project/IDE workspace** (using their
        local IDE context) to automatically analyze their Java/Python source
        files for `FilterBase` logic and generate Bigtable Translation Rules.
*   **Migration Execution Strategy (Transition to Phase 3)**:

    *   The Migration Plan artifact **MUST** deliver a highly thorough
        "Migration Execution Strategy" section.
    *   **Required Content**:
        *   Step-by-step provisioning instructions to create the target Cloud
            Bigtable instance using the `gcloud bigtable instances create` CLI
            command (defaulting to a cost-effective single-node development
            cluster setup).
        *   Detailed list of tools needed (`cbt`, `Hadoop CLI`, `Dataflow
            templates`).
        *   Deep comparison of the **Offline Pathway** (MapReduce `Export` /
            `ExportSnapshot` to GCS -> Dataflow) vs the **Online Pathway**
            (Replication Library), with live documentation links
            (`cloud.google.com/bigtable/docs/...`).
        *   A concluding technical offer/CTA prompting the user that the agent
            is ready to **assist in deploying and configuring these exact
            tools** (e.g., generating `cbt` setup blocks or Cloud Dataflow
            execution commands) to kick off **Phase 3: Data Migration**.

*   **GC Policy Translation**:

    *   HBase `TTL => 'SEC'` or `'FOREVER'` maps to Bigtable Age-based GC
        (calculated in seconds/days for `cbt setgcpolicy`, stored at microsecond
        granularity).
    *   HBase `VERSIONS => 'N'` maps to Bigtable Version-based GC.

*   **Pre-splitting Infrastructure (`cbt addsplits`)**:

    *   Iterate through the extracted `split_keys_sample`. Generate the precise
        CLI execution block using `cbt addsplits <table-id> <key>...` so the
        user can warm the Bigtable cluster prior to bulk ingestion.

*   **Replication Risks**:

    *   If `replication_peers` are detected, the plan **MUST** recommend the
        Online (Replication Endpoint) migration path and highlight the Gotcha of
        out-of-order `Deletes-mask-Puts`.

*   **Performance Policy Mapping**:

    *   **`IN_MEMORY`**: Note that Bigtable handles in-memory caching implicitly
        via its optimized block cache layer. Map tables with `IN_MEMORY =>
        'true'` to high-priority read workloads in the capacity plan.
    *   **`BLOCKCACHE`**: For column families with `BLOCKCACHE => 'false'`,
        evaluate whether skipping Bigtable's server-side block cache is desired
        (Bigtable caches aggressively by default; high-scale scan workloads
        should be optimized via SSD node types).

## Migration Strategies

Selecting a migration strategy requires balancing application downtime
constraints against operational complexity.

### 1. Offline Migration (Requires Application Downtime)

*   **Trade-off**: **High Downtime**. Application writes to HBase must be
    completely stopped for the entire duration of the export, upload, and import
    process. Any writes occurring after the export starts will result in data
    drift and inconsistency.
*   **Process**: Stop client application writes -> Export data via HBase
    `ExportSnapshot` (or MapReduce `Export`) -> Copy to GCS -> Import to
    Bigtable via Cloud Dataflow.
*   **Corner Case**: Large cells exceeding Bigtable limits (100 MB hard limit)
    will fail during import. Pre-migration scanning for large cells is highly
    recommended.

--------------------------------------------------------------------------------

### 2. Online Migration (Zero Downtime)

*   **Trade-off**: **Zero Downtime**. Client applications continue reading and
    writing to HBase with zero interruption. HBase asynchronously mirrors all
    live updates to Cloud Bigtable in real-time, while historical data is copied
    in the background.
*   **Detailed Operational Process**:

#### Step 1: Download and Install the Bigtable Replication Library

Download the Google Cloud Bigtable replication library JAR compatible with your
HBase major version (e.g., HBase 2.x) and copy it directly to the `/lib` folder
on all active HBase RegionServers:

```bash
# Replace '<HBASE_HOME>' with your actual HBase installation directory (e.g. /usr/local/hbase)
# Replace '<VERSION>' with the latest stable library version (e.g. 2.14.2)
wget https://repo1.maven.org/maven2/com/google/cloud/bigtable/bigtable-hbase-2.x-replication/<VERSION>/bigtable-hbase-2.x-replication-<VERSION>.jar \
    -P <HBASE_HOME>/lib/
```

*Note: You must restart your HBase RegionServers (or the HBase service) to load
the classpath.*

#### Step 2: Configure target properties in `hbase-site.xml`

Edit `<HBASE_HOME>/conf/hbase-site.xml` on the HBase cluster to define the
target Bigtable connection and credential settings. Add the following XML block
inside the `<configuration>` tag:

```xml
<property>
  <name>google.bigtable.project.id</name>
  <value>PROJECT_ID</value>
</property>
<property>
  <name>google.bigtable.instance.id</name>
  <value>INSTANCE_ID</value>
</property>
<property>
  <name>google.bigtable.auth.json.keyfile</name>
  <value>PATH_TO_ADC_OR_SERVICE_ACCOUNT_JSON</value>
</property>
```

#### Step 3: Register the Bigtable Replication Peer and Enable Replication Scope

1.  **Enable replication scope on your table's column families**: HBase requires
    you to explicitly enable replication on every column family by setting its
    `REPLICATION_SCOPE` to `1`. Run this in the HBase shell for all families:

    ```hbase
    # Open HBase shell: hbase shell
    alter 'TABLE_ID', {NAME => 'COLUMN_FAMILY_NAME', REPLICATION_SCOPE => '1'}
    ```

2.  **Register Cloud Bigtable as an asynchronous replication peer**: Use the
    correct shaded package matching your source HBase version:

    *   **For HBase 2.x**:

        ```hbase
        add_peer '1', ENDPOINT_CLASSNAME => 'com.google.cloud.bigtable.hbase2_x.replication.HbaseToCloudBigtableReplicationEndpoint'
        ```

    *   **For HBase 1.x**:

        ```hbase
        add_peer '1', ENDPOINT_CLASSNAME => 'com.google.cloud.bigtable.hbase1_x.replication.HbaseToCloudBigtableReplicationEndpoint'
        ```

3.  **Disable the peer temporarily to start buffering**: Disable the peer
    immediately after creation. This forces HBase to begin buffering all live
    mutations safely in the replication log queue while we copy historical data,
    preventing live writes from failing before the table is populated:

    ```hbase
    disable_peer '1'
    ```

#### Step 4: Bootstrap Historical Data (Snapshot & Export)

*   **Take an HBase snapshot**:

    ```hbase
    snapshot 'TABLE_ID', 'SNAPSHOT_NAME'
    ```

*   **Export the snapshot to GCS**: Depending on whether your HBase cluster has
    the Hadoop Google Cloud Storage (GCS) connector configured, choose one of
    the following methods:

    *   **Method A: Export locally and sync with `gsutil rsync` (Recommended
        when GCS Connector is not installed)**: Export the snapshot to a local
        disk first, then sync it to GCS using `gsutil rsync` (which safely
        preserves hidden `.hbase-snapshot` directories):

        ```bash
        hbase org.apache.hadoop.hbase.snapshot.ExportSnapshot \
            -snapshot SNAPSHOT_NAME \
            -copy-to file:///path/to/hbase-export/ \
            -mappers 1

        gsutil rsync -r /path/to/hbase-export gs://YOUR_BUCKET/snapshots
        ```

    *   **Method B: Export directly to GCS (Clusters with GCS Connector
        configured)**:

        ```bash
        hbase org.apache.hadoop.hbase.snapshot.ExportSnapshot \
            -snapshot SNAPSHOT_NAME \
            -copy-to gs://YOUR_BUCKET/snapshots/ \
            -mappers 16
        ```

*   **Import the snapshot into Cloud Bigtable via Cloud Dataflow**: Download the
    Beam Import tool shaded JAR and execute the `importsnapshot` job on Cloud
    Dataflow:

    ```bash
    wget https://repo1.maven.org/maven2/com/google/cloud/bigtable/bigtable-beam-import/<VERSION>/bigtable-beam-import-<VERSION>-shaded.jar

    java -jar bigtable-beam-import-<VERSION>-shaded.jar importsnapshot \
        --runner=DataflowRunner \
        --project=PROJECT_ID \
        --bigtableInstanceId=INSTANCE_ID \
        --bigtableTableId=TABLE_ID \
        --hbaseSnapshotSourceDir=gs://YOUR_BUCKET/snapshots \
        --snapshotName=SNAPSHOT_NAME \
        --stagingLocation=gs://YOUR_BUCKET/staging \
        --tempLocation=gs://YOUR_BUCKET/temp \
        --maxNumWorkers=<3 * Bigtable Nodes> \
        --region=REGION
    ```

#### Step 5: Re-enable Replication, Synchronize, and Cut Over

*   Re-enable the replication peer in `hbase shell` (`enable_peer '1'`) and
    allow the replication endpoint to process and drain the WAL backlog queue.
*   Verify synchronization (row counts and latest timestamps).
*   Update application database connection strings to switch traffic to Cloud
    Bigtable with zero downtime.

## References

*   [HBase to Cloud Bigtable Migration Guide](https://cloud.google.com/bigtable/docs/hbase-migration-overview)

## Generating the Formal Migration Plan

When a user provides the JSON assessment payload (following the schema of the
example file
[scripts/hbase/hbase_assessment_input_example.json](../../scripts/hbase/hbase_assessment_input_example.json))
generated by the `hbase_extractor.py` script, the agent **MUST** use this
information to generate a formal **Migration Plan Artifact**
(`migration_plan.md`).

### Required Plan Structure:

The Migration Plan generated by the agent must include:

1.  **Executive Summary**: Overview of cluster size (sum of HDFS bytes and rows
    if known) and top-level hosting platform.
2.  **Compatibility Matrix**: Checklisted table mapping identifying OK vs.
    Violating Column Families, and Coprocessor presence.
3.  **Remediation Action Plan**: Step-by-step technical instructions for
    removing or migrating any detected Coprocessors or Custom Filters to the
    application layer.
4.  **Target Instance Provisioning**: Step-by-step provisioning instructions to
    create the target Cloud Bigtable instance using the `gcloud bigtable
    instances create` CLI command (defaulting to a cost-effective single-node
    development cluster setup).
5.  **Bigtable Schema Design**: Recommend table names (with namespace prefixes
    if applicable) and verify Column Family naming compliance. Provide optimized
    `cbt` CLI creation commands as the default schema setup block (noting that
    they must be run against the newly created instance), while noting that
    declarative Terraform resources or Java/Python SDK programmatic client code
    blocks can be generated upon request.
6.  **Migration Execution Strategy**: Recommend a primary pathway (e.g., Offline
    for small test tables or Online for zero-downtime production workloads).
    However, the plan **MUST** present both the **Offline Pathway
    (Export/Import)** and the **Online Pathway (Live Replication)** as explicit,
    actionable options, including tool requirements and a step-by-step technical
    walkthrough for both so the user can choose their preferred route.
