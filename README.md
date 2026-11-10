# Bigtable Ecosystem

This repository serves as a central hub for resources related to Google Cloud Bigtable, providing links to various tools, libraries, and documentation that contribute to the Bigtable Ecosystem.

## Contents

*   [AI Agent Skills](#ai-agent-skills)
*   [Migration Tools](#migration-tools)
*   [Other Utilities](#other-utilities)

## AI Agent Skills

*   **[Bigtable Skills](./skills/bigtable)** - A collection of skills and reference materials that enable AI agents (such as Antigravity, Gemini, Claude, and Cursor) to assist with Bigtable-related tasks, including schema design, SQL querying, and infrastructure management.

*   **[Bigtable Migration Skill (Preview)](./skills/bigtable-migration)** - Enables AI agents (such as Antigravity, Gemini, Claude, and Cursor) to streamline the database migration journey from Apache HBase to Bigtable — leveraging tools on your local machine to assess source clusters, plan target schemas and capacity, orchestrate offline or online data migration, and validate data parity. Support for Cassandra to Bigtable and Aerospike to Bigtable migrations is coming soon.

### Installation

#### Antigravity CLI (agy) Installation

```bash
agy plugin install https://github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem
```

#### Gemini CLI Installation

Gemini CLI extensions are installed directly from their remote GitHub repositories.

```bash
gemini extensions install https://github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem
```

#### Claude Code Installation

Claude Code utilizes a marketplace system for plugins.

```bash
# Step 1. Add the Bigtable marketplace
claude plugin marketplace add GoogleCloudPlatform/cloud-bigtable-ecosystem

# Step 2. Install the Bigtable plugin
claude plugin install bigtable@bigtable-marketplace
```

#### Codex Installation

```bash
git clone https://github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem.git
cd cloud-bigtable-ecosystem

# Step 2. Open the plugin manager interface
codex
/plugins
# Browse & install plugins from available marketplaces.
```

#### Cursor Installation

See [Cursor Guide](https://cursor.com/docs/skills#installing-skills-from-github)

#### Agent Plugins–compatible clients

This repository is a valid [Agent Plugins](https://github.com/agentplugins/agent-plugins-spec) (v1) plugin. Any [compatible client](https://agent-plugins.org/compatible-clients) (VS Code, Cursor, GitHub Copilot, Codex, Kiro, …) can install it directly using its own built-in plugin command — skills and MCP server included — by pointing at this repository:

```
https://github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem
```

See your agent's documentation for its exact install command.

## Migration Tools

*   **[Bigtable Migration Skill (Preview)](./skills/bigtable-migration)** - Enables AI agents to streamline the database migration journey from Apache HBase to Bigtable across cluster assessment (`hbase_extractor.py`), schema and capacity planning, offline or online data migration, and post-migration validation.

*   **[Cassandra-Bigtable Adapter](https://github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/tree/main/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy)** - This Proxy adapter allows existing Cassandra-based applications to connect seamlessly to Bigtable. This adapter functions as a wire-compatible Cassandra interface, enabling interaction via CQL with minimal configuration. It can be deployed on the same compute as your application or standalone.

*   **[Cassandra Bigtable Java Client](https://github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/tree/main/cassandra-bigtable-migration-tools/cassandra-bigtable-java-client)** - The Cassandra Bigtable Java Client allows your Java applications using Apache Cassandra, to connect seamlessly to a Bigtable instance.

*   **[DynamoDB to Bigtable Migration Tool](https://github.com/GoogleCloudPlatform/professional-services/tree/main/tools/dynamodb-bigtable-migration#bigtable-data-bridge---dynamodb-to-bigtable-migration-utility)** - The DynamoDB to Bigtable Migration tool is a powerful solution designed to streamline data transfer from DynamoDB to Bigtable. This tool automates schema translation, ensuring your data structure is mapped to Bigtable. It also provides options to accelerate and scale data transfer efficiently using Dataflow, minimizing downtime and maximizing performance.

*  **[Bigtable HBase Replication Library](https://github.com/googleapis/java-bigtable-hbase/tree/main/hbase-migration-tools/bigtable-hbase-replication)** - Facilitate near-zero downtime migrations from HBase to Bigtable by enabling to keep your Bigtable instance in sync with your production HBase cluster. Adding Bigtable as an HBase replica guarantees that mutations are applied to Bigtable in the same order as on HBase.

## Other Utilities

* **[Kafka Connect Bigtable Sink](https://github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/tree/main/kafka-connect-bigtable-sink)** - This repository contains the source code a Kafka Connect sink connector for Bigtable. This tool enables the streaming of data records from Apache Kafka topics directly into Bigtable tables.

* **[Cassandra to Bigtable Dataflow Template](https://github.com/GoogleCloudPlatform/DataflowTemplates/blob/main/v1/README_Cassandra_To_Cloud_Bigtable.md)** - The Cassandra to Bigtable Dataflow template copies a table from Cassandra to Bigtable. This template requires minimal configuration and replicates the table structure in Cassandra as closely as possible in Bigtable.
  
*  **[HBase Sequence Files to Bigtable using Dataflow](https://github.com/googleapis/java-bigtable-hbase/blob/v2.15.0/bigtable-dataflow-parent/bigtable-beam-import/README.md)** - This folder contains tools to support importing and exporting HBase data to Bigtable using Dataflow and Apache beam.
