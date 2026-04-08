# Bigtable Skill Evaluation Guide

This directory contains the test definitions and performance history for the `bigtable` skill.

## 1. Prerequisites

The evaluation infrastructure (`skill-creator`) is NOT committed to this repository. To run or analyze evals, you must first install the `skill-creator` toolset:

1.  **Download/Clone**: Get the `skill-creator` directory from the [Anthropic Skills GitHub](https://github.com/anthropics/skills/tree/main/skills/skill-creator).
2.  **Placement**: Place the folder at `agents/skills/skill-creator/`.
3.  **Dependencies**: Ensure you have **Python 3.9+** and the **Gemini CLI** installed.

## 2. Running Evaluations

The evaluation process is automated by the `skill-creator` agent.

### **How to Start**
Ask a Gemini CLI agent (with access to the `skill-creator` skill) the following:
> "Help me run evals for the bigtable skill located at agents/skills/bigtable"

The agent will:
1.  Read `evals/evals.json`.
2.  Spawn parallel subagents to test the skill vs. a baseline.
3.  Store results in `agents/skills/bigtable-workspace/`.

## 3. Manual Analysis (Post-Run)

If you have already run the evals and want to re-generate the reports:

### **Aggregate Statistics**
```bash
python3 agents/skills/skill-creator/scripts/aggregate_benchmark.py \
  agents/skills/bigtable-workspace/iteration-1 \
  --skill-name bigtable
```

### **Generate Side-by-Side Review**
```bash
python3 agents/skills/skill-creator/eval-viewer/generate_review.py \
  agents/skills/bigtable-workspace/iteration-1 \
  --skill-name "bigtable" \
  --benchmark agents/skills/bigtable-workspace/iteration-1/benchmark.json \
  --static agents/skills/bigtable-workspace/iteration-1/review.html
```

## 4. Repository Content

- `evals.json`: The "Source of Truth" for test prompts and success assertions.
- `schemas.md`: Documentation for the JSON formats and report mapping.
- `gemini_benchmark_summary.md`: Performance snapshot for Gemini 3 Pro.
- `claude_benchmark_summary.md`: Performance snapshot for Claude Opus.
- `sonnet_benchmark_summary.md`: Performance snapshot for Claude Sonnet.
- `cross_model_comparison.md`: The Quantitative Leaderboard. Compares pass rates and deltas across all models (Opus, Sonnet, Gemini 3 Pro).
- `cross_model_report.md`: The Qualitative Strategy Doc. Deep dive into model-specific behaviors, strengths, and failure modes.
- `README.md`: This guide.
