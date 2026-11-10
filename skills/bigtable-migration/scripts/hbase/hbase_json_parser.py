#!/usr/bin/env python3
# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""HBase Assessment JSON Parser Tool for Bigtable Migration Agent."""

import argparse
import json
import logging
import sys
from typing import Any, Dict


def parse_hbase_assessment_json(json_string: str) -> Dict[str, Any]:
  """Parses the HBase assessment JSON string and returns a structured summary.

  Args:
    json_string: A JSON string containing the HBase assessment data.

  Returns:
    A dictionary containing the parsed summary with the following schema:
      status: "success" or "error"
      message: Error message (only present if status is "error")
      hosting: Hosting platform name
      total_tables: Total number of tables analyzed
      replication_peers: List of configured HBase replication peers
      unsupported_features: List of unsupported features detected
      tables_analyzed: List of dicts, each containing:
        table_name: Name of the table
        column_family_count: Number of column families
        column_families: Dict of column family policies
        coprocessor_count: Number of coprocessors
        coprocessors: List of coprocessors
        row_count_estimate: Row count estimate or command
        table_size_bytes: Table size in bytes
        row_counter_command: Command to run exact row count
        region_count: Number of regions
        split_keys_sample: Sample of split keys
        naming_warnings: List of naming warnings
  """
  logging.debug("[Tool Invocation] parse_hbase_assessment_json called.")
  try:
    data = json.loads(json_string)
    if not isinstance(data, dict) or not isinstance(data.get("tables"), dict):
      return {
          "status": "error",
          "message": (
              "Invalid JSON format: Missing or non-dict top-level 'tables'"
              " dictionary."
          ),
      }

    parsed_summary: Dict[str, Any] = {
        "status": "success",
        "hosting": data.get("hosting", "Unknown"),
        "total_tables": len(data["tables"]),
        "replication_peers": data.get("replication_peers", []),
        "unsupported_features": data.get("unsupported_features_detected", []),
        "tables_analyzed": [],
    }

    for table_name, table_data in data["tables"].items():
      if not isinstance(table_data, dict):
        continue
      column_families = table_data.get("column_families", {})
      coprocessors = table_data.get("coprocessors", [])
      stats = table_data.get("stats", {})
      regional_setup = table_data.get("regional_setup", {})
      naming_warnings = table_data.get("naming_warnings", [])

      table_summary = {
          "table_name": table_name,
          "column_family_count": len(column_families),
          "column_families": column_families,
          "coprocessor_count": len(coprocessors),
          "coprocessors": coprocessors,
          "row_count_estimate": stats.get("row_count_estimate", "Unknown"),
          "table_size_bytes": stats.get("table_size_bytes", "Unknown"),
          "row_counter_command": stats.get("row_counter_command", ""),
          "region_count": regional_setup.get("region_count", "Unknown"),
          "split_keys_sample": regional_setup.get("split_keys_sample", []),
          "naming_warnings": naming_warnings,
      }
      parsed_summary["tables_analyzed"].append(table_summary)

    return parsed_summary

  except json.JSONDecodeError as e:
    return {
        "status": "error",
        "message": f"Failed to parse JSON: {e}",
    }
  except Exception as e:  # pylint: disable=broad-except
    return {
        "status": "error",
        "message": f"An unexpected error occurred during parsing: {e}",
    }


def main() -> None:
  parser = argparse.ArgumentParser(
      description="Parse an hbase_assessment_input.json file or stdin payload."
  )
  parser.add_argument(
      "input_file",
      nargs="?",
      help="Path to hbase_assessment_input.json (reads from stdin if omitted)",
  )
  args = parser.parse_args()

  if args.input_file:
    with open(args.input_file, "r", encoding="utf-8") as f:
      raw_json = f.read()
  else:
    raw_json = sys.stdin.read()

  result = parse_hbase_assessment_json(raw_json)
  sys.stdout.write(json.dumps(result, indent=2) + "\n")
  if result.get("status") != "success":
    sys.exit(1)


if __name__ == "__main__":
  main()
