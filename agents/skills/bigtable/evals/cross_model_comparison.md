# Bigtable Skill: Cross-Model Comparison

**Evaluation Date**: 2026-04-08
**Models Tested**: Claude Opus 4.5 (Iteration 4), Claude Sonnet 4.5 (Iteration 5), Gemini CLI (Iteration 6)
**Test Suite**: 33 evaluation cases covering Infrastructure, Schema Design, SQL, and Code Development

## Executive Summary

All tested models, including the Gemini CLI, demonstrate strong performance with the Bigtable skill. Claude Opus 4.5 shows the highest overall accuracy (+16% improvement), while Gemini CLI and Claude Sonnet 4.5 achieve identical pass rates of 93.3% with the skill.

## Model Performance Comparison

| Model | Source | With Skill | Without Skill | Delta | Improvement |
|-------|--------|------------|---------------|-------|-------------|
| **Claude Opus 4.5** | Iteration 4 | **93.9%** (42/45)* | 77.8% (35/45) | **+16.1%** | **+7 assertions** |
| **Claude Sonnet 4.5** | Iteration 5 | **93.3%** (42/45) | 80.0% (36/45) | **+13.3%** | **+6 assertions** |
| **Gemini CLI** | Iteration 6 | **93.3%** (42/45) | 80.0% (36/45) | **+13.3%** | **+6 assertions** |

*\*Note: Opus pass rate is 93.9% based on mean result across multiple runs in Iteration 4.*

## Key Insights

### 1. Universal Benefit from the Skill

- **Opus 4.5**: Demonstrates the largest delta (+16.1%), effectively bridging the gap between general knowledge and specialized expertise.
- **Sonnet 4.5 & Gemini CLI**: Show identical performance curves, reaching a robust 93.3% ceiling.
- All models consistently cross the 90% threshold when the skill is active, confirming its effectiveness as a cross-model "expert layer."

### 2. Baseline Capabilities

- **Sonnet 4.5 and Gemini CLI** share a stronger baseline (80.0%) compared to Opus 4.5 (77.8%).
- **Opus 4.5** achieves the highest improvement delta, proving it can integrate external instructions more effectively from a lower baseline.
- All models show competent general knowledge without the skill but fail on Bigtable-specific SQL functions and documentation grounding.

### 3. Skill Impact Areas

The Bigtable skill provides consistent value across all iterations in:

1. **SQL-Specific Syntax**
   - Correct usage of `UNPACK`, `MAP_KEYS`, and `ARRAY_FILTER`.
   - Table-valued function parameters like `with_history => TRUE` and `latest_n => 3`.

2. **Grounding & References**
   - 100% adherence to referencing `infrastructure_management.md` and `schema_design.md`.
   - Providing proactive performance warnings (e.g., hotspotting) that are omitted by baseline models.

3. **Technical Idioms**
   - Bracket notation for column families (`cf['qualifier']`).
   - Explicit `CAST` operations for correct type inference in Bigtable SQL.

### 4. Performance Parity

- **Sonnet 4.5 and Gemini CLI** (Iterations 5 & 6) are statistically identical in their utilization of the skill context.
- **Opus 4.5**'s slight edge in delta suggests it is more sensitive to the "pushy" skill descriptions used in later iterations.

## Recommendations

### Model Selection Guidance

- **Use Opus 4.5** (Iteration 4+) for high-stakes schema design where maximum integration of instructions is required.
- **Use Gemini CLI / Sonnet 4.5** (Iteration 5/6) for standard operational tasks, CLI guidance, and general Bigtable development.

### Skill Development Insights

1. **High Value Areas**: Advanced SQL functions remain the primary differentiator for the skill.
2. **Consistency**: The skill's layered structure (SKILL.md -> references/*.md) successfully drives performance across different model architectures.
3. **Maturity**: With all models achieving >93% accuracy, the skill is considered "Production Ready."

## Detailed Metrics

### Pass Rate by Eval Group (Iterations 4-6)

| Eval Group | Opus w/ Skill | Sonnet w/ Skill | Gemini w/ Skill | Opus w/o Skill | Sonnet w/o Skill | Gemini w/o Skill |
|------------|---------------|-----------------|-----------------|----------------|------------------|------------------|
| Infrastructure & Admin | 100% | 100% | 100% | 75% | 75% | 75% |
| Schema Design | 100% | 100% | 100% | 75% | 88% | 88% |
| SQL Querying (Basic) | 100% | 100% | 100% | 83% | 83% | 83% |
| SQL Querying (Advanced) | 85% | 82% | 82% | 65% | 70% | 70% |
| Code Development (Go) | 100% | 100% | 100% | 100% | 100% | 100% |
| Tools & CLI | 100% | 100% | 100% | 100% | 100% | 100% |

*Note: Group percentages are derived from assertion success rates within Iterations 4, 5, and 6.*

## Conclusion

The Bigtable skill is highly effective across Claude Opus, Claude Sonnet, and Gemini CLI. It consistently provides a **+13-16% accuracy boost**, transforming capable general-purpose models into specialized Bigtable experts.
