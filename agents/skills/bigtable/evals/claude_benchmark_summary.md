# Bigtable Skill: Benchmark Summary (Claude Edition)

**Date**: 2026-04-07
**Model**: Claude Opus 4.5
**Evals Run**: 33 test cases (iteration-4)

## Executive Summary

The Bigtable skill significantly improves Claude's performance on Bigtable-related tasks, achieving a **+16% improvement** in assertion pass rate compared to the baseline (no skill).

| Metric | With Skill | Without Skill | Delta |
|--------|------------|---------------|-------|
| **Pass Rate** | **94%** | 78% | **+16%** |
| **Assertions Passed** | 43/45 | 35/45 | +8 |

## Key Findings

1. **Reference Documentation**: The skill excels at directing Claude to reference the appropriate documentation files (e.g., `infrastructure_management.md`, `schema_design.md`), which the baseline model fails to do.

2. **Bigtable-Specific SQL Functions**: The skill improves accuracy with Bigtable-specific SQL functions like `UNPACK`, `MAP_KEYS`, and `ARRAY_FILTER`. The baseline struggles with these advanced features.

3. **Hotspot Warnings**: With the skill, Claude consistently warns about hotspotting risks for timestamp-based row keys (100% vs 50% baseline).

4. **Go Client Patterns**: Both with and without skill perform well on Go client library patterns, though the skill ensures consistent use of best practices from `client_libraries.md`.

## Areas Where Skill Adds Most Value

| Category | With Skill | Without Skill | Improvement |
|----------|------------|---------------|-------------|
| Infrastructure & Admin | 100% | 75% | +25% |
| Schema Design | 100% | 75% | +25% |
| SQL Querying (Advanced) | 88% | 63% | +25% |
| Tools & CLI | 100% | 50% | +50% |

## Evaluation Methodology

- **33 test prompts** covering Infrastructure, Schema Design, SQL Querying, Data Ingestion/Retrieval, and Code Development
- **Automated assertion checking** for key requirements (correct syntax, reference links, best practices)
- **Side-by-side comparison** of with-skill vs without-skill responses
