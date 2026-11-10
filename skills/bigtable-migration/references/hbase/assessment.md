# HBase to Bigtable Migration Assessment

## Overview

This sub-skill guides you through the initial data and schema extraction phase
of migrating Apache HBase to Google Cloud Bigtable. It is designed to safely
gather cluster metadata without requiring direct agent access to the HBase
environment.

> [!WARNING]
> **Dataproc Anti-Pattern Warning**: HBase on Google Cloud Dataproc is **not** a
> supported hosting or target environment, and must **NEVER** be mentioned or
> suggested to users. The only supported hosting source environments are
> **On-Premises** (bare-metal/VMs) or **AWS EMR**.

## The Assessment Workflow

### 1. Automated Extraction

A packaged Python extraction script is provided to automate read-only metadata
and schema discovery.

*   **Script Path**:
    [scripts/hbase/hbase_extractor.py](../../scripts/hbase/hbase_extractor.py)
*   **Execution Setup**: Copy `hbase_extractor.py` to (or run it from) a machine
    with HBase client connectivity, ideally using a Python Virtual Environment
    (`venv`):

    ```bash
    python3 -m venv hbase_assessment_venv
    source hbase_assessment_venv/bin/activate
    # Run against all tables
    python3 hbase_extractor.py
    # OR Scope to specific tables
    python3 hbase_extractor.py --include-tables "table1,table2"
    # OR Scope via regex
    python3 hbase_extractor.py --table-regex "^prod_.*"
    ```

*   **Interactive Sizing Prompts**: The script prompts you whether to scan
    physical HDFS table sizes via Hadoop CLI (useful for Capacity Planning, but
    requires a configured Hadoop environment; you can also pass `--scan-hdfs` or
    `--no-scan-hdfs` for non-interactive runs).

*   **Outputs**:

    *   `hbase_assessment_report.txt`: A thorough human-readable checklist and
        review for the operator.
    *   `hbase_assessment_input.json`: The machine-readable payload intended for
        input to the migration agent.

### 2. Manual or Guided Discovery

If automated extraction is not feasible, the assessment relies on collecting the
following critical inputs from the cluster:

*   HBase version and connectivity architecture.
*   Explicit checklist of **Unsupported Features**:
    *   Are **Coprocessors** used?
    *   Are **Custom Filters** deployed?
    *   Are **Namespaces** configured?
*   Column Family names (must match regex `[_a-zA-Z0-9][-_.a-zA-Z0-9]*`).
*   Estimated table row counts and physical byte volume.

## Transition to Planning

Once the `hbase_assessment_input.json` payload (or equivalent manual discovery
data) is collected and provided in the conversation, the workflow transitions to
the **Planning** sub-skill to generate the formal Migration Plan.

## Automated Assessment Processing & Operations

When a user initiates the Automated Assessment workflow:

1.  **Local Execution Guarantees**: Assure the user that the extraction script
    (`hbase_extractor.py`) runs completely locally in their secure environment
    and only performs read-only metadata queries (`list`, `describe`,
    `get_splits`, `list_peers`, and `hadoop fs -du`). It does not require
    external ZooKeeper connectivity or secret/credential sharing.
2.  **Metadata Extracted**:
    *   Table lists and active schemas.
    *   Column family policies (compression, bloom filters, TTL, max versions,
        block cache, in-memory priority).
    *   Regional metadata (region counts and sample split keys).
    *   Physical HDFS table sizes (if Hadoop CLI is configured).
    *   Replication peers.
    *   Presence of server-side Coprocessors.
3.  **Payload Analysis**: When the user provides the
    `hbase_assessment_input.json` payload back to the conversation:
    *   Acknowledge receipt of the payload.
    *   Invoke `parse_hbase_assessment_json` (or run
        `python3 scripts/hbase/hbase_json_parser.py <file>`) to parse and digest
        the table structures and replication peer settings.
    *   Use search tools to look up documentation if needed for specific GC
        policy calculations or secondary access advice.
    *   Identify schema risks: Column Family names violating regex
        `[_a-zA-Z0-9][-_.a-zA-Z0-9]*`, detected Coprocessors, Custom Filters, or
        explicit Namespaces.
    *   Route directly to Phase 2 (Planning) to generate the formal
        `migration_plan.md` plan.
