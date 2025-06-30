# Integrating CQLSH with Cassandra-to-Bigtable Proxy

This guide provides detailed instructions on setting up CQLSH to connect to the Cassandra-to-Bigtable proxy, along with an overview of supported and unsupported queries, and known limitations.

## Prerequisites

- **CQLSH Version:** Ensure you have CQLSH versions from the branches `cassandra-4.0.13` or `cassandra-4.1.5`.
- **Proxy Setup:** Ensure the Cassandra-to-Bigtable proxy is set up and running.

## Setup Instructions

>> NOTE: You may skip these instructions, if you already have CQLSH interface installed.

### Option 1: Install CQLSH Directly

#### Step 1: Download CQLSH

Download the appropriate version of CQLSH by cloning the repository and following the instructions:

- **CQLSH 4.0.13:**
  ```sh
  git clone https://github.com/apache/cassandra.git -b cassandra-4.0.13
  cd cassandra/bin
  ```

- **CQLSH 4.1.5:**
  ```sh
  git clone https://github.com/apache/cassandra.git -b cassandra-4.1.5
  cd cassandra/bin
  ```

#### Step 2: Configure CQLSH to Connect to the Proxy (Optional)

Edit the CQLSH configuration to point to the Cassandra-to-Bigtable proxy:

1. Open the `cqlshrc` configuration file. If it does not exist, create one in your home directory:
   ```sh
   nano ~/.cassandra/cqlshrc
   ```

2. Add the following configuration:
   ```ini
   [connection]
   hostname = <proxy_hostname>
   port = <proxy_port>
   ```

   Replace `<proxy_hostname>` and `<proxy_port>` with the appropriate values for your proxy setup.

#### Step 3: Launch CQLSH

Launch CQLSH with the configured/default settings:
```sh
./cqlsh
```

Launch CQLSH with the custom hostname and port:
```sh
./cqlsh <proxy_hostname> <proxy_port>
```

Replace `<proxy_hostname>` and `<proxy_port>` with the appropriate values.

### Option 2: Use Dockerized CQLSH

#### Step 1: Install Docker

Ensure Docker is installed on your machine. Follow the instructions on the [Docker website](https://docs.docker.com/get-docker/) to install Docker for your operating system.

#### Step 2: Download and Run the Dockerized CQLSH

Download the relevant Docker image and open a bash shell:
```sh
docker run -it nuvo/docker-cqlsh bash
```

#### Step 3: Find Your Machine's IP Address

Find the local IP address of the machine if the proxy is running locally, or use the proxy server IP address. For macOS, you can get the local machine IP address using:
```sh
ifconfig | grep "inet " | grep -v 127.0.0.1
```

#### Step 4: Connect to the Proxy

Open a bash shell in the Docker image:
```sh
docker run -it nuvo/docker-cqlsh bash
```

Connect to the proxy using your IP address and port:
```sh
cqlsh --cqlversion='3.4.5' '<your_ip_address>'
```

Replace `<your_ip_address>` with the local IP address obtained in Step 3.

## Supported Operations

### DDL Operations

The proxy supports Data Definition Language (DDL) operations, making it easier to create and manage tables directly through CQL commands.

#### CREATE TABLE
```sql
CREATE TABLE keyspace.table (
    id bigint,
    name text,
    age int,
    PRIMARY KEY ((id), name)
);
```

#### ALTER TABLE
```sql
-- Add a new column
ALTER TABLE keyspace.table ADD email text;

-- Drop a column
ALTER TABLE keyspace.table DROP email;
```

#### DROP TABLE
```sql
DROP TABLE keyspace.table;
```

#### Batch Table Creation
You can create multiple tables at once using a SQL file:
```bash
cqlsh localhost 9042 -f schema.sql
```

Example schema.sql content:
```sql
CREATE TABLE keyspace.table1 (
    id bigint,
    name text,
    age int,
    PRIMARY KEY ((id), name)
);

CREATE TABLE keyspace.table2 (
    user_id text,
    email text,
    created_at timestamp,
    PRIMARY KEY (user_id)
);
```

### DML Operations

#### INSERT
```sql
INSERT INTO keyspace_name.table_name (col1, col2, time, count)
  VALUES ('1234', 'check', '2011-02-13T05:19:16.882Z', 10);

// Inserting a row only if it does not already exist
INSERT INTO keyspace_name.table_name (id, lastname, firstname)
   VALUES (c4b65263-fe58-4846-83e8-f0e1c13d518f, 'John', 'Rissella')
IF NOT EXISTS;
```

#### SELECT
```sql
SELECT * FROM keyspace_name.table_name WHERE col1 = '1234';
SELECT * FROM keyspace_name.table_name LIMIT 2;
SELECT * FROM keyspace_name.table_name ORDER BY col2 ASC|DESC LIMIT 2;
```

#### UPDATE
```sql
UPDATE keyspace_name.table_name SET count = 15 WHERE col1 = '1234';
UPDATE keyspace_name.table_name SET count = 15 WHERE col1 = '1234' IF EXISTS;
```

#### DELETE
```sql
DELETE FROM keyspace_name.table_name WHERE col1 = '1234';
DELETE FROM keyspace_name.table_name WHERE col1 = '1234' IF EXISTS;
```

#### Using Timestamp
```sql
INSERT INTO keyspace_name.table_name (col1, col2, col3) 
VALUES ('Value1', '3', test_blob_data) USING TIMESTAMP 1686638356882;

UPDATE keyspace_name.table_name USING TIMESTAMP 1686638356882 
SET col2 = 'value2', col3 = 'value3' WHERE col1 = 'Value1';

DELETE FROM keyspace_name.table_name USING TIMESTAMP 1686638356882 
WHERE col1='Value1';
```

## Unsupported Operations

### System Queries
System queries using cqlsh are not supported:
```sql
SELECT * FROM system_schema.keyspaces;
SELECT * FROM system_schema.tables;
SELECT * FROM system_schema.columns;
```

### Batch Queries
Raw batch queries are not yet supported (support coming soon):
```sql
BEGIN BATCH
INSERT INTO keyspace_name.table_name (col1, col2, time, count) 
VALUES ('1234', 'check', '2024-06-13T05:19:16.882Z', 10);
INSERT INTO keyspace_name.table_name (col1, col2, time, count) 
VALUES ('1234', 'check', '2024-06-13T05:19:16.882Z', 20);
APPLY BATCH;
```

## Best Practices

1. Always use single quotes for string values in DML operations
2. Use prepared statements for better performance
3. Keep table names and column names consistent with Bigtable naming conventions
4. Use appropriate data types that are supported by both Cassandra and Bigtable
5. When creating tables, ensure primary keys are properly defined
6. Use batch operations for multiple related changes
7. Use the correct timestamp format: `2024-06-13T05:19:16.882Z`

## Limitations

1. Some advanced CQL features may not be supported
2. Complex queries with multiple conditions may have performance implications
3. Certain data type conversions may have limitations
4. Index operations are not supported
5. Materialized views are not supported
6. System queries are not supported
7. Raw batch queries are not yet supported

For more detailed information about limitations, please refer to the [Limitations](./limitations.md) document.

