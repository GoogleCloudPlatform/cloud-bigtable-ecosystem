# Bigtable SQL Guide for Agents

This document outlines key aspects of Google Bigtable's SQL dialect which extends GoogleSQL to support a multi-version wide-column data model. Bigtable currently only supports SELECT statements over single tables i.e. JOIN and UNION operations and nested queries are not supported.

## Bigtable SQL Data Structures

### Columns  

Unless table metadata indicates otherwise, columns are of **Map type** which hold versioned key-value pairs. In legacy Bigtable APIs, SQL maps correspond to **column families**.

* **Access Pattern:** You cannot select a column directly as a scalar value. You must access the specific key within the column family.
* **Correct Syntax Example:**

    ```sql
    -- CORRECT: Use map-like bracket notation for the column qualifier.
    SELECT cf1['text'] FROM messages;
    ```

* **Incorrect Syntax Example:**

    ```sql
    -- INCORRECT: Dot notation is not supported and will fail.
    SELECT cf1.text FROM messages;
    ```

### Primitive types

There is often no type information associated with column values. So generated SQL queries should try to infer the type from the name of the column and include an explicit cast.

* *Example:* `SELECT CAST(info['age'] AS INT64) AS age FROM table_name`
* *Example:* `SELECT CAST(info['address'] AS STRING) AS address FROM table_name`
* *Example:* `SELECT CAST(CAST(info['age'] AS STRING) AS INT64) AS age FROM table_name`
* *Example:* `SELECT TO_INT64(cf['age']) as age FROM table_name`
* *Example:* `SELECT CAST(CAST(cf['checkin_date'] AS STRING) AS DATE) AS checkin_date FROM table_name`
* *Example:* `SELECT TIMESTAMP(CAST(CAST(cf['checkin_date'] AS STRING) AS DATE)) AS checkin_date_time FROM table_name`
* *Example:* `SELECT CAST(CAST(cf['is_booked'] AS STRING) AS BOOL) AS is_booked FROM table_name` - if "true" or "false" was stored
* *Example:* `SELECT CAST(TO_INT64(cf['is_booked']) AS BOOL) AS is_booked FROM table_name` - if 1 or 0 was stored`
* *Example:* `SELECT CAST(SAFE_CONVERT_BYTES_TO_STRING(cf['is_booked']) AS BOOL) AS is_booked from table_name`, if "true" or "false" was stored

### Timestamps

 By default Bigtable returns the  **latest value** of each column when use with a standard SELECT statement. You can use different flags explained below to get prior versions. Using any flag other than "as_of" and "with_history => FALSE" will return timestamp-value pairs. SQL interface exposes Bigtable timestamps as SQL TIMESTAMP type.

* **Access Pattern:** To retrieve values as of a certain point in time (on or immediately prior to the provided timestamp), use the as_of flag.
* **Syntax:** `SELECT * FROM table_name(as_of => TIMESTAMP("2025-03-28 14:13:40-0400"))`

---

* **Access Pattern:** To retrieve all versions treat table name as a table-valued function and set the with_history flag to TRUE.  
* **Syntax:** `SELECT * FROM table_name(with_history => TRUE)`

---

* **Access Pattern:** To retrieve last 5 versions treat table name as a table-valued function and use the with_history flag.
* **Syntax:** `SELECT * FROM table_name(with_history => TRUE, latest_n => 5)`

---

* **Access Pattern:** To retrieve a range of timestamps treat table name as a table-valued function and use the before, after, after_or_equal, and before_or_equal flags.
* **Syntax:** `SELECT * FROM table_name(with_history => true, after => TIMESTAMP("2025-03-28 14:13:40-0400"), before_or_equal => TIMESTAMP("2025-03-28 14:15:10-04:00")`
* **Agent Action:** For details on how to utilize other version management flags refer to the Google Cloud documentation.

### Row keys

Each Bigtable row is identified with a unique key. Bigtable SQL interface has a pseudo-column named  **_key** that should to query by row key.

* **Syntax:** `SELECT * FROM table_name WHERE _key = row_key`
* **Agent Action:** When generating queries that are not looking for exact matches i.e. **WHERE _key=** or key ranges i.e. **_key > AND _key<=** or prefixes **STARTS_WITH(_key, prefix)** warn the user that the query will result in a full table scan and won't be performant.

### Functions and operators

Bigtable offers a wide range of [SQL functions](https://docs.cloud.google.com/bigtable/docs/reference/sql/functions-all) and [operators](https://docs.cloud.google.com/bigtable/docs/reference/sql/operators) and [conditional expressions](https://docs.cloud.google.com/bigtable/docs/reference/sql/conditional_expressions).
