# Bigtable Skill Evaluation: Claude Sonnet

**Date**: 2026-04-10
**Iteration**: iteration-9
**Evals Run**: 50 test cases

## Summary

| Model | Iteration | With Skill | Without Skill | Delta | Suite Size |
|-------|-----------|------------|---------------|-------|------------|
| **Claude Sonnet 4.6** | 9 | **92%** (46/50) | 76% (38/50) | **+16%** | 50 Evals |
| Claude Sonnet 4.5 | 5 | 93% (42/45) | 80% (36/45) | +13% | 33 Evals |

## Key Findings (Iteration 9 — Sonnet 4.6)

1. **Expanded Suite**: The test suite grew from 33 to 50 cases, adding harder SQL, schema design, and DevOps scenarios. Despite the broader coverage, Sonnet 4.6 maintained strong with-skill performance at **92%**.

2. **Skill Improvement**: The Bigtable skill provides a **+16% improvement** over baseline — up from +13% on Sonnet 4.5 — due to the expanded SQL and infrastructure cases where baseline performance drops further.

3. **Areas Where Skill Excels**:
   - Bigtable SQL TVF syntax: `with_history`, `after_or_equal`, `before`, `as_of` flags
   - Bracket notation for column family map access (`cf['column']`)
   - Reference documentation grounding (`sql_guide.md`, `infrastructure_management.md`, `schema_design.md`)
   - Hotspot diagnosis via `gcloud bigtable hot-tablets list` + Key Visualizer
   - Timestamp precision rules in Go client library code

4. **Notable Patterns**:
   - Both configurations perform well on basic CLI, CAST types, and row key design
   - Skill most impactful on advanced SQL functions and documentation adherence
   - Baseline model uses `_timestamp` in WHERE instead of TVF range flags (common failure mode)

## Pass Rate by Eval Group (Iteration 9)

| Eval Group | With Skill | Without Skill | Delta |
|------------|------------|---------------|-------|
| Infrastructure & Admin | 86% | 71% | +15% |
| Schema Design | 100% | 50% | +50% |
| Structured Row Keys | 100% | 100% | +0% |
| Logical Views | 100% | 100% | +0% |
| Data Ingestion (Writes) | 100% | 100% | +0% |
| Data Retrieval (Reads) | 100% | 100% | +0% |
| SQL Querying (Basic) | 83% | 67% | +16% |
| SQL Querying (Advanced) | 67% | 33% | +34% |
| SQL Querying (CAST Types) | 100% | 100% | +0% |
| Tools & CLI | 100% | 75% | +25% |
| Code Development | 100% | 83% | +17% |
| Design & Discovery | 100% | 100% | +0% |

## Detailed Results

Full benchmark data available in: `agents/skills/bigtable-workspace/iteration-9/benchmark.json`
