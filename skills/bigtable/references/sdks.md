# Google Cloud Bigtable Client SDKs & Connectors Reference

This document provides the canonical GitHub repository links, monorepo subdirectories, and lookup instructions for all Google Cloud Bigtable client libraries, connectors, and developer tools.

## 1. Idiomatic Client Libraries (SDKs)

| SDK / Language | Primary GitHub Repository / Monorepo Path | Notes |
| :--- | :--- | :--- |
| **Java (Veneer/gRPC)** | https://github.com/googleapis/google-cloud-java/tree/main/java-bigtable | Official Java client |
| **Java (HBase Client)** | https://github.com/googleapis/java-bigtable-hbase | HBase 1.x and 2.x compatible client for Bigtable |
| **Go** | https://github.com/googleapis/google-cloud-go/tree/main/bigtable | Official Go client (`cloud.google.com/go/bigtable`) |
| **Python** | https://github.com/googleapis/google-cloud-python/tree/main/packages/google-cloud-bigtable | Official Python client (`google-cloud-bigtable`) |
| **Node.js (TS/JS)** | https://github.com/googleapis/nodejs-bigtable | Official Node.js client (`@google-cloud/bigtable`) |
| **C++** | https://github.com/googleapis/google-cloud-cpp/tree/main/google/cloud/bigtable | Official C++ client |
| **.NET (C#)** | https://github.com/googleapis/google-cloud-dotnet/tree/main/apis/Google.Cloud.Bigtable.V2 | Official .NET client (`Google.Cloud.Bigtable.V2`) |
| **Ruby** | https://github.com/googleapis/google-cloud-ruby/tree/main/google-cloud-bigtable | Official Ruby gem (`google-cloud-bigtable`) |
| **PHP** | https://github.com/googleapis/google-cloud-php/tree/main/Bigtable | Official PHP client (read-only subtree: https://github.com/googleapis/google-cloud-php-bigtable) |
| **Rust** | https://github.com/googleapis/google-cloud-rust | Official Google Cloud Rust SDK (WIP) |

## 2. Connectors & Ecosystem Adapters

| Connector / Tool | GitHub Repository URL | Notes |
| :--- | :--- | :--- |
| **Apache Spark Connector** | https://github.com/GoogleCloudDataproc/spark-bigtable-connector | Official Spark-Bigtable connector (Scala/Java/PySpark DataFrames & SQL) |
| **Cassandra Adapter & Ecosystem** | https://github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem | Cassandra-Bigtable proxy adapter and ecosystem tools |
| **Apache Beam / Dataflow (Java)** | https://github.com/apache/beam/tree/master/sdks/java/io/google-cloud-platform/src/main/java/org/apache/beam/sdk/io/gcp/bigtable | Beam Java SDK Bigtable IO connector |
| **Apache Beam / Dataflow (Python)** | https://github.com/apache/beam/blob/master/sdks/python/apache_beam/io/gcp/bigtableio.py | Beam Python SDK Bigtable IO connector |
| **Kafka Connect Sink** | https://github.com/GoogleCloudPlatform/kafka-connect-bigtable-sink | Official Kafka Connect sink connector for Bigtable |

## 3. CLI & Developer Tools

| Tool | GitHub Repository URL | Notes |
| :--- | :--- | :--- |
| **`cbt` CLI** | https://github.com/googleapis/google-cloud-go/tree/main/bigtable/cmd/cbt | Official command-line tool for Cloud Bigtable |
| **Bigtable Emulator (`bttest`)** | https://github.com/googleapis/google-cloud-go/tree/main/bigtable/bttest | In-memory Bigtable server used by `gcloud beta emulators bigtable` |

## 4. Code Samples & Reference Architectures

| Language / Category | GitHub Repository URL |
| :--- | :--- |
| **General Examples & Dataflow Templates** | https://github.com/GoogleCloudPlatform/cloud-bigtable-examples |
| **Java Documentation Samples** | https://github.com/GoogleCloudPlatform/java-docs-samples/tree/main/bigtable |
| **Python Documentation Samples** | https://github.com/GoogleCloudPlatform/python-docs-samples/tree/main/bigtable |
| **Go Documentation Samples** | https://github.com/GoogleCloudPlatform/golang-samples/tree/main/bigtable |

---

## Instructions for Fetching the Latest Information

When the user asks a question about a specific Bigtable SDK or connector (e.g., latest release, recent commits, specific code implementation, or documentation):

1. **Check Releases & Tags:**
   - Use `search_web` with `site:<repo_url>/releases` or use `read_url_content` on `https://github.com/<org>/<repo>/releases` to inspect the latest published versions and changelogs.
2. **Read Source Code at HEAD:**
   - Use `read_url_content` on raw GitHub URLs (`https://raw.githubusercontent.com/<org>/<repo>/main/<path>`) or use `search_web` scoped to the repository path.
