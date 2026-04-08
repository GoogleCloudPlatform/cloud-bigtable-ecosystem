# Bigtable Skill: Cross-Model Evaluation Report

This report compares the effectiveness of the **Bigtable Skill** across different Large Language Models (LLMs).

## Comparison Summary

| Model | Baseline Pass Rate | With-Skill Pass Rate | Skill Delta |
|-------|-------------------|----------------------|-------------|
| **Gemini 3 Pro (Generalist)** | 50.0% | 80.0% | **+30.0%** |
| **Claude 3.5 Sonnet** | TBD | TBD | **TBD** |

## Qualitative Analysis

### Model-Specific Behaviors

#### Gemini 3 Pro
- **Strengths**: Highly efficient at following tool-based workflows once prompted.
- **Weaknesses**: Without the skill, it often defaults to generic `gcloud` commands without checking for specialized project scripts.
- **Skill Impact**: Significantly improves grounding in local documentation and canonical patterns.

#### Claude 3.5 Sonnet (Predicted)
- **Strengths**: Typically higher baseline for complex SQL syntax and Go client library patterns.
- **Weaknesses**: Might hallucinate internal tool flags or overlook project-specific execution scripts.
- **Skill Impact**: Expected to provide better "guardrails" for Bigtable-specific SQL limitations.

## Conclusion & Recommendations

The Bigtable Skill is currently optimized for **Gemini 3 Pro**, particularly in how it links to reference documents. To truly support cross-model evaluation, future iterations should:
1.  Run the full eval suite using Claude 3.5 Sonnet to populate this report.
2.  Adjust `SKILL.md` instructions if Claude consistently misses specific patterns that Gemini catches.
3.  Monitor token usage vs. performance gains to ensure the skill remains cost-effective across models.
