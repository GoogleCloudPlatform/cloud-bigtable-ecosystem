# Cassandra to Bigtable Proxy - Limitations

## Overview

The Cassandra to Bigtable Proxy is intended to help you in migrating and
integrating your Cassandra applications with Google Cloud Bigtable, ensuring
that this complicated move runs as smoothly as possible. However, it is
important to understand that this process is not without its challenges. Some of
the limitations you might encounter start from fundamental incompatibilities
between the Cassandra and Bigtable database architectures—imagine them as
mismatched puzzle pieces that don't quite fit together. Other limitations exist
simply because certain features haven't been fully implemented yet in the proxy.

## 1. Supported data types

| CQL Type              | Supported | Primary Key | Cloud Bigtable Mapping                                                             |
|-----------------------|-----------|-------------|------------------------------------------------------------------------------------|
| text                  | yes       | yes         | RAW BYTES                                                                          |
| varchar               | yes       | yes         | RAW BYTES                                                                          |
| timestamp             | yes       | yes         | RAW BYTES                                                                          |
| int                   | yes       | yes         | RAW BYTES                                                                          |
| bigint                | yes       | yes         | RAW BYTES                                                                          |
| blob                  | yes       | yes         | RAW BYTES                                                                          |
| ascii                 | yes       | yes         | RAW BYTES                                                                          |
| float                 | yes       | no          | RAW BYTES                                                                          |
| double                | yes       | no          | RAW BYTES                                                                          |
| boolean               | yes       | no          | RAW BYTES                                                                          |
| counter               | yes       | no          | Col name as col family, values stored in a 'v' column.                             |
| map&lt;key, value&gt; | yes       | n/a         | Col name as col family, MAP key as column qualifier, value as value                |
| set&lt;item&gt;       | yes       | n/a         | Col name as col family, SET item as column qualifier, value remain empty           |
| list&lt;item&gt;      | yes       | n/a         | Col name as col family, current timestamp as column qualifier, list items as value |

All list types follow the same storage pattern:  
**Col name as col family, current timestamp (with nanosecond precision) as
column qualifier, list items as column value.**

### Non-supported data types

The proxy currently doesn't support the following data types: 
date, decimal, duration, inet, smallint, time, timeuuid, tinyint, uuid,
varint, frozen and user-defined types (UDT).

## 2. Supported Functions

We are only supporting these functions as of now.

- **count** - `"select count(colx) from keyspacex.tablex.keyspaceX.tablex`
- **sum** - `"select sum(colx) from keyspacex.tablex.keyspaceX.tablex`
- **avg** - `"select avg(colx) from keyspacex.tablex.keyspaceX.tablex`
- **min** - `"select min(colx) from keyspacex.tablex.keyspaceX.tablex`
- **max** - `"select max(colx) from keyspacex.tablex.keyspaceX.tablex`
- **writetime** - `select writetime(colx)  from keyspacex.tablex`
- **toTimestamp(now())** - `insert into keyspacex.tablex (event_time) values (toTimestamp(now()))`

## 3. Queries with Literals

Due to limitations in the CQL grammar, you might encounter issues with certain
column names that are treated as literals, such as `time`, `key`, `type`,
and `json`. While we have added support for these specific keywords, there could
still be cases where other literals cause conflicts. The good news is that the
proxy is flexible, and we can easily extend support for additional keywords if
needed. If you encounter any unexpected behavior with specific column names,
reach out, and we'll help you resolve it promptly.

## 4. Group By Queries

Currently, the proxy supports `GROUP BY` queries with the following limitations:

1. The column must be a non-collection data type

For example:

```sql
-- Supported:
SELECT column1, COUNT(*)
FROM keyspace.table
GROUP BY column1;

-- Not Supported:
SELECT column1, COUNT(*)
FROM keyspace.table
GROUP BY collection_column; -- Collection columns not supported
```

## 5. Partial Prepared Queries

Currently, the proxy does not support prepared queries where only some values
are parameterized, while others are hardcoded. This means that queries where a
mix of actual values and placeholders (`?`) are used in the same statement are
not supported, except in the case of `LIMIT` clauses. Below are some examples to
clarify:

- **Supported**:
  ```sql
  INSERT INTO keyspace1.table1 (col1, col2, col3) VALUES (?, ?, ?);
  ```
- **Not Supported**:
  ```sql
  INSERT INTO keyspace1.table1 (col1, col2, col3) VALUES ('value', ?, ?);
  SELECT * FROM tableX WHERE col1='valueX' and col2 =?;
  ```

We aim to enhance support for partial prepared queries in future updates. For
now, it's recommended to fully parameterize your queries or use literals
consistently within the same statement.

## 6. Raw Queries in a batch is not supported

We do not support raw queries in batch

**Not Supported**

```python
# Define the raw CQL queries
query1 = "INSERT INTO table1 (col1, col2, col3) VALUES ('value1', 'value2', 'value3');"
query2 = "INSERT INTO table1 (col1, col2, col3) VALUES ('value4', 'value5', 'value6');"
query3 = "UPDATE table1 SET col2 = 'updated_value' WHERE col1 = 'value1';"

# Create a BatchStatement
batch = BatchStatement()

# Add the raw queries to the batch
batch.add(SimpleStatement(query1))
batch.add(SimpleStatement(query2))
batch.add(SimpleStatement(query3))
```

```python
# Prepare the queries
insert_stmt = session.prepare("INSERT INTO table1 (col1, col2, col3) VALUES (?, ?, ?);")
update_stmt = session.prepare("UPDATE table1 SET col2 = ? WHERE col1 = ?;")

# Create a BatchStatement
batch = BatchStatement()

# Add the prepared statements to the batch with bound values
batch.add(insert_stmt, ('value1', 'value2', 'value3'))
batch.add(insert_stmt, ('value4', 'value5', 'value6'))
batch.add(update_stmt, ('updated_value', 'value1'))
```

## 7. CQlSH support

We have had limited support for cqlsh - [cqlsh support](./cqlsh.md)

## 8. Mandatory single quote surrounding values

- To run the Raw DML queries, it is mandatory for all values except numerics to
  have single quotes added to it. For eg.
    ```sh
    SELECT * FROM table WHERE name='john doe';
    INSERT INTO table (id, name) VALUES (1, 'john doe');
    ```

## 9. WHERE Clause Operator Limitations

Currently, the proxy supports only a limited set of operators in WHERE clauses:

1. Equals to operator (`=`)
2. IN operator (`IN`)
3. Greater than (`>`)
4. Greater than or equal to (`>=`)
5. Less than (`<`)
6. Less than or equal to (`<=`)
7. Between (`BETWEEN ... AND ...`)
8. Like (`LIKE`)
9. Contains (`CONTAINS`)
10. Contains Key (`CONTAINS KEY`)

For example:

```sql
-- Supported:
SELECT *
FROM table
WHERE id = 123;
SELECT *
FROM table
WHERE name IN ('John', 'Jane', 'Bob');
SELECT *
FROM table
WHERE col1 > 18;
SELECT *
FROM table
WHERE col1 BETWEEN 10 AND 20;
SELECT *
FROM table
WHERE name LIKE 'John%';
SELECT *
FROM table
WHERE col1 >= 18;
SELECT *
FROM table
WHERE col1 <= 18;
SELECT *
FROM table
WHERE col1 < 18;
SELECT col1
FROM table
WHERE col1 CONTAINS 'name'
SELECT col1
FROM table
WHERE col1 CONTAINS KEY 'name'
SELECT col1
FROM table
WHERE col1 CONTAINS VALUE 'name'
```

If your queries use unsupported operators, you'll need to modify them to use
only the supported operators or handle the filtering in your application logic.

## 10. Timestamp format

We do support below TIMESTAMP for INSERT and UPDATE operation.

- 1299038700000
- '2011-02-03 04:05+0000'
- '2011-02-03 04:05:00+0000'
- '2011-02-03 04:05:00.000+0000'
- '2011-02-03T04:05+0000'
- '2011-02-03T04:05:00+0000'
- '2011-02-03T04:05:00.000+0000'

## 11. Using timestamp not supported with DELETE operation

As we have identified that USING TIMESTAMP could lead to data inconsistency due
to the limitation of applying USING TIMESTAMP on scalar columns, we will not add
the support of UT for delete operations.

As we have identified that USING TIMESTAMP could lead to data inconsistency due
the limitation of applying USING TIMESTAMP on scalar columns, we will not add
the support of UT for delete operations.

This decision might come up with some limitations in application heavily relying
on this feature of cassandra

## 12. Using TTL

We do not support TTL (Time-To-Live) in the proxy because Bigtable manages TTL
at the column family level, whereas Cassandra applies TTL at the cell level. Due
to this fundamental difference in TTL handling, it is currently not possible to
implement this feature in the proxy. As a result, this remains a limitation of
bigtable proxy.

## 13. Limited support for system Queries

We only support limited ***system Queries***

- `SELECT * FROM system.local WHERE key='local'`
- `SELECT * FROM system.peers`
- `SELECT * FROM system_schema.keyspaces`
- `SELECT * FROM system_schema.tables`
- `SELECT * FROM system_schema.columns`
- `SELECT * FROM system_virtual_schema.keyspaces`
- `SELECT * FROM system_virtual_schema.tables`
- `SELECT * FROM system_virtual_schema.columns`
- `USE keyspace_name`
- `DESCRIBE KEYSPACES`
- `DESCRIBE KEYSPACE keyspace_name`

## 14. Reconnect to proxy after DDL

It is necessary to reconnect or restart the session after performing DDL (
Create, Alter, Drop) in order to refresh the schema at the client (i.e., the
schema metadata information at CQLSH); otherwise, there is a possibility that
some queries might fail.

## 15. Inserting "empty rows"

In Cassandra, it is possible to insert "empty rows" (rows without any columns)
using a command
like `INSERT INTO table_name (primary_key_column) VALUES (value);`. However,
this behavior is not supported in Bigtable, which requires at least one column
to be present in a row. As a result, attempting to insert an empty row through
the proxy will lead to an error. To work around this limitation, you can insert
a placeholder column with a default value when creating a new row.