# Bigtable Ecosystem

This repository serves as a central hub for resources related to Google Cloud Bigtable, providing links to various tools, libraries, and documentation that contribute to the Bigtable Ecosystem.

## Contents

*   [Migration Tools](#migration-tools)
*   [Other Utilities](#other-utilities)

## Migration Tools

*   **[Cassandra-Bigtable Proxy Adapter](https://github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/tree/main/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy)** - This Proxy Adapter allows existing Cassandra-based applications to connect seamlessly to Bigtable. This adapter functions as a wire-compatible Cassandra interface, enabling interaction via CQL with minimal configuration. It can be deployed on the same compute as your application or standalone.
  
*   **[Bigtable CQL Client for Java](https://github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/tree/main/cassandra-bigtable-migration-tools/cassandra-bigtable-cql-client-java)** - The Bigtable CQL Client for Java allows your Java applications using Apache Cassandra, to connect seamlessly to a Bigtable instance.

*   **[DynamoDB to Bigtable Migration Tool](https://github.com/GoogleCloudPlatform/professional-services/tree/main/tools/dynamodb-bigtable-migration#bigtable-data-bridge---dynamodb-to-bigtable-migration-utility)** - The DynamoDB to Bigtable Migration tool is a powerful solution designed to streamline data transfer from DynamoDB to Bigtable. This tool automates schema translation, ensuring your data structure is mapped to Bigtable. It also provides options to accelerate and scale data transfer efficiently using Dataflow, minimizing downtime and maximizing performance.

*  **[Bigtable HBase Replication Library](https://github.com/googleapis/java-bigtable-hbase/tree/main/hbase-migration-tools/bigtable-hbase-replication)** - Facilitate near-zero downtime migrations from HBase to Bigtable by enabling to keep your Bigtable instance in sync with your production HBase cluster. Adding Bigtable as an HBase replica guarantees that mutations are applied to Bigtable in the same order as on HBase.
  
## Other Utilities

* **[Kafka Connect Bigtable Sink](https://github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/tree/main/kafka-connect-bigtable-sink)** - This repository contains the source code a Kafka Connect sink connector for Bigtable. This tool enables the streaming of data records from Apache Kafka topics directly into Bigtable tables.

* **[Cassandra to Bigtable Dataflow Template](https://github.com/GoogleCloudPlatform/DataflowTemplates/blob/main/v1/README_Cassandra_To_Cloud_Bigtable.md)** - The Cassandra to Bigtable Dataflow template copies a table from Cassandra to Bigtable. This template requires minimal configuration and replicates the table structure in Cassandra as closely as possible in Bigtable.
  
*  **[HBase Sequence Files to Bigtable using Dataflow](https://github.com/googleapis/java-bigtable-hbase/blob/v2.15.0/bigtable-dataflow-parent/bigtable-beam-import/README.md)** - This folder contains tools to support importing and exporting HBase data to Bigtable using Dataflow and Apache beam.
