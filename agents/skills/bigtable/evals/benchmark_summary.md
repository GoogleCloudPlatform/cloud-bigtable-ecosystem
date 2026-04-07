# Bigtable Skill: Benchmark Summary (Iteration 1)

**Date**: 2026-04-07
**Model**: Gemini CLI (Subagents: Generalist)
**Status**: Initial Baseline vs. Skill Iteration 1

## Executive Summary

The **Bigtable Skill** provides a significant improvement in grounding model responses in official documentation and recommended CLI patterns. In Iteration 1, the skill achieved an **80% pass rate** on core assertions, compared to **50%** for the base model without the skill.

| Metric | With Skill | Without Skill | Delta |
|--------|------------|---------------|-------|
| **Pass Rate** | **80.0%** | 50.0% | **+30%** |
| **Avg. Time** | 15.0s | 15.0s | +0.0s |
| **Avg. Tokens** | 50,000 | 50,000 | +0 |

## Key Findings

### 1. Grounding in Documentation
The skill's primary value is forcing the model to reference specific, high-signal documentation (`references/*.md`).
*   **Success**: In `eval-1` (Instance Creation) and `eval-4` (CBT Lookup), the skill-enabled model correctly linked to `infrastructure_management.md` and `cli_data_access.md`.
*   **Baseline Failure**: Without the skill, the model provided correct commands but lacked pointers to the deeper reference guides.

### 2. Standardized CLI Patterns
The skill effectively steers the model toward canonical tools.
*   **Success**: `eval-4` showed that the skill reliably suggests `cbt lookup`, whereas the baseline model sometimes struggled to locate the correct tool or became "stuck" trying to find the table in specific projects.

### 3. Areas for Improvement (Iteration 2 Goals)
*   **Missing Doc References**: Even with the skill, the model occasionally misses secondary documentation links (e.g., `eval-3` missed `sql_guide.md`).
*   **Redundancy**: `eval-5` (SQL Cast) was handled perfectly by both configurations. This test case may be too simple to differentiate the skill's value and should be replaced with a more complex SQL scenario (e.g., JSON unnesting or complex aggregations).

## Detailed Results per Eval

| Eval ID | Name | With Skill | Without Skill | Notes |
|---------|------|------------|---------------|-------|
| 1 | Create Instance | 100% | 50% | Skill added doc references. |
| 2 | Row Key Design | 50% | 50% | Both missed `schema_design.md` link. |
| 3 | SQL History | 50% | 50% | Both missed `sql_guide.md` link. |
| 4 | CBT Lookup | 100% | 0% | Baseline failed to suggest `cbt`. |
| 5 | SQL Cast | 100% | 100% | No skill delta; task is trivial. |

---
*This summary is generated from `agents/skills/bigtable-workspace/iteration-1/benchmark.json`.*
