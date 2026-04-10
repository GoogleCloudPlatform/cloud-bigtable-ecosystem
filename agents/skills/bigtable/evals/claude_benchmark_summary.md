# Bigtable Skill Evaluation: Claude Opus

**Date**: 2026-04-10
**Iteration**: iteration-10
**Evals Run**: 50 test cases

## Summary

| Model | Iteration | With Skill | Without Skill | Delta | Suite Size |
|-------|-----------|------------|---------------|-------|------------|
| **Claude Opus 4.6** | 10 | **100%** (50/50) | 88% (44/50) | **+12%** | 50 Evals |
| Claude Opus 4.5 | 4 | 94% (42/45)* | 78% (35/45) | +16% | 33 Evals |

*\*Opus 4.5 pass rate is based on the 33-eval suite (45 total assertions).*

## Key Findings (Iteration 10 — Opus 4.6)

1. **Perfect With-Skill Score**: Opus 4.6 is the first model to achieve **100% pass rate** (50/50 evals, 68/68 assertions) with the Bigtable skill on the full 50-case suite.

2. **Highest Baseline**: Opus 4.6 has the strongest baseline of any tested model at **88% without-skill** — substantially higher than Sonnet 4.6 (76%) and Gemini 3 Pro (64.7%). This results in a smaller delta (+12%) because the model already knows most Bigtable patterns.

3. **Where the Skill Still Matters**:
   - **Documentation grounding**: 5 of 6 without-skill failures are reference doc assertions (`infrastructure_management.md`, `schema_design.md`, `sql_guide.md`, `cli_data_access.md`, `client_libraries.md`). The model cannot cite skill-specific docs without the skill.
   - **Niche SQL syntax**: Eval 48 (`SAFE_CONVERT_BYTES_TO_STRING` for bytes-to-BOOL conversion) — a Bigtable-specific pattern the baseline model misses.

4. **Notable Patterns**:
   - Opus 4.6 passes all advanced SQL evals (UNPACK, MAP_KEYS, ARRAY_FILTER, as_of, with_history ranges) without the skill — a significant improvement over prior models
   - Baseline handles Go client patterns (CheckAndMutateRow, PrefixRange, StripValueFilter), gcloud infrastructure commands, and Terraform resources correctly
   - The skill transforms responses from "correct" to "production-ready" by adding doc references and best-practice warnings

## Pass Rate by Eval Group (Iteration 10)

| Eval Group | With Skill | Without Skill | Delta |
|------------|------------|---------------|-------|
| Infrastructure & Admin | 100% | 91% | +9% |
| Schema Design | 100% | 75% | +25% |
| Structured Row Keys | 100% | 100% | +0% |
| Logical Views | 100% | 100% | +0% |
| Data Ingestion (Writes) | 100% | 100% | +0% |
| Data Retrieval (Reads) | 100% | 100% | +0% |
| SQL Querying (Basic) | 100% | 83% | +17% |
| SQL Querying (Advanced) | 100% | 94% | +6% |
| SQL Querying (CAST Types) | 100% | 100% | +0% |
| Tools & CLI | 100% | 83% | +17% |
| Code Development | 100% | 83% | +17% |
| Design & Discovery | 100% | 100% | +0% |

## Failed Without-Skill Assertions

| Eval | Group | Failed Assertion | Reason |
|------|-------|-----------------|--------|
| 1 | Infrastructure & Admin | ref_infra_mgmt | No reference to infrastructure_management.md |
| 3 | Schema Design | ref_schema_design | No reference to schema_design.md |
| 14 | SQL Querying (Basic) | ref_sql_guide | No reference to sql_guide.md |
| 30 | Tools & CLI | ref_cli_data_access | No reference to cli_data_access.md |
| 31 | Code Development | ref_client_libs | No reference to client_libraries.md |
| 48 | SQL Advanced | sql_safe_convert_bytes | Used CAST(AS STRING) instead of SAFE_CONVERT_BYTES_TO_STRING |

## Detailed Results

Full benchmark data available in: `agents/skills/bigtable-workspace/iteration-10/`
