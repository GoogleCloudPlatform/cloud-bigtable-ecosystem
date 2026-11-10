# HBase to Bigtable Migration Skill

> [!NOTE]
> **Preview:** This skill is currently in **Preview**. While we encourage you
> to try it out and provide feedback, it is not yet considered generally
> available (GA) for production workloads. The included skills, scripts, and
> behaviors are subject to change without notice. Additionally, please be aware
> that this skill may use Google Cloud Bigtable migration tools and features
> that are also currently in Preview.

## Overview

This skill (`bigtable-migration`) enables AI agents (including Antigravity, Claude Code, Gemini CLI, Cursor, and other [Agent Plugins](https://agent-plugins.org/)-compatible clients) to streamline the database migration journey to Google Cloud Bigtable. Leveraging tools available on your local machine (`gcloud`, `cbt`, `hbase`, `gsutil`, and a read-only assessment script), the agent helps discover source cluster metadata, plan target schemas and capacity, orchestrate offline or online data migration, and validate post-migration data parity.

Currently, this skill supports **Apache HBase to Bigtable** migrations. Support for **Apache Cassandra to Bigtable** and **Aerospike to Bigtable** migrations is coming soon.

## Supported Migration Paths

| Migration path | Status | Details |
| :--- | :--- | :--- |
| **Apache HBase to Bigtable** | **Available (Preview)** | Full 4-phase migration workflow (`references/hbase/`) and read-only cluster assessment script (`scripts/hbase/hbase_extractor.py`). |
| **Apache Cassandra to Bigtable** | **Coming soon** | Migration skill in active development. In the meantime, see the [Cassandra-to-Bigtable migration tools](../../cassandra-bigtable-migration-tools) in this repository. |
| **Aerospike to Bigtable** | **Coming soon** | Migration skill in active development. In the meantime, see the [Aerospike-to-Bigtable migration tools](../../aerospike-bigtable-migration-tools) in this repository. |

## Prerequisites

Before using the migration skill, confirm that you have the following tools installed and authenticated on your machine:

*   **Google Cloud SDK (`gcloud`) and `cbt` CLI** (`gcloud components install cbt`), authenticated to your Google Cloud environment:
    ```bash
    gcloud auth login
    gcloud auth application-default login
    ```
*   **Target Google Cloud APIs enabled** on your destination project: Cloud Bigtable API (`bigtable.googleapis.com`), Cloud Bigtable Admin API (`bigtableadmin.googleapis.com`), Cloud Dataflow API (`dataflow.googleapis.com`), and Cloud Storage API (`storage.googleapis.com`).
*   **Optional (for automated HBase assessment):** Python 3 and access to `hbase shell` (plus the `hadoop` CLI if scanning physical HDFS table sizes) on a node or bastion host with connectivity to your source HBase cluster.

## Installation

Because this skill is included under the root `cloud-bigtable-ecosystem` plugin manifests (`plugin.json`, `.claude-plugin/`, and `gemini-extension.json`), you can install it directly:

### Option 1: Antigravity CLI (`agy`)

You can add the plugin using the Antigravity CLI:

```bash
agy plugin install https://github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem
```

### Option 2: Claude Code

Add the Bigtable marketplace for Claude plugins:

```bash
claude plugin marketplace add GoogleCloudPlatform/cloud-bigtable-ecosystem
```

Install the Claude plugin:

```bash
claude plugin install bigtable@bigtable-marketplace
```

### Option 3: Gemini CLI

```bash
gemini extensions install https://github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem
```

## Migration Workflow

For Apache HBase to Bigtable, the skill guides and orchestrates four phases with mandatory human-in-the-loop (HITL) confirmation before executing any infrastructure or data-movement command:

1.  **Assessment (`references/hbase/assessment.md`):** Supports two discovery modes:
    *   **Automated assessment (script extraction):** Run the read-only [`scripts/hbase/hbase_extractor.py`](scripts/hbase/hbase_extractor.py) script locally against your HBase cluster to extract table schemas, column family garbage collection policies (`TTL`, `VERSIONS`), region split keys, HDFS table sizes, replication peers, and coprocessors into `hbase_assessment_input.json` without exposing cluster credentials to the agent.
    *   **Guided discovery:** Walk through an interactive compatibility checklist when direct script execution is not feasible.
2.  **Planning (`references/hbase/planning.md`):** Parses `hbase_assessment_input.json` via [`scripts/hbase/hbase_json_parser.py`](scripts/hbase/hbase_json_parser.py) to generate a formal `migration_plan.md` covering coprocessor and custom filter remediation, garbage collection policy translation, target instance provisioning, and `cbt addsplits` pre-splitting commands.
3.  **Data migration (`references/hbase/data_migration.md`):** Step-by-step orchestration for both **offline migration** (`ExportSnapshot` HFiles or `Export` SequenceFiles to Cloud Storage + `bigtable-beam-import` on Cloud Dataflow) and **online zero-downtime migration** (`bigtable-hbase-replication` library + snapshot bootstrap).
4.  **Validation (`references/hbase/validation.md`):** Post-migration row count verification (`RowCounter` vs. `cbt count` with expired-cell garbage collection filter handling) and byte-level checksum verification (`HashTable` + Cloud Dataflow `sync-table`).

### Running the read-only HBase extractor manually (optional)

```bash
# Run against all HBase tables
python3 skills/bigtable-migration/scripts/hbase/hbase_extractor.py

# Scope to specific tables or a regex pattern
python3 skills/bigtable-migration/scripts/hbase/hbase_extractor.py --include-tables "table1,table2"
python3 skills/bigtable-migration/scripts/hbase/hbase_extractor.py --table-regex "^prod_.*"
```

## Directory Structure

```text
skills/bigtable-migration/
├── SKILL.md                                         # Skill entrypoint & router
├── README.md                                        # Overview, installation & usage guide
├── LICENSE.txt                                      # Apache 2.0 License
├── references/
│   └── hbase/                                       # Apache HBase migration runbooks
│       ├── hbase_migration.md                       # 4-phase HBase router & core insights
│       ├── assessment.md                            # Phase 1: Cluster discovery & compatibility checks
│       ├── planning.md                              # Phase 2: Target schema, GC mapping & migration plan
│       ├── data_migration.md                        # Phase 3: Offline (Snapshot/Dataflow) & Online (Replication)
│       └── validation.md                            # Phase 4: RowCounter, HashTable & SyncTable verification
└── scripts/
    └── hbase/                                       # Apache HBase helper scripts & schemas
        ├── hbase_extractor.py                       # Read-only HBase shell/HDFS metadata extractor
        ├── hbase_json_parser.py                     # Assessment JSON parser helper
        └── hbase_assessment_input_example.json      # Example assessment JSON payload
```

## Sample Prompts

*   *"Help me plan and execute a migration from Apache HBase to Cloud Bigtable."*
*   *"I ran `hbase_extractor.py` and have `hbase_assessment_input.json`. Analyze it and generate a `migration_plan.md` for Bigtable."*
*   *"Walk me through setting up a zero-downtime online migration from HBase 2.x to Bigtable using the replication library and `ExportSnapshot`."*
*   *"How do I validate data integrity between HBase and Bigtable using `HashTable` and `sync-table` without false-positive mismatches?"*
