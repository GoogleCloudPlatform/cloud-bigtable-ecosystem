# Overview

This is an Apache Cassandra proxy that translates CQL requests into Bigtable operations, allowing the user to use
existing CQL DDL and DML commands to read and write from Bigtable. Bigtable currently only supports SELECT SQL queries,
so all query types are translated to normal Bigtable API calls. CQL queries are translated by translators in the
translator/ module, into intermediate golang types that implement the types.IPreparedQuery interface. The translator
also handles binding a prepared query with parameters, when the query is executed, creating a types.IExecutableQuery
struct. Executable queries are run by various executors in the executors/ module. Most queries are handled by
bigtable_executor.go, but system queries are handled by other executors because they don't interact with Bigtable. All
direct Bigtable interactions are handled in the bigtable/ module. Both prepared and adhoc queries are supported.

All schema information is stored in a dedicated schema_mapping Bigtable table, which contains one row per column in a
table. Each Cassandra keyspace is mapped to a Bigtable Instance and each table gets its own Bigtable table. All schema
information for a table is stored in a schemaMapping.TableConfig object. The table schema information is cached in a
global schema-mapping/SchemaMappingConfig object. The cache is refreshed whenever a DDL operation is run.

See [architecture.md](./architecture.md) for more details.

## Supported CQL Queries

* SELECT - uses the Bigtable SQL API to query data in a table
* DELETE - uses the Bigtable mutation API to delete data
* UPDATE - uses the Bigtable mutation API to write data
* INSERT - uses the Bigtable mutation API to write data
* TRUNCATE - uses the Bigtable Admin API to delete all rows
* CREATE - uses the Bigtable Admin API to create a new Bigtable table and then updates the schema_mapping table
* ALTER - uses the Bigtable mutation API to update the schema_mapping table
* DROP - uses the Bigtable Admin API to delete the Bigtable table and then updates the schema_mapping table
* DESCRIBE - uses the cached schema mappings to respond to the client directly
* USE - sets the proxy session keyspace in memory

# Life of a prepared CQL SELECT Query

1. The CQL query is received by the server code in third_party/proxy/proxy.go
2. The query type is determined, and translators/TranslatorManager routes the raw query string to the appropriate
   translator
3. The translators/select_translator/SelectTranslator translates the query into an intermediate
   types.PreparedSelectQuery struct which contains various structs describing the query clauses and a TranslatedQuery
   string which is the Bigtable SQL string.
4. The third_party/proxy/proxy.go then caches the prepared query and returns the query id and required query parameters
   to the client.
5. When the client executes the prepared query, the PreparedSelectQuery is retrieved from the cache, and the
   translators/select_translator/SelectTranslator binds any query parameters to the prepared query, turning it into a
   types.ExecutableSelectQuery, which implements the types.IExecutableQuery interface.
6. The executors/QueryExecutorManager routes the executable query to the executors/bigtable_executor.go executor, which
   invokes the bigtable/ module.
7. The bigtable/ module handles executing the select query against the Bigtable SQL API.
8. The read response is translated into plain go types.
9. The plain go types are encoded into a CQL response message.

# System Queries

System queries do not interact with Bigtable at all - they are entirely driven by in memory data stored in the
SchemaMappingConfig struct. This means all select queries on system tables are implemented with a basic in memory query
handler located in the mem_table/ module.

# Source Code locations

- All common types and intermediate query types are located in global/types/
- The translators/ module handles converting CQL into intermediate structs. Translators prepare queries and bind
  parameters into executable queries.
- The executors/ module handles running executable queries.
