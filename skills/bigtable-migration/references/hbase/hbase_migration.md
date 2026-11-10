# HBase to Bigtable Migration Framework

## Overview

This skill serves as the primary technical interface, router, and architectural
layer for AI agents assisting with migrations from Apache HBase to Google Cloud
Bigtable. It encapsulates the end-to-end migration lifecycle and routes to
specialized technical sub-skills for deep execution.

This sub-skill guides you through the multi-phase migration lifecycle.

### The Multi-Phase Migration Journey

Migrations are supported across four core operational phases:

1.  **Assessment**: Initial cluster and schema analysis.
2.  **Planning**: Data-driven target schema design and capacity planning.
3.  **Migration**: Data export and import (Online or Offline).
4.  **Validation**: Post-migration integrity verification.

*   **Pathway A: Automated Assessment (Script Extraction)**: Run the packaged
    [hbase_extractor.py](../../scripts/hbase/hbase_extractor.py) script on a
    machine with `hbase` shell access to automatically extract schemas.
*   **Pathway B: Guided Discovery Session**: Walk through the checklists
    manually if script extraction is not feasible.

### Interaction & Intent Routing Guidance

When a user requests to start, plan, or execute an HBase migration:

1.  **Start Broad**: Offer a high-level summary of the 4-phase journey
    (Assessment, Planning, Migration, Validation) without listing technical
    limits immediately.
2.  **Detect Intent**:
    *   **Guided/Instructional Intent** (e.g., "Guide me", "Tell me how", "How
        do I do it"): Directly launch **Guided Discovery** (Pathway B).
    *   **Automated/Active Execution Intent** (e.g., "Do it for me", "Let's run
        the assessment", "I want an automated scan"): Directly route to
        **Automated Assessment** (Pathway A) and present the local extraction
        script execution steps.
3.  **Ambiguous Intent**: If the user's intent is generic, present both Pathway
    A (Automated Assessment via script) and Pathway B (Guided Discovery Session)
    as equally weighted choices and let them choose.

## Sub-Skills, Triggers & Core Concepts

Route the user or transition the conversation to the appropriate specialized
sub-skill based on their intent and migration phase. Even before opening the
sub-skills, be aware of the following **Core Technical Insights**:

### Phase 1: Initial Assessment (`assessment.md`)

Assists with automated or guided metadata extraction, checking for Bigtable
unsupported features, and table sizing.

*   **Deep Link**:
    [assessment.md](assessment.md)

### Phase 2: Migration Planning (`planning.md`)

Consumes assessment outputs to generate formal Migration Plans
(`migration_plan.md`) covering schema design, salting, Capacity Planning, and
detailed tool/pathway deployment options.

*   **Deep Link**:
    [planning.md](planning.md)

### Phase 3: Data Migration Execution (`data_migration.md`)

Provides highly technical, step-by-step guidance on offline and online bulk data
export and import strategies.

*   **Triggers**: Questions about using the HBase MapReduce `Export` tool, Cloud
    Dataflow templates (`HBase to Bigtable`), Bigtable HBase Replication Library
    for zero-downtime online migration, or pre-splitting tables.
*   **Core Technical Insights**:
    *   **Pre-splitting**: Always use `cbt addsplits` before bulk-importing to
        prevent node hotspotting.
    *   **Gotchas**: Watch out for **Millisecond (HBase) to Microsecond
        (Bigtable)** timestamp translation, and the difference in
        **Deletes-mask-Puts** execution ordering.
*   **Deep Link**:
    [data_migration.md](data_migration.md)

### Phase 4: Validation & Integrity Verification (`validation.md`)

Provides technical procedures, commands, and Gotcha-handling to verify data
integrity and row counts post-migration.

*   **Triggers**: Questions about validating data migration, comparing database
    sizes, or running `cbt count` vs. HBase MapReduce `RowCounter`.
*   **Core Technical Insights**:
    *   **Row Counting**: For large HBase tables, use `RowCounter` instead of
        `count` to avoid timeouts.
    *   **Discrepancy Gotcha**: Bigtable does **not** automatically filter out
        expired data marked for GC during read scans. You must use explicit
        filters during validation to avoid row count mismatches against HBase.
*   **Deep Link**:
    [validation.md](validation.md)
