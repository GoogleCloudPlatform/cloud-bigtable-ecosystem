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
    *   **Coprocessors**: Bigtable does not support coprocessors
        (`org.apache.hadoop.hbase.coprocessor`). Any server-side logic must be
        moved to the application layer.
    *   **Custom Filters**: Custom filters inheriting from
        `org.apache.hadoop.hbase.filter.Filter` are not supported (to use a
        Bigtable filter with no HBase equivalent, wrap it in
        `com.google.cloud.bigtable.hbase.filter.BigtableFilter`). Only
        `BinaryComparator` and `RegexStringComparator` (using RE2 syntax) are
        supported.
    *   **Namespaces**: Bigtable does not use namespaces (namespace `Admin`
        methods throw `UnsupportedOperationException` unless
        `google.bigtable.namespace.warnings=true` is set). Per Bigtable
        documentation, you can
        [use row key prefixes to simulate namespaces](https://cloud.google.com/bigtable/docs/schema-design#multi-tenancy)
        (for example, `tenant#device#...`) when consolidating datasets that
        share the same schema into one table—the most efficient approach and
        avoids Bigtable's 1,000-table-per-instance limit. If you keep separate
        tables instead, Bigtable table IDs must match
        `[_a-zA-Z0-9][-_.a-zA-Z0-9]*` (up to 50 characters, so `:` and `#` are
        not permitted in table names)—replace `namespace:tablename` with an
        underscore prefix such as `namespace_tablename` (or
        `namespace__tablename` when table names themselves contain underscores).
*   **Mutations and Deletes Mismatches**:
    *   Deleting only the latest version of a column, deleting all values in a
        row with a specific timestamp, or deleting all values in a column family
        at or before a specific timestamp is **not supported**.
    *   The following `Delete` methods are not supported: `Delete(byte[] row,
        long timestamp)`, `addColumn(byte[] family, byte[] qualifier)`,
        `addFamily(byte[] family, long timestamp)`, and `addFamilyVersion(byte[]
        family, long timestamp)`.
*   **Gets and Scans Mismatches**:
    *   `Scan#setMaxResultsPerColumnFamily(int limit)` is **not supported**.
    *   `Scan#setBatch(int batch)`, `Scan#setAllowPartialResults(boolean)`, and
        `Scan#setMaxResultSize(long)` are ignored (rows are always returned
        whole).
    *   On synchronous `Table#getScanner(Scan)` calls, `Scan#setCaching(int
        caching)` controls how many rows the client fetches at a time;
        `AsyncTable#getScanner()` ignores `setCaching()`.
    *   Bigtable doesn't automatically filter out expired data marked for GC
        during reads. You must use a filter in the read request to avoid reading
        expired data.
*   **Connection Management**: Creating a `Connection` object is expensive.
    Create one per process and share it. They are thread-safe.

## Schema Design Considerations

*   **Row Key Design**: Design row keys (4 KB maximum) around your planned read
    queries (point lookups, row key prefixes, or contiguous row ranges) using
    human-readable `#`-delimited string segments so Key Visualizer remains
    effective. Do **not** start row keys with a timestamp alone, reversed
    timestamp alone, or sequential numeric ID, which pushes writes onto a single
    node and creates hotspots—precede timestamps with a high-cardinality value
    like a user or device ID (e.g., `machine_4223421#1425330757685`), reverse
    sequential numeric IDs, and avoid starting write-heavy row keys with very
    low-cardinality prefixes unless combined with granular segments or salting.
*   **Column Families**: Create up to about **100 column families per table**
    (creating more than 100 may cause performance degradation). Choose short
    names for column families because names are included in the data transferred
    for each request.
*   **Table and Column Family Names**: Table IDs (up to 50 characters) and
    column family names (up to 64 characters) must follow the regular expression
    `[_a-zA-Z0-9][-_.a-zA-Z0-9]*` (letters, digits, underscores, hyphens, and
    periods only; `:` and `#` are not permitted). You may need to rename tables
    or column families during migration.
*   **Row and Cell Size Limits**: Keep single-cell values under **10 MB** (hard
    limit **100 MB** per cell) and all values in a single row under **100 MB**
    (hard limit **256 MB** per row). HBase clusters sometimes store larger
    cells, which will fail during Bigtable import if not remediated beforehand.

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
            Bigtable instance using `gcloud bigtable instances create
            <INSTANCE_ID> --display-name="<DISPLAY_NAME>"
            --cluster-config=id=<CLUSTER_ID>,zone=<ZONE>,nodes=<NODES>` (or
            autoscaling flags; defaulting to a cost-effective single-node
            cluster for initial testing or sized to target load).
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

    *   HBase `TTL => '<SEC>'` maps to Bigtable age-based GC (`cbt setgcpolicy
        <TABLE_ID> <FAMILY_ID> maxage=<N>s` or `<N>d`; `'FOREVER'` means no
        age-based expiration).
    *   HBase `VERSIONS => '<N>'` maps to Bigtable version-based GC (`cbt
        setgcpolicy <TABLE_ID> <FAMILY_ID> maxversions=<N>`), and combined
        policies use `and` / `or` in `cbt setgcpolicy` (or `&&` / `||` inside
        quoted `cbt createtable "families=..."` expressions).

*   **Pre-splitting at Table Creation (`splits=` / `--splits`)**:

    *   Bigtable only accepts manual split points **when creating a new table**
        (manual splits cannot be added after a table is created, and the Google
        Cloud Console does not support pre-splitting). Iterate through the
        extracted `split_keys_sample` and generate the table creation block
        using the Cloud Bigtable Schema Translation tool (when preserving exact
        HBase table names, column families, GC policies, and splits), `cbt
        createtable <TABLE_ID> "families=..." splits=<KEY1>,<KEY2>`, or `gcloud
        bigtable instances tables create <TABLE_ID> --instance=<INSTANCE_ID>
        --project=<PROJECT_ID> --column-families="..." --splits=<KEY1>,<KEY2>`
        so tablets are pre-distributed across nodes prior to bulk ingestion.

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

### 2. Online Migration (Almost No Downtime)

*   **Trade-off**: **Minimal / Near-Zero Downtime**. Client applications
    continue reading and writing to HBase while the Cloud Bigtable HBase
    replication library asynchronously replicates WAL mutations to Bigtable and
    historical snapshot data is imported in parallel. Note that by default, the
    destination Bigtable table must have the same name and column families as
    the source HBase table.
*   **Detailed Operational Process**:

#### Step 1: Download and Install the Bigtable Replication Library

Download the `jar-with-dependencies` file for your HBase major version (`1.x` or
`2.x`; HBase 3.x is not supported) from Maven Central and copy it to a folder on
the HBase classpath (for example, `/usr/lib/hbase/lib/`) on **every** HBase
server, including masters and RegionServers:

```bash
# Example for HBase 2.x (use bigtable-hbase-1.x-replication for HBase 1.x; e.g., version 1.0.0)
wget https://repo1.maven.org/maven2/com/google/cloud/bigtable/bigtable-hbase-2.x-replication/<VERSION>/bigtable-hbase-2.x-replication-<VERSION>-jar-with-dependencies.jar \
    -P /usr/lib/hbase/lib/
```

*Note: Restart the HBase servers after copying the JAR so the replication
library is loaded.*

#### Step 2: Configure Target Properties in `hbase-site.xml`

Create a service account with `roles/bigtable.user` and a Bigtable app profile
configured with **single-cluster routing** (required to preserve write
ordering), then add the following properties to `hbase-site.xml` across the
entire HBase cluster:

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
  <name>google.bigtable.app_profile.id</name>
  <value>APP_PROFILE_ID</value>
</property>
<property>
  <name>google.bigtable.auth.json.keyfile</name>
  <value>JSON_FILE_PATH</value>
</property>
```

#### Step 3: Add the Bigtable Peer, Buffer Writes, and Enable Table Replication

1.  **Add the Cloud Bigtable replication endpoint as a peer** in HBase shell:

    *   **For HBase 2.x**:

        ```hbase
        add_peer '1', ENDPOINT_CLASSNAME => 'com.google.cloud.bigtable.hbase2_x.replication.HbaseToCloudBigtableReplicationEndpoint'
        ```

    *   **For HBase 1.x**:

        ```hbase
        add_peer '1', ENDPOINT_CLASSNAME => 'com.google.cloud.bigtable.hbase1_x.replication.HbaseToCloudBigtableReplicationEndpoint'
        ```

2.  **Disable the peer temporarily to buffer live writes**: Immediately disable
    the peer so live mutations from this point onward buffer in the HBase
    cluster's WAL while you migrate historical data:

    ```hbase
    disable_peer '1'
    ```

3.  **Enable table replication** for each table you are migrating:

    ```hbase
    enable_table_replication 'TABLE_NAME'
    ```

#### Step 4: Bootstrap Historical Data (Snapshot & Export)

*   **Take an HBase snapshot**:

    ```bash
    echo "snapshot 'TABLE_NAME', 'SNAPSHOT_NAME'" | hbase shell -n
    ```

*   **Export the snapshot to GCS**: Depending on whether your HBase cluster has
    the Hadoop Cloud Storage connector installed across its nodes, choose one of
    the following methods:

    *   **Method A: Export directly to GCS (Clusters with GCS Connector
        installed)**:

        ```bash
        hbase org.apache.hadoop.hbase.snapshot.ExportSnapshot \
            -Dhbase.zookeeper.quorum=ZOOKEEPER_QUORUM:ZOOKEEPER_PORT \
            -snapshot SNAPSHOT_NAME \
            -copy-from MIGRATION_SOURCE_DIRECTORY \
            -copy-to gs://YOUR_BUCKET/hbase-migration-snap/data
        ```

    *   **Method B: Export locally and sync with `gsutil rsync` (Fallback when
        GCS Connector is not installed)**: Export the snapshot to a local disk
        first, then sync it to GCS using `gsutil rsync` (which preserves hidden
        `.hbase-snapshot` directories):

        ```bash
        hbase org.apache.hadoop.hbase.snapshot.ExportSnapshot \
            -snapshot SNAPSHOT_NAME \
            -copy-to file:///path/to/hbase-export/ \
            -mappers 1

        gsutil rsync -r /path/to/hbase-export gs://YOUR_BUCKET/hbase-migration-snap/data
        ```

*   **Import the snapshot into Cloud Bigtable via Cloud Dataflow**: Download the
    `bigtable-beam-import` shaded JAR (for example, `2.20.2`) and execute
    `importsnapshot` on Cloud Dataflow (append `--enableSnappy=true` if the
    table uses Snappy compression):

    ```bash
    wget https://repo1.maven.org/maven2/com/google/cloud/bigtable/bigtable-beam-import/<VERSION>/bigtable-beam-import-<VERSION>-shaded.jar

    java -jar bigtable-beam-import-<VERSION>-shaded.jar importsnapshot \
        --runner=DataflowRunner \
        --project=PROJECT_ID \
        --bigtableInstanceId=INSTANCE_ID \
        --bigtableTableId=TABLE_NAME \
        --hbaseSnapshotSourceDir=gs://YOUR_BUCKET/hbase-migration-snap/data \
        --snapshotName=SNAPSHOT_NAME \
        --stagingLocation=gs://YOUR_BUCKET/hbase-migration-snap/staging \
        --tempLocation=gs://YOUR_BUCKET/hbase-migration-snap/temp \
        --maxNumWorkers=$((3 * CLUSTER_NUM_NODES)) \
        --region=REGION
    ```

#### Step 5: Re-enable Replication, Validate Parity, and Cut Over

*   Re-enable the replication peer in `hbase shell` (`enable_peer '1'`) so the
    buffered WAL writes drain and replay onto Bigtable.
*   Validate data parity (`HashTable` + Dataflow `sync-table`).
*   Configure your application to connect to Bigtable and switch traffic over.

## References

*   [Migrate data to Bigtable (HBase overview)](https://cloud.google.com/bigtable/docs/migrate#migrate-hbase)
*   [Migrate data from HBase to Bigtable offline](https://cloud.google.com/bigtable/docs/migrate-hbase-data-to-bigtable)
*   [Replicate from HBase to Bigtable (online migration)](https://cloud.google.com/bigtable/docs/hbase-replication)
*   [Differences between HBase and Cloud Bigtable](https://cloud.google.com/bigtable/docs/hbase-differences)
*   [Schema design best practices](https://cloud.google.com/bigtable/docs/schema-design)

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
    create the target Cloud Bigtable instance using `gcloud bigtable instances
    create` with `--cluster-config=id=<CLUSTER_ID>,zone=<ZONE>,nodes=<NODES>`
    (or autoscaling flags).
5.  **Bigtable Schema Design**: Recommend compliant table IDs (simulating HBase
    namespaces with `namespace#rowkey` prefixes when consolidating matching
    schemas, or replacing `namespace:tablename` colons with underscore prefixes
    such as `namespace_tablename` / `namespace__tablename` when keeping separate
    tables) and verify Column Family naming compliance. Provide the Cloud
    Bigtable Schema Translation tool or optimized `cbt createtable` commands
    (including `families=...` and `splits=...` at table creation time) as the
    default schema setup block, while noting that declarative Terraform
    resources or Java/Python SDK programmatic client code blocks can be
    generated upon request.
6.  **Migration Execution Strategy**: Recommend a primary pathway (e.g., Offline
    for small test tables or Online for zero-downtime production workloads).
    However, the plan **MUST** present both the **Offline Pathway
    (Export/Import)** and the **Online Pathway (Live Replication)** as explicit,
    actionable options, including tool requirements and a step-by-step technical
    walkthrough for both so the user can choose their preferred route.
