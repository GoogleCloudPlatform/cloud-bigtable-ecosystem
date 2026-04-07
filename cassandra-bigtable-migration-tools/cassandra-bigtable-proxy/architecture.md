# Cassandra-to-Bigtable Proxy Architecture

This document describes the high-level architecture of the Cassandra-to-Bigtable proxy, which translates Cassandra Query Language (CQL) requests into Google Cloud Bigtable operations.

## Table of Contents
- [Overview](#overview)
- [System Components](#system-components)
- [Data Flow](#data-flow)
- [Life of a Prepared SELECT Query](#life-of-a-prepared-select-query)
- [Life of an INSERT Query](#life-of-a-prepared-insert-query)
- [Handling DDL, USE, and DESCRIBE Statements](#handling-ddl-use-and-describe-statements)
- [The Schema Mapping Table](#the-schema-mapping-table)
- [Response Handling](#response-handling)
- [Configuration](#configuration)
- [CQL Data Type Storage in Bigtable](#cql-data-type-storage-in-bigtable)
- [Schema Mapping Strategy](#schema-mapping-strategy)

## Overview

The proxy acts as an intermediary between Cassandra clients and Bigtable. It implements the Cassandra Native Protocol, allowing existing applications to interact with Bigtable using CQL drivers. The proxy handles the translation of CQL queries into Bigtable SQL or Bigtable Mutation API calls.

## System Components

The project is structured into several key layers:

### 1. Server & Protocol Layer
- **`proxy.go` (root)**: The entry point that initializes the proxy.
- **`third_party/datastax/proxy`**: Contains the core networking logic, session management, and request handling. It uses the `proxycore` library for Cassandra protocol frame handling.
- **`third_party/datastax/proxy/proxy.go`**: The `Proxy` and `client` structs manage the lifecycle of client connections and route incoming CQL frames (e.g., `QUERY`, `PREPARE`, `EXECUTE`, `BATCH`) to the translation and execution layers.

### 2. Parsing Layer
- **`third_party/cqlparser`**: An ANTLR-based CQL parser that converts raw CQL strings into a parse tree.
- **`parser/parser.go`**: Provides a `ProxyCqlParser` wrapper to facilitate navigation and extraction of information from the parse tree.

### 3. Translation Layer (`translators/`)
This layer is responsible for converting CQL into intermediate, structured query types.
- **`TranslatorManager`**: Orchestrates the translation process by routing queries to specific translators based on the `QueryType` (SELECT, INSERT, UPDATE, etc.).
- **Specific Translators**: (e.g., `select_translator`, `insert_translator`) handle the logic of mapping CQL clauses to Bigtable equivalents. For `SELECT` queries, this involves generating Bigtable SQL. For mutations, it involves preparing mutation data.
- **`IPreparedQuery`**: The output of the translation phase, representing a query that has been parsed and mapped but might still have unbound parameters.

### 4. Binding Phase
- **`BindQuery`**: Before execution, `TranslatorManager.BindQuery` takes a `IPreparedQuery` and client-provided values to produce an `IExecutableQuery`. This step handles parameter substitution and type conversion.

### 5. Execution Layer (`executors/`)
- **`QueryExecutorManager`**: Routes an `IExecutableQuery` to the appropriate executor.
- **`BigtableExecutor`**: The primary executor for most DML and DDL queries. it uses the `BigtableAdapter` to interact with the Bigtable API.
- **`DescribeExecutor`**: Handles `DESCRIBE` queries by fetching schema information from the metadata cache.
- **`UseExecutor`**: Handles `USE <keyspace>` queries by updating the session state.
- **`SelectSystemTableExecutor`**: Routes queries for Cassandra system tables to an in-memory engine.

### 6. Bigtable Adapter (`bigtable/`)
- **`BigtableAdapter`**: A high-level wrapper around the official Google Cloud Bigtable Go client. It handles:
    - Executing Bigtable SQL queries.
    - Applying single or bulk mutations.
    - Table administration (Create, Drop, Truncate).

### 7. Metadata & Schema Mapping (`metadata/`)
- **`MetadataStore`**: Manages the mapping between Cassandra tables/columns and Bigtable tables/families/qualifiers.
- **Schema Mapping Table**: The proxy stores its schema definitions in a dedicated Bigtable table (often named `schema_mapping`). This table records column types, primary key roles, and ordering.
- **Caching**: Schema information is cached in memory for performance and refreshed during DDL operations.

### 8. In-Memory Engine (`mem_table/`)
- **`InMemEngine`**: A basic in-memory query engine used to respond to queries against Cassandra system tables (e.g., `system.local`, `system.peers`), which are required by many Cassandra drivers to discover cluster topology.

## Data Flow

### Ad-hoc Query (QUERY)
1.  Client sends a `QUERY` frame.
2.  `Proxy` receives the frame and parses the query type.
3.  `TranslatorManager` translates the CQL into an `IPreparedQuery`.
4.  `TranslatorManager` binds any provided parameters to create an `IExecutableQuery`.
5.  `QueryExecutorManager` selects an executor (e.g., `BigtableExecutor`).
6.  Executor runs the query against Bigtable and receives results.
7.  Results are formatted into a Cassandra `RESULT` frame and sent back to the client.

### Prepared Query (PREPARE & EXECUTE)
1.  **PREPARE**:
    - Client sends a `PREPARE` frame.
    - `Proxy` translates the query and caches the `IPreparedQuery` with a unique ID.
    - Proxy returns a `PREPARED` result frame containing the ID and metadata about parameters.
2.  **EXECUTE**:
    - Client sends an `EXECUTE` frame with the query ID and parameter values.
    - `Proxy` retrieves the `IPreparedQuery` from the cache.
    - `TranslatorManager` binds the values to create an `IExecutableQuery`.
    - `QueryExecutorManager` executes the query and returns the results.

### Batch Query (BATCH)
1.  Client sends a `BATCH` frame containing multiple mutations.
2.  `Proxy` iterates through the mutations, preparing and binding each one.
3.  Mutations for the same table are grouped.
4.  `BigtableAdapter` uses `ApplyBulkMutation` to execute them efficiently.

## Life of a Prepared SELECT Query

1.  **Preparation Phase**:
    - The client sends a `PREPARE` frame containing a CQL `SELECT` string.
    - `Proxy.handlePrepare` (in `proxy.go`) calls `TranslatorManager.TranslateQuery`.
    - `SelectTranslator` parses the CQL, identifies the table/columns, and generates a corresponding Bigtable SQL string.
    - It returns a `PreparedSelectQuery` which is cached by the `Proxy` and assigned a 16-byte ID.
    - The proxy sends a `PREPARED` result frame back to the client.

2.  **Execution Phase**:
    - The client sends an `EXECUTE` frame with the query ID and parameter values.
    - `Proxy.handleExecute` retrieves the `PreparedSelectQuery` from the cache.
    - It calls `TranslatorManager.BindQuery`, which converts the CQL parameter values into Go types and produces an `ExecutableSelectQuery`.
    - `QueryExecutorManager` routes the query to the `BigtableExecutor`.

3.  **Bigtable Interaction**:
    - `BigtableExecutor` calls `BigtableAdapter.ExecutePreparedStatement`.
    - The adapter converts the bound parameters into Bigtable SQL parameters via `BuildBigtableParams`.
    - It uses the Bigtable Go client to execute the SQL statement against the Bigtable SQL API.

4.  **Result Processing**:
    - As rows are returned from Bigtable, `BigtableAdapter.convertResultRow` maps each Bigtable column back to its CQL equivalent.
    - This involves decoding Big-Endian bytes for scalars and reconstructing collection types (lists, sets, maps) from multiple cells/qualifiers if necessary.
    - `responsehandler.BuildRowsResultResponse` packages the resulting Go rows into a Cassandra protocol `RESULT` frame (specifically a `RowsResult`).
    - The `Proxy` sends this frame back to the client.

## Life of a Prepared INSERT Query

1.  **Preparation Phase**:
    - The client sends a `PREPARE` frame containing a CQL `INSERT` string.
    - `InsertTranslator` (via `TranslatorManager`) parses the query to identify the table, columns, and provided values.
    - It determines which columns are part of the primary key to be used for row key generation later.
    - It returns a `PreparedInsertQuery` which is cached by the `Proxy`.

2.  **Execution Phase**:
    - The client sends an `EXECUTE` frame with the query ID and parameter values.
    - `TranslatorManager.BindQuery` is called to create an `IExecutableQuery`.
    - `common.BindRowKey` generates the Bigtable **Row Key** by encoding the primary key values using the "Ordered Code" format.
    - `common.BindMutations` converts the non-primary key column values into `BigtableWriteMutation` operations.
    - `QueryExecutorManager` routes the `BigtableWriteMutation` to the `BigtableExecutor`.

3.  **Bigtable Interaction**:
    - `BigtableExecutor` calls `BigtableAdapter.mutateRow`.
    - `BigtableAdapter.buildMutation` constructs a `bigtable.Mutation` object.
    - Scalar values are encoded into Big-Endian bytes and added to the mutation via `mut.Set`.
    - Collection assignments (sets, maps, lists) are converted into multiple `mut.Set` operations using the appropriate column qualifiers.
    - If `IF NOT EXISTS` was specified, the adapter uses a `CondMutation` (Conditional Mutation) with a `CellsPerRowLimitFilter(1)` as a predicate to check for row existence before applying the mutation.

4.  **Result Processing**:
    - If the mutation is successful, the proxy returns a `VOID` result frame (or a `RowsResult` for conditional inserts to indicate if they were `[applied]`).

## Response Handling

- **`responsehandler/`**: Contains utilities for converting Bigtable results (from the SQL API or Row API) back into Cassandra protocol frames. This includes handling data type conversions and row formatting.

## Configuration

- **`config.yaml`**: The primary configuration file for the proxy. It defines:
    - Listen address and port.
    - Bigtable project, instance, and keyspace mappings.
    - Security settings (TLS, authentication).
    - Logging levels and OpenTelemetry exporters.
    - Schema mapping table name.

## CQL Data Type Storage in Bigtable

The proxy maps Cassandra's data model to Bigtable's cell-based storage using several strategies depending on the CQL type and the column's role (Primary Key vs. Regular Column).

### 1. Primary Keys (Row Key)
Primary keys (partition and clustering keys) are concatenated to form the Bigtable Row Key.
- **Encoding**: Uses an "Ordered Code" encoding (see `translators/common/orderedcode.go`) to ensure that the byte-order of the Bigtable row keys matches the logical CQL ordering (especially important for clustering columns).
- **String/Blob**: Encoded using Ordered Code but with the trailing delimiter removed to allow for prefix scans.
- **Integers**: Encoded to preserve numerical order. Supports both Big-Endian (for non-negative values) and Ordered Code (for full range support).
- **Timestamp**: Converted to Unix microseconds and encoded as an ordered 64-bit integer.
- **Delimiter**: Components of a composite primary key are separated by a specific 2-byte delimiter (`\x00\x01`).

### 2. Regular Columns (Cell Values)
For regular (non-primary key) columns, values are generally stored as the cell's value.
- **Encoding**: Most scalar types are encoded using the Cassandra Native Protocol's Big-Endian format (Protocol Version 4).
- **Scalars**:
    - `text`, `varchar`, `ascii`: Raw UTF-8 string bytes.
    - `int`, `tinyint`, `smallint`: Encoded as 32-bit or 64-bit Big-Endian integers.
    - `bigint`, `counter`: Encoded as 64-bit Big-Endian integers.
    - `boolean`: Stored as a 64-bit Big-Endian integer (1 for true, 0 for false).
    - `float`, `double`: Standard IEEE 754 Big-Endian floating-point encoding.
    - `timestamp`: 64-bit Big-Endian integer representing milliseconds since epoch.
    - `blob`: Raw bytes.
    - `uuid`, `timeuuid`: 16-byte raw representation.

### 3. Collection Types
Collections are mapped to multiple Bigtable cells within the same row and column family, using the **Column Qualifier** to store element information.
- **Sets**: Each element is a separate cell. The **Column Qualifier** is the string representation of the element value (e.g., `"123"` for an int, `"true"` for a bool). The cell value is empty.
- **Maps**: Each key-value pair is a separate cell. The **Column Qualifier** is the string representation of the map key. The cell value is the encoded map value (using the same Big-Endian encoding as regular columns).
- **Lists**: To maintain order while allowing efficient appends/prepends, each element is stored in a cell where the **Column Qualifier** is a 12-byte custom-encoded value. It consists of an 8-byte Big-Endian timestamp (milliseconds) followed by a 4-byte Big-Endian "nanoseconds" offset to ensure uniqueness even for elements added at the same millisecond.

### 4. Special Types
- **Counters**: Use Bigtable's native counter support (64-bit increments).

## Handling DDL, USE, and DESCRIBE Statements

Beyond standard DML queries (SELECT, INSERT, etc.), the proxy handles administrative and session-level CQL commands.

### 1. DDL Statements (CREATE, ALTER, DROP, TRUNCATE)
DDL operations modify both the Bigtable physical schema and the proxy's internal metadata.
- **`BigtableExecutor`**: Routes these requests to the `BigtableAdapter`.
- **`MetadataStore`**: Orchestrates the changes for `CREATE`, `ALTER`, and `DROP`.
    - It uses the Bigtable Admin API to create, modify, or delete the physical Bigtable table.
    - It updates the `schema_mapping` Bigtable table (the proxy's "source of truth" for schema) with details about column types and primary key roles.
    - It refreshes the global in-memory schema cache to ensure subsequent queries use the updated definitions.
- **`TRUNCATE`**: The `BigtableAdapter` directly calls the Bigtable Admin API's `DropAllRows` method for the target table.

### 2. USE Statements
- **`UseExecutor`**: Processes `USE <keyspace>;` commands.
- **Validation**: It verifies that the requested keyspace exists in the configured Bigtable instances.
- **Session State**: Updates the `sessionKeyspace` on the `client` connection object. All subsequent queries on that connection will use this keyspace by default.
- **Result**: Returns a `SetKeyspaceResult` frame to the client.

### 3. DESCRIBE Statements
- **`DescribeExecutor`**: Handles `DESCRIBE` commands (e.g., `DESCRIBE TABLE`, `DESCRIBE KEYSPACES`).
- **Metadata-Driven**: These queries **do not** interact with Bigtable. They are served entirely from the in-memory schema cache.
- **Formatting**: The executor formats the cached metadata into a `RowsResult` that mimics the output format expected by Cassandra tools like `cqlsh`.

## The Schema Mapping Table

The proxy maintains a "source of truth" for the Cassandra-to-Bigtable schema mapping in a dedicated Bigtable table (default: `schema_mapping`). This table allows the proxy to persist and recover the mapping across restarts and among multiple proxy instances.

### Table Structure
The schema mapping table uses a single column family (`cf`) and stores one Bigtable row for each column in every Cassandra table.
- **Row Key**: The row key is formatted as `TableName#ColumnName`.
- **Columns (in `cf`)**:
    - `TableName`: The name of the Cassandra table.
    - `ColumnName`: The name of the Cassandra column.
    - `ColumnType`: The CQL type string (e.g., `varchar`, `int`, `map<text, int>`).
    - `IsPrimaryKey`: A boolean string (`true`/`false`) indicating if the column is part of the primary key.
    - `PK_Precedence`: An integer (1-indexed) indicating the column's position in the primary key.
    - `KeyType`: The role of the column in the primary key (e.g., `partition_key`, `clustering`).

### Usage
1.  **Initialization**: During startup, the `MetadataStore` reads all rows from the schema mapping table for each configured keyspace to build the in-memory schema cache.
2.  **DDL Operations**: When a `CREATE TABLE` or `ALTER TABLE` command is executed, the proxy updates the schema mapping table before refreshing its local cache.
3.  **Discovery**: `DESCRIBE` commands and queries against `system_schema` tables use the metadata stored in this table (via the cache) to respond to clients.
4.  **Validation**: Every DML query is validated against the schema mapping to ensure columns exist and types are compatible.

## Schema Mapping Strategy

- **Keyspace**: Each Cassandra keyspace is mapped to a Bigtable Instance (configured in `config.yaml`).
- **Table**: Each Cassandra table is mapped to a unique Bigtable table.
- **Column Families**: By default, all regular columns are mapped to a single Bigtable column family (e.g., `cf`).
- **Primary Keys**: Cassandra's Partition and Clustering keys are combined into the Bigtable Row Key. The `MetadataStore` tracks the order and types of these components to ensure correct row key generation and query translation.
- **Types**: Cassandra types (e.g., `text`, `int`, `uuid`, `timestamp`) are mapped to their Bigtable SQL equivalents or serialized to bytes for storage in cells.
