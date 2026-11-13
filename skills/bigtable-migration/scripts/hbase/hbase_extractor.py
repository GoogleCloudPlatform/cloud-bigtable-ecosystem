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
"""HBase Schema and Metadata Extractor for Bigtable Migration Assessment.

Run this script on a node with access to the `hbase` shell to generate a
standardized `hbase_assessment_input.json` file for the Bigtable Migration
Agent.
"""

import argparse
import ast
import json
import logging
import os
import re
import shlex
import shutil
import subprocess
import sys
from typing import Any, Dict, List, Optional, Tuple

_VALID_HBASE_TABLE_RE = re.compile(r"^[a-zA-Z0-9_.:-]+$")


def print_step(message: str) -> None:
  """Logs a standard progress message."""
  logging.info("[Checking...] %s", message)


def print_success(message: str) -> None:
  """Logs a success message."""
  logging.info("[Success]     %s", message)


def print_warning(message: str) -> None:
  """Logs a warning message."""
  logging.warning("[WARNING]     %s", message)


def run_command_with_timeout(
    cmd: List[str], input_data: Optional[str] = None, timeout: int = 30
) -> Tuple[str, int]:
  """Executes a shell command and returns (stdout, returncode)."""
  try:
    process = subprocess.run(
        cmd,
        input=input_data.encode("utf-8") if input_data else None,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        timeout=timeout,
        check=False,
    )
    if process.returncode != 0:
      err = process.stderr.decode("utf-8", errors="replace").strip()
      error_lines = [
          line
          for line in err.splitlines()
          if line.strip() and not re.search(r"\b(WARN|INFO)\b", line)
      ]
      if error_lines:
        print_warning(f"Command error: {' | '.join(error_lines[:3])}")
    return process.stdout.decode("utf-8", errors="replace"), process.returncode
  except subprocess.TimeoutExpired:
    print_warning(f"Command timed out after {timeout} seconds: {' '.join(cmd)}")
    return "", -1
  except Exception as e:  # pylint: disable=broad-except
    print_warning(f"Failed to run command {' '.join(cmd)}: {e}")
    return "", -1


def run_hbase_shell_command(
    command: str, timeout: int = 30
) -> Tuple[str, int]:
  """Executes an HBase shell command and returns (stdout, returncode)."""
  return run_command_with_timeout(["hbase", "shell", "-n"], command, timeout)


def escape_hbase_ruby_str(value: str) -> str:
  """Escapes a string for safe inclusion inside single-quoted JRuby literals."""
  return value.replace("\\", "\\\\").replace("'", "\\'")


def get_hdfs_table_size(table_name: str, timeout: int = 15) -> str:
  """Attempts to get table size in bytes using Hadoop fs -du."""
  print_step(f"Retrieving HDFS size for '{table_name}'...")
  if ":" in table_name:
    namespace, tbl = table_name.split(":", 1)
  else:
    namespace, tbl = "default", table_name

  dirs = [f"/hbase/data/{namespace}", f"/apps/hbase/data/{namespace}"]

  for base_dir in dirs:
    path = f"{base_dir}/{tbl}"
    output, rc = run_command_with_timeout(
        ["hadoop", "fs", "-du", "-s", path], timeout=timeout
    )
    if rc == 0 and output and "No such file" not in output:
      for raw_line in output.splitlines():
        parts = raw_line.strip().split()
        if parts and parts[0].isdigit() and path in raw_line:
          size = parts[0]
          print_success(f"HDFS size for '{table_name}': {size} bytes")
          return size
      numeric_tokens = [t for t in output.split() if t.isdigit()]
      if numeric_tokens:
        size = numeric_tokens[0]
        print_success(f"HDFS size for '{table_name}': {size} bytes")
        return size

  print_warning(
      f"Could not retrieve HDFS size for '{table_name}' (Paths checked: {dirs})"
  )
  return "Unknown"


def ask_user_yes_no(question: str, default: bool = False) -> bool:
  """Prompts the user with a yes/no question in the terminal."""
  if not sys.stdin.isatty():
    logging.info(
        "Non-interactive session detected; defaulting to '%s' for: %s",
        "yes" if default else "no",
        question,
    )
    return default

  valid = {"yes": True, "y": True, "no": False, "n": False}
  prompt = " [Y/n] " if default else " [y/N] "

  while True:
    sys.stdout.write(question + prompt)
    sys.stdout.flush()
    try:
      choice = input().lower().strip()
    except EOFError:
      return default
    if choice == "":
      return default
    if choice in valid:
      return valid[choice]
    sys.stdout.write("Please respond with 'yes'/'y' or 'no'/'n'.\n")
    sys.stdout.flush()


def parse_describe_attribute(desc_block: str, attribute: str) -> str:
  """Extracts a specific attribute (e.g., TTL, VERSIONS) from an HBase CF block."""
  pattern = (
      r"\b"
      + re.escape(attribute)
      + r"\s*=>\s*(?:'([^']*)'|\"([^\"]*)\"|([^,\s\}]+))"
  )
  match = re.search(pattern, desc_block)
  if match:
    val = next((g for g in match.groups() if g is not None), "")
    return val.strip()
  return "Default/Unknown"


def extract_cf_blocks(desc_output: str) -> List[str]:
  """Extracts column family attribute blocks from HBase `describe` output.

  Handles single-line comma-separated descriptors, multi-line wrapped output,
  and nested HBase 2.x `CONFIGURATION => {...}` / `METADATA => {...}` maps.
  """
  blocks: List[str] = []
  stack: List[List[str]] = []
  in_quote: Optional[str] = None
  escaped = False

  for ch in desc_output:
    if in_quote:
      if stack:
        stack[-1].append(ch)
      if escaped:
        escaped = False
      elif ch == "\\":
        escaped = True
      elif ch == in_quote:
        in_quote = None
      continue

    if ch in ("'", '"'):
      in_quote = ch
      if stack:
        stack[-1].append(ch)
    elif ch == "{":
      stack.append([])
    elif ch == "}":
      if stack:
        candidate = "".join(stack.pop())
        if re.search(r"(?:^|,)\s*NAME\s*=>", candidate):
          blocks.append(candidate)
    else:
      if stack:
        stack[-1].append(ch)

  return blocks


def parse_list_peers_output(peers_output: str) -> List[str]:
  """Extracts actual peer rows from HBase shell `list_peers` output."""
  peers: List[str] = []
  in_peer_table = False
  for raw_line in peers_output.splitlines():
    line = raw_line.strip()
    if not line:
      continue
    if line.startswith("PEER_ID"):
      in_peer_table = True
      continue
    if in_peer_table:
      if re.match(r"^\d+\s+row\(s\)", line) or line.startswith("Took "):
        break
      if line.startswith("=>"):
        continue
      peers.append(line)
  return peers


def parse_splits_output(splits_output: str) -> List[str]:
  """Extracts split keys from HBase shell `get_splits` output."""
  for raw_line in reversed(splits_output.splitlines()):
    line = raw_line.strip()
    if line.startswith("=> [") and "]" in line:
      try:
        array_str = line[line.find("[") : line.rfind("]") + 1]
        parsed = ast.literal_eval(array_str)
        if isinstance(parsed, list):
          return [str(k) for k in parsed if str(k)]
      except Exception:  # pylint: disable=broad-except
        pass

  split_keys: List[str] = []
  capture = False
  for raw_line in splits_output.splitlines():
    line = raw_line.strip()
    if line.startswith("Total number of splits"):
      capture = True
      continue
    if capture:
      if line.startswith(("Took ", "=>")) or re.match(r"^\d+\s+row\(s\)", line):
        break
      if line:
        split_keys.append(line)
  return split_keys


def extract_metadata(
    scan_hdfs: bool,
    include_tables: Optional[List[str]] = None,
    table_regex: Optional[str] = None,
    timeout: int = 30,
) -> Dict[str, Any]:
  """Extracts tables, column families, regional setups, and meta from HBase."""
  metadata: Dict[str, Any] = {
      "hosting": os.environ.get("HBASE_HOSTING_PLATFORM", "Unknown"),
      "replication_peers": [],
      "tables": {},
      "unsupported_features_detected": [],
      "checks_performed": [
          "Coprocessors",
          "Namespaces",
          "Table & Column Family Naming Conventions",
          "Garbage Collection (TTL/VERSIONS)",
          "Replication Peers",
          "Region Distribution & Splits",
      ],
  }

  # 1. Extract Replication Peers
  print_step("Checking HBase Replication Peers...")
  peers_output, peers_rc = run_hbase_shell_command(
      "list_peers", timeout=timeout
  )
  if peers_rc != 0:
    print_warning("Failed to query HBase replication peers via 'list_peers'.")
  else:
    metadata["replication_peers"] = parse_list_peers_output(peers_output)
    if metadata["replication_peers"]:
      print_success(
          f"Found Replication Peers: {len(metadata['replication_peers'])}"
      )
    else:
      print_success("No Replication Peers detected.")

  # 2. List Tables
  print_step("Listing HBase tables...")
  tables_output, list_rc = run_hbase_shell_command("list", timeout=timeout)
  if list_rc != 0:
    print_warning(
        "Failed to connect to HBase or execute 'list' command. Aborting."
    )
    sys.exit(1)

  all_tables: List[str] = []
  parsed_table_list = False
  for raw_line in tables_output.splitlines():
    line = raw_line.strip()
    if line.startswith("=> [") and "]" in line:
      try:
        array_str = line[line.find("[") : line.rfind("]") + 1]
        parsed_tables = ast.literal_eval(array_str)
        if isinstance(parsed_tables, list):
          all_tables = [t for t in parsed_tables if isinstance(t, str)]
          parsed_table_list = True
          break
      except Exception:  # pylint: disable=broad-except
        pass

  if not parsed_table_list:
    in_table_section = False
    for raw_line in tables_output.splitlines():
      line = raw_line.strip()
      if line == "TABLE":
        in_table_section = True
        continue
      if in_table_section:
        if re.match(r"^\d+\s+row\(s\)", line) or line.startswith(
            ("Took ", "=>")
        ):
          break
        if line and _VALID_HBASE_TABLE_RE.match(line):
          all_tables.append(line)

  # Apply Scoping Filters
  if include_tables:
    tables = [t for t in all_tables if t in include_tables]
    print_step(f"Scoping assessment to explicitly included tables: {tables}")
  elif table_regex:
    try:
      compiled_regex = re.compile(table_regex)
      tables = [t for t in all_tables if compiled_regex.search(t)]
      print_step(
          f"Scoping assessment to regex '{table_regex}' matches: {tables}"
      )
    except re.error as e:
      print_warning(
          f"Invalid table regex '{table_regex}': {e}. Using all tables."
      )
      tables = all_tables
  else:
    tables = all_tables
    print_success(f"Found total tables: {len(tables)}")

  # 3. Analyze Each Table
  for table in tables:
    if not _VALID_HBASE_TABLE_RE.match(table):
      print_warning(f"Skipping table with unexpected identifier '{table}'.")
      continue

    escaped_table = escape_hbase_ruby_str(table)
    print_step(f"\nAnalyzing Table: '{table}'")
    desc_output, desc_rc = run_hbase_shell_command(
        f"describe '{escaped_table}'", timeout=timeout
    )

    metadata["tables"][table] = {
        "column_families": {},
        "coprocessors": [],
        "regional_setup": {"region_count": "Unknown", "split_keys_sample": []},
        "stats": {
            "row_count_estimate": "Proxy via Size/Regions",
            "table_size_bytes": "Unknown",
            "row_counter_command": (
                "hbase org.apache.hadoop.hbase.mapreduce.RowCounter "
                f"{shlex.quote(table)}"
            ),
        },
        "naming_warnings": [],
    }

    # Check HBase namespace and Bigtable table ID rules
    if ":" in table:
      ns, bare_tbl = table.split(":", 1)
      if ns != "default":
        ns_warning = (
            f"Table '{table}' uses non-default HBase namespace '{ns}'."
            " Bigtable does not support explicit namespaces and ':' is illegal"
            f" in Bigtable table IDs (rename e.g. to '{ns}_{bare_tbl}' or"
            f" '{ns}__{bare_tbl}', or consolidate with '{ns}#<rowkey>'"
            " prefixes)."
        )
      else:
        ns_warning = (
            f"Table '{table}' includes explicit 'default:' namespace prefix;"
            f" strip 'default:' to '{bare_tbl}' for Bigtable."
        )
      print_warning(ns_warning)
      if ns_warning not in metadata["unsupported_features_detected"]:
        metadata["unsupported_features_detected"].append(ns_warning)
      metadata["tables"][table]["naming_warnings"].append(ns_warning)
      candidate_table_id = f"{ns}_{bare_tbl}" if ns != "default" else bare_tbl
    else:
      candidate_table_id = table

    if (
        not re.fullmatch(r"[_a-zA-Z0-9][-_.a-zA-Z0-9]*", candidate_table_id)
        or len(candidate_table_id) > 50
    ):
      tbl_warning = (
          f"Table ID '{candidate_table_id}' violates Bigtable table ID rules"
          " (regex [_a-zA-Z0-9][-_.a-zA-Z0-9]*, max 50 characters)."
      )
      print_warning(tbl_warning)
      metadata["tables"][table]["naming_warnings"].append(tbl_warning)
      if tbl_warning not in metadata["unsupported_features_detected"]:
        metadata["unsupported_features_detected"].append(tbl_warning)

    if desc_rc != 0:
      desc_err = f"Failed to describe table '{table}'; schema details unavailable."
      print_warning(desc_err)
      metadata["tables"][table]["naming_warnings"].append(desc_err)
      if desc_err not in metadata["unsupported_features_detected"]:
        metadata["unsupported_features_detected"].append(desc_err)
      continue

    # Extract Region Count and Split Keys using 'get_splits' command
    print_step(f"Retrieving Region and Split metadata for '{table}'...")
    splits_output, splits_rc = run_hbase_shell_command(
        f"get_splits '{escaped_table}'", timeout=timeout
    )
    if splits_rc == 0 and splits_output:
      split_keys = parse_splits_output(splits_output)
      region_count = len(split_keys) + 1
      metadata["tables"][table]["regional_setup"]["region_count"] = region_count
      metadata["tables"][table]["regional_setup"]["split_keys_sample"] = (
          split_keys[:10]
      )
      print_success(
          f"Table '{table}' distribution: {region_count} Region(s),"
          f" {len(split_keys)} Split Point(s) found."
      )
    else:
      print_warning(f"Could not retrieve split metadata for '{table}'.")

    # Get HDFS Size
    if scan_hdfs:
      size_bytes = get_hdfs_table_size(table, timeout=max(15, timeout // 2))
      metadata["tables"][table]["stats"]["table_size_bytes"] = size_bytes
    else:
      print_step(f"Skipping HDFS sizing for '{table}' as requested.")

    # Parse column families, GC policies, and storage policies from describe
    reserved_cf_names = {"_key", "_timestamp", "_exists"}
    for cf_block in extract_cf_blocks(desc_output):
      try:
        family = parse_describe_attribute(cf_block, "NAME")
        if family == "Default/Unknown" or not family:
          continue

        ttl = parse_describe_attribute(cf_block, "TTL")
        versions = parse_describe_attribute(cf_block, "VERSIONS")
        compression = parse_describe_attribute(cf_block, "COMPRESSION")
        bloomfilter = parse_describe_attribute(cf_block, "BLOOMFILTER")
        in_memory = parse_describe_attribute(cf_block, "IN_MEMORY")
        blockcache = parse_describe_attribute(cf_block, "BLOCKCACHE")

        metadata["tables"][table]["column_families"][family] = {
            "TTL": ttl,
            "VERSIONS": versions,
            "COMPRESSION": compression,
            "BLOOMFILTER": bloomfilter,
            "IN_MEMORY": in_memory,
            "BLOCKCACHE": blockcache,
        }

        # Bigtable Column Family ID rules: 1-64 chars, regex, no reserved names
        cf_warning: Optional[str] = None
        if (
            not re.fullmatch(r"[_a-zA-Z0-9][-_.a-zA-Z0-9]*", family)
            or len(family) > 64
        ):
          cf_warning = (
              f"Column Family '{family}' on table '{table}' violates Bigtable"
              " rules (regex [_a-zA-Z0-9][-_.a-zA-Z0-9]*, max 64 characters)."
          )
        elif family.lower().startswith("__bti_") or (
            family.lower() in reserved_cf_names
        ):
          cf_warning = (
              f"Column Family '{family}' on table '{table}' uses a reserved"
              " Bigtable system/SQL identifier."
          )
        if cf_warning:
          print_warning(cf_warning)
          metadata["tables"][table]["naming_warnings"].append(cf_warning)
          if cf_warning not in metadata["unsupported_features_detected"]:
            metadata["unsupported_features_detected"].append(cf_warning)
      except Exception:  # pylint: disable=broad-except
        pass

    cp_matches = re.findall(
        r"coprocessor\$\d+\s*=>\s*(?:'[^']*'|\"[^\"]*\"|[^,\}]+)",
        desc_output,
        re.IGNORECASE,
    )
    if cp_matches:
      warning = f"Table '{table}' uses Coprocessors."
      print_warning(warning)
      if warning not in metadata["unsupported_features_detected"]:
        metadata["unsupported_features_detected"].append(warning)
      metadata["tables"][table]["coprocessors"].extend(
          m.strip() for m in cp_matches
      )
    else:
      for raw_line in desc_output.splitlines():
        line = raw_line.strip()
        if "coprocessor" in line.lower():
          warning = f"Table '{table}' uses Coprocessors."
          print_warning(warning)
          if warning not in metadata["unsupported_features_detected"]:
            metadata["unsupported_features_detected"].append(warning)
          metadata["tables"][table]["coprocessors"].append(line)

  return metadata


def generate_human_readable_report(data: Dict[str, Any]) -> str:
  """Generates a thorough human-readable migration assessment report."""
  report = []
  report.append("==================================================")
  report.append("HBase to Cloud Bigtable Migration Assessment Report")
  report.append(f"Hosting Platform: {data.get('hosting', 'Unknown')}")
  report.append("==================================================")

  report.append("\n## Assessment Checks Performed:")
  for check in data.get("checks_performed", []):
    report.append(f" [x] {check}")

  report.append("\n## Global Cluster Topologies:")
  if data.get("replication_peers"):
    report.append(
        " [WARNING] Replication Peers Detected:"
        f" {len(data['replication_peers'])}"
    )
    for p in data["replication_peers"]:
      report.append(f"   - {p}")
  else:
    report.append(
        " [x] No Global Replication Peers detected (Standalone Cluster)."
    )

  if data.get("unsupported_features_detected"):
    report.append("\n## [CRITICAL] Unsupported Features Detected:")
    for feature in data["unsupported_features_detected"]:
      report.append(f" - {feature}")
  else:
    report.append("\n## Supported Features Check: OK")
    report.append(" - No Coprocessors detected.")
    report.append(" - No non-default Namespaces or Naming Violations detected.")
    report.append(
        " - Note: Custom client-side Filters are not stored in HBase table"
        " descriptors and must be audited in application code."
    )

  report.append("\n## Table Details, Sizing, & Policy Analysis:")
  for table, details in data.get("tables", {}).items():
    report.append("\n--------------------------------------------------")
    report.append(f"Table: {table}")

    report.append("  Column Families & Policies:")
    for cf, policies in details.get("column_families", {}).items():
      report.append(f"    - Family: '{cf}'")
      report.append(f"      * TTL: {policies.get('TTL', 'Default')}")
      report.append(f"      * VERSIONS: {policies.get('VERSIONS', 'Default')}")
      report.append(
          f"      * COMPRESSION: {policies.get('COMPRESSION', 'Default')}"
      )
      report.append(
          f"      * BLOOMFILTER: {policies.get('BLOOMFILTER', 'Default')}"
      )
      report.append(
          f"      * IN_MEMORY: {policies.get('IN_MEMORY', 'Default')}"
      )
      report.append(
          f"      * BLOCKCACHE: {policies.get('BLOCKCACHE', 'Default')}"
      )

    if details.get("naming_warnings"):
      report.append("  [WARNING] Naming Violations:")
      for w in details["naming_warnings"]:
        report.append(f"    - {w}")
    else:
      report.append(
          "  Table & Column Family Naming: OK (Matches Bigtable conventions)"
      )

    if details.get("coprocessors"):
      report.append(
          f"  [CRITICAL] Coprocessors: {len(details['coprocessors'])} found."
      )
      report.append(
          "    -> Remediation required: Move server-side logic to the"
          " application layer."
      )
    else:
      report.append("  Coprocessors: None")

    report.append(
        "  Regional Setup:"
        f" {details.get('regional_setup', {}).get('region_count', 'Unknown')}"
        " Region(s)"
    )
    splits_sample = details.get("regional_setup", {}).get(
        "split_keys_sample", []
    )
    if splits_sample:
      report.append(f"    * Sample Split Key: '{splits_sample[0]}'")
      report.append(
          "    -> Action: Pass split keys at table creation time"
          " ('cbt createtable ... splits=...') in Phase 2 (Planning)."
      )

    report.append(
        "  Physical Size (HDFS):"
        f" {details.get('stats', {}).get('table_size_bytes', 'Unknown')} bytes"
    )
    report.append(
        "  Row Count Policy: Proxy via Sizing/Regions (Shell 'count' disabled"
        " for high-scale safety)"
    )
    report.append("    -> MapReduce Command for Exact Count:")
    report.append(
        f"       `{details.get('stats', {}).get('row_counter_command')}`"
    )

  report.append("\n==================================================")
  report.append("End of Assessment Report")
  report.append("==================================================")
  return "\n".join(report)


def main() -> None:
  logging.basicConfig(
      level=logging.INFO, format="%(message)s", stream=sys.stdout
  )
  parser = argparse.ArgumentParser(
      description=(
          "HBase Schema and Metadata Extractor for Bigtable Migration"
          " Assessment"
      )
  )
  parser.add_argument(
      "--include-tables",
      type=str,
      help="Comma-separated list of specific table names to extract",
  )
  parser.add_argument(
      "--table-regex",
      type=str,
      help="Regex pattern to match table names to extract",
  )
  parser.add_argument(
      "--scan-hdfs",
      action="store_true",
      default=None,
      help="Scan physical HDFS table sizes via Hadoop CLI without prompting",
  )
  parser.add_argument(
      "--no-scan-hdfs",
      dest="scan_hdfs",
      action="store_false",
      help="Skip physical HDFS table sizing scan without prompting",
  )
  parser.add_argument(
      "--output-dir",
      type=str,
      default=".",
      help="Directory to write JSON and text assessment outputs (default: .)",
  )
  parser.add_argument(
      "--timeout",
      type=int,
      default=30,
      help="Per-command timeout in seconds for HBase shell queries (default: 30)",
  )
  args = parser.parse_args()

  logging.info("==================================================")
  logging.info("Starting HBase to Bigtable Migration Assessment Extractor")
  logging.info("==================================================")

  if shutil.which("hbase") is None:
    print_warning(
        "HBase CLI ('hbase') was not found in your PATH. Run this script on a"
        " host with HBase client access."
    )
    sys.exit(1)

  logging.info("Value Proposition:")
  logging.info(
      "Scanning physical table sizing via the Hadoop CLI helps Bigtable"
      " capacity planning,"
  )
  logging.info(
      "but requires Hadoop to be configured in your environment. It is optional"
      " for core schema assessment.\n"
  )

  hadoop_installed = shutil.which("hadoop") is not None
  if args.scan_hdfs is not None:
    if args.scan_hdfs and not hadoop_installed:
      print_warning(
          "Hadoop CLI ('hadoop') not detected in PATH; skipping HDFS scan."
      )
      scan_hdfs = False
    else:
      scan_hdfs = args.scan_hdfs
  elif not hadoop_installed:
    print_warning("Hadoop CLI ('hadoop') not detected in your PATH.")
    logging.info(
        "Skipping physical HDFS sizing scan automatically (safe for core"
        " migration planning).\n"
    )
    scan_hdfs = False
  else:
    scan_hdfs = ask_user_yes_no(
        "Do you wish to scan physical HDFS table sizing via the Hadoop CLI?",
        default=False,
    )
    logging.info("")

  parsed_include_tables = None
  if args.include_tables:
    parsed_include_tables = [
        t.strip() for t in args.include_tables.split(",") if t.strip()
    ]

  data = extract_metadata(
      scan_hdfs=scan_hdfs,
      include_tables=parsed_include_tables,
      table_regex=args.table_regex,
      timeout=args.timeout,
  )

  os.makedirs(args.output_dir, exist_ok=True)
  json_output_file = os.path.join(
      args.output_dir, "hbase_assessment_input.json"
  )
  with open(json_output_file, "w", encoding="utf-8") as f:
    json.dump(data, f, indent=2)

  readable_output_file = os.path.join(
      args.output_dir, "hbase_assessment_report.txt"
  )
  report_content = generate_human_readable_report(data)
  with open(readable_output_file, "w", encoding="utf-8") as f:
    f.write(report_content)

  logging.info("\n==================================================")
  logging.info(" Extraction and Assessment Complete")
  logging.info("==================================================")
  logging.info(
      "Saved machine-readable payload for Agent to:"
      f" {os.path.abspath(json_output_file)}"
  )
  logging.info(
      "Saved thorough assessment report for Review to:"
      f" {os.path.abspath(readable_output_file)}"
  )
  logging.info("\nNext Steps:")
  logging.info(
      f"1. Open '{readable_output_file}' to review your cluster assessment and"
      " any critical warnings."
  )
  logging.info(
      f"2. Copy the entire contents of '{json_output_file}' and paste it into"
      " your conversation with the Bigtable Migration Agent to automatically"
      " generate your formal Migration Plan."
  )


if __name__ == "__main__":
  main()
