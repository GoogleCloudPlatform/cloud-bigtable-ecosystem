---
name: bigtable
description: Manage Google Bigtable instances/tables, design schemas, and query data using SQL or client libraries. Use for provisioning, schema updates, or code generation.
---

# Bigtable Skill

This skill provides core workflows and guidance for administering and developing with Google Bigtable. 

## Core Principles

- **Control Plane vs. Data Plane:**
  - Use **`gcloud`** for Control Plane operations: Instances, Clusters, App Profiles, Backups, and IAM.
  - Use **`cbt`** for Data Plane operations: Tables, Column Families, and reading/writing data.
- **Performance First:** Bigtable is a NoSQL database. Efficiency is tied to Row Key design. Always warn about Full Table Scans.
- **Client Selection:** For production use cases, **Java** or **Go** are preferred for their superior performance and feature coverage compared to other languages.
- **Observability:** When diagnosing performance or hotspotting, **ALWAYS** mention **Key Visualizer** (via Cloud Console) as the primary diagnostic tool.

> [!IMPORTANT]
> **Safety Rule:** Always obtain explicit user confirmation before making non-emulator database changes.

## Quick Recipes

### 1. Querying Data
Use SQL for complex transforms or aggregations and key-value APIs for simpler query patterns.
*Note: Use exact match, prefix (_key LIKE 'myprefix%') or range predicates on `_key` to avoid expensive unbounded scans.*
If expensive scans (either unbounded or prefix or range queries scanning a large range) are unavoidable due to multiple access patterns that can’t all be accommodated in a single schema, consider one of these two options:
- If the query  will be used in user facing and/or latency sensitive applications, use continuous materialized views with keys optimized for the additional access patterns.
- If secondary access patterns are infrequent, batch patterns like ETL, ML model training or analytical read-only tasks, use Bigtable Data Boost instead. 


### 2. Diagnosing Hotspotting
1. **Visual:** Recommend Key Visualizer in the Cloud Console.
2. **CLI:** List hot tablets for immediate hotspots:
   ```bash
   gcloud bigtable hot-tablets list ${BIGTABLE_CLUSTER} --instance=${BIGTABLE_INSTANCE}
   ```

### 3. Schema Metadata
Quickly inspect table structure:
```bash
cbt ls [TABLE_NAME]
```

### 4. Point Lookup
Read all data for a single row efficiently:
```bash
cbt lookup [TABLE_NAME] [ROW_KEY]
```
*Note: Use `lookup` instead of `read` when the Row Key is known for maximum efficiency.*


## Reference Guides

- **CLI Operations**:
  - [infrastructure_management.md](references/infrastructure_management.md) - Provisioning instances, clusters, and table schemas.
  - [cli_data_access.md](references/cli_data_access.md) - Reading and writing data via the `cbt` CLI.
- **Design & Discovery**:
  - [schema_design.md](references/schema_design.md) - Best practices for row keys and performance with tables and continuous materialized views.
  - [dataplex.md](references/dataplex.md) - Data catalog search for Bigtable assets.
- **Querying & Code**:
  - [sql_guide.md](references/sql_guide.md) - Querying structured row keys via SQL and CLI.
  - [client_libraries.md](references/client_libraries.md) - Patterns for high-performance Go/Java/Python code.

## Common Workflows

### Schema Evolution (DevOps)
1. **Prefer Terraform** for production schema changes to prevent accidental data loss.
2. For manual `cbt` changes, verify column family GC policies:
   ```bash
   cbt createfamily [TABLE] [FAMILY]
   cbt setgcpolicy [TABLE] [FAMILY] "maxversions=5 AND maxage=30d"
   ```
3. Reference [infrastructure_management.md](references/infrastructure_management.md) for full syntax.
