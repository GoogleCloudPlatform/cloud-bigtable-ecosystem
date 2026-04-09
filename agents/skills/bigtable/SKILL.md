---
name: bigtable
description: Manage Google Bigtable instances/tables, design schemas, and query data using SQL or client libraries. Use for provisioning, schema updates, or code generation.
---

# Bigtable Skill

This skill provides core workflows and guidance for administering and developing with Google Bigtable. For detailed command patterns and in-depth guides, see the [References](#references) section.

## Prerequisites

- `gcloud` and `cbt` CLIs installed and authenticated.
- A Google Cloud Project ID and Bigtable Instance ID.
- Access to the `bigtable` skill scripts for SQL execution.

> [!IMPORTANT]
> **Safety Rule:** Always obtain explicit user confirmation before making non-emulator database changes.

## Querying and Development

- **Access Patterns:**
  - Use **Client Libraries** for writes, deletes, and simple reads (point lookups, scans). See [client_libraries.md](references/client_libraries.md).
  - Use **SQL API** for server-side processing (JSON parsing, transforms, aggregations). See [sql_guide.md](references/sql_guide.md).
- **Executing SQL:** Use the `cbt sql` command to execute queries directly:
  ```bash
  cbt sql "[SQL_QUERY]"
  ```

## Reference Guides

Access detailed guidance through these functional reference documents:

- **CLI Operations**:
  - [infrastructure_management.md](references/infrastructure_management.md) - Provisioning instances, clusters, and table schemas.
  - [cli_data_access.md](references/cli_data_access.md) - Reading and writing data via the `cbt` CLI (debugging).
- **Design & Discovery**:
  - [schema_design.md](references/schema_design.md) - Best practices for row keys and performance.
  - [dataplex.md](references/dataplex.md) - Data catalog search for Bigtable assets.
- **Querying & Code**:
  - [sql_guide.md](references/sql_guide.md) - Nuances of Bigtable's SQL dialect.
  - [client_libraries.md](references/client_libraries.md) - Patterns for high-performance client code.

## Common Workflows

### 1. Local Development
1. Start the emulator: `gcloud beta emulators bigtable start` (See [infrastructure_management.md](references/infrastructure_management.md)).
2. Design and create tables (See [schema_design.md](references/schema_design.md) and [infrastructure_management.md](references/infrastructure_management.md)).
3. Test application code against the local emulator.

### 2. Schema Evolution (DevOps)
1. **Always use Terraform** for production schema changes to prevent accidental data loss.
2. Calculate deltas with `terraform plan` before applying.
3. Verify IAM policies and column family configurations via [infrastructure_management.md](references/infrastructure_management.md).
