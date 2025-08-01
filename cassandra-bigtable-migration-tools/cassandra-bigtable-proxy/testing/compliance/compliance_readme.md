# Compliance Test Framework
## Overview

This compliance test framework provides a comprehensive suite for validating the functionality and consistency of a proxy that translates Cassandra Query Language (CQL) into Bigtable operations. It enables you to define detailed test scenarios using JSON files, which are then executed against both Bigtable (through the proxy) and a native Cassandra instance.

The framework focuses on ensuring:

* **Validate Proxy Functionality (CQL Translation and Execution):** This framework rigorously validates the proxy's ability to accurately translate Cassandra Query Language (CQL) queries into their equivalent Bigtable operations (queries and mutations).  It ensures that the proxy can:
    * **Correctly parse and interpret a wide range of CQL queries.**
    * **Generate the corresponding Bigtable requests, including data transformations and schema mappings.**
    * **Successfully execute these requests against Bigtable.**
    * **Transform the Bigtable responses back into the expected Cassandra format.**
    This comprehensive testing ensures that end-users can seamlessly interact with Bigtable using familiar CQL syntax without needing to worry about underlying code changes or data translation complexities.

* **Cross-Database Consistency:** Compare the output of Bigtable (via proxy) with Cassandra for the same test cases, ensuring consistency.

* **Automated End-to-End Integration Testing:** This framework provides a robust, automated end-to-end integration testing suite. It simulates real-world scenarios by performing actual database operations (inserts, selects, updates, deletes, and batch operations) on both Bigtable (through the proxy) and Cassandra. This ensures that the entire data flow, from CQL query submission to data retrieval, functions correctly and consistently across both systems. This eliminates manual testing efforts and provides continuous validation of the proxy's integration with Bigtable.

By automating these tests, the framework streamlines the validation process, reduces manual effort, and ensures the reliability and accuracy of the proxy in handling CQL queries against Bigtable.

## Architecture

The framework is structured as follows:

* **`schema_setup`:** Contains scripts for setting up the necessary schemas and infrastructure in both Bigtable and Cassandra to support the compliance tests.
    * **`bigtable_schema_setup.go`:** This Go script automates the creation of all required infrastructure and resources within Google Cloud Bigtable. This includes:
        * Provisioning the Cloud Bigtable instance itself.
        * Creating the essential `schema_mapping` table, which serves as a central registry for mapping Cassandra schemas to their Bigtable counterparts.
        * Generating all other tables required for executing the compliance test cases.

        This script relies on environment variables to obtain critical configuration details, such as the Bigtable instance ID, region, and project information.

        The script uses a `schema.sql` file containing CQL DDL statements. It assumes that the proxy is already running (either via Docker or locally). This leverages the proxy's DDL support feature to create the necessary tables and populate the schema_mapping table.

        The script also handles cleanup by dropping all created tables and deleting the Bigtable instance after the tests complete.
    * **`cassandra_schema_setup.go`:** This Go script automates the creation of keyspaces and tables within Cassandra. It reads the same `schema.sql` file used by the Bigtable setup script to maintain schema consistency across both databases. The script executes the CQL statements directly against the Cassandra instance to create the necessary keyspaces and tables as defined in the SQL file.
    * **`schema.sql`:** This SQL file contains CQL DDL statements for creating all tables used in the compliance tests, encompassing both Bigtable and Cassandra. It includes CREATE TABLE statements with column definitions, data types, and primary key information. The proxy's DDL support feature automatically handles the creation of tables in Bigtable and populates the schema_mapping table with the necessary configuration.

        Example of schema.sql content:
        ```sql
        CREATE TABLE keyspace.table1 (
            id bigint,
            name text,
            age int,
            timestamp_col timestamp,
            active boolean,
            balance float,
            credit double,
            metadata map<text, text>,
            tags set<text>,
            items list<text>,
            PRIMARY KEY ((id), name)
        );

        CREATE TABLE keyspace.table2 (
            user_id text,
            email text,
            created_at timestamp,
            PRIMARY KEY (user_id)
        );
        ```

        The proxy will automatically:
        - Create these tables in Bigtable
        - Set up necessary column families
        - Populate the schema_mapping table with the configuration

* **`test_files`:** Stores all the JSON files defining the test cases.
You can create multiple test files and place them here. You can run a single test file by providing the file name as an argument. Alternatively, you can give the file any name, but it must have the suffix `_test.json` so that the framework can automatically pick it up for execution.

#### Examples:

* `compliance_batch_test.json`: Tests batch operations.
* `compliance_delete_test.json`: Tests delete operations.
* `compliance_insert_test.json`: Tests insert operations.
* `compliance_limit_and_order_by_test.json`: Tests limit and order by clauses.
* `compliance_select_test.json`: Tests select operations.
* `compliance_update_test.json`: Tests update operations.

* **`utility`:** Contains utility functions and logging.
    * `logger.go`: Go script for logging functionalities.
    * `utils.go`: Go script with general utility functions.
* **`compliance_readme.md`:** This document.
* **`compliance_test.go`:** The main Go script that executes the compliance tests.

## Usage

### Prerequisites

* **Docker:** Required for building and running the proxy and Cassandra tests.
* **Go:** Required for building and running the test framework.
* **Google Cloud SDK (gcloud):** Required for interacting with Bigtable.
* **Cassandra:** A running Cassandra instance.

### Setup
1.  **Clone the Repository:**
    ```bash
    git clone <repository_url>
    cd <repository_directory>
    ```

2.  **Create a Docker Container (for Proxy, Cassandra Tests):**

    * `Bigtable Proxy`: Update ***`config.yaml`*** at root directory with correct details and execute the below command to create docker build

        ```bash
        docker build --no-cache -t bigtable-adaptor:compliance-test .
        ```
    * `Cassandra`: For cassandra we will use the cassandra image from docker-hub. No need to take any action for this.

    The image name which we are using is:
    ***`cassandra:latest`***
    
    > **Note:**  
     Ensure that the Cassandra keyspace to Bigtable instance mapping is correctly configured for compliance tests to run successfully. The mapping should look like:
    
     ```yaml
     instances:
       - bigtableInstance: bigtabledevinstance
         keyspace: bigtabledevinstance
       - bigtableInstance: bt-instance
         keyspace: cassandrakeysapce
    ```
    
    > The `bigtableInstance` value must match the actual Bigtable instance name, and the `keyspace` must correspond to the Cassandra keyspace being tested.

3.  **Environment Variables and Arguments:**
    Before running tests against the proxy, ensure you have a exported necessary environment variable..

    **Environment Variables:**

    * `GOOGLE_APPLICATION_CREDENTIALS`: Path to your Google Cloud service account credentials JSON file.
    * `GCP_PROJECT_ID`: Your Google Cloud project ID.
    * `ZONE`: Your Google Cloud zone.
    * `INTEGRATION_TEST_CRED_PATH`: Path to your integration test credentials secret.
    * `CONTAINER_IMAGE`: Name of your created docker image.

    **Arguments:**

    * `-target`: Specifies the target database. Options: `proxy`, `cassandra`.
    * `-testFileName`: (Optional) Specifies the JSON file containing the test cases. If omitted, all JSON files in the `test_files` directory will be executed.
    * `-enableFileLogging`: (Optional) Enables file logging. Set to `true` to log console output to a file.


### Running Tests

You can execute the tests using the `compliance_test` file.


**Examples:**

* **Run all tests against the proxy:**
    ```bash
    go test -run TestCompliance ./testing/compliance/compliance_test.go -target=proxy -v 
    ```

* **Run a specific test file against Cassandra:**
    ```bash
    go test -run TestCompliance ./testing/compliance/compliance_test.go -target=cassandra -v -testFileName=compliance_list_test.json
    ```

* **Run all tests against the proxy with file logging enabled:**
    ```bash
    go test -run TestCompliance ./testing/compliance/compliance_test.go -target=cassandra -v -enableFileLogging=true
    ```

### Test Case Format (JSON)

Test cases are defined in JSON files located within the `test_files` directory. Each JSON file represents a collection of operations designed to test specific functionalities of the proxy or database.

The JSON structure for a test case is as follows:

* **`title` (string):** A descriptive title for the test case, clearly indicating the functionality being tested.
* **`description` (string):** A detailed explanation of the test case, including the purpose and expected behavior.
* **`kind` (string):** Specifies the type of operation being tested (e.g., `dml`, `batch`) currently these 2 kinds are supported.
* **`operations` (array of objects):** An array containing one or more operation objects. Each operation object represents a single database query or action.
    * **`query` (string):** The CQL query to be executed.
    * **`query_type` (string):** The type of query (e.g., `INSERT`, `SELECT`, `DELETE`, `UPDATE`).
    * **`query_desc` (string):** A description of the query's purpose within the test case.
    * **`params` (array of objects):** An array of parameter objects, where each object defines a parameter used in the query.
        * **`value` (any):** The value of the parameter.
        * **`datatype` (string):** The data type of the parameter (e.g., `text`, `bigint`, `int`, `double`, `bigintTimestamp`).
    * **`expected_result` (array of objects or array of arrays of objects):** Defines the expected result of the query.
        * For `SELECT` queries, this is an array of objects, where each object represents a column and its expected value.
        * For `DELETE` or other operations that might return an error, this can include an object with `expect_error: true` and `error_message` containing the expected error message.
        * For `SELECT` queries returning multiple rows, you can use `expected_multi_row_result` with an array of arrays of objects.
    * **`expected_multi_row_result` (array of arrays of objects, optional):** Used for `SELECT` queries that are expected to return multiple rows. Each inner array represents a row, and each object within the row represents a column and its expected value.
    * **`expect_cassandra_specific_error` (string, optional):** Specifies an error message that is expected only when running against Cassandra.
    * **`default_diff` (boolean, optional):** A flag to control default result comparison behavior.
    * **`is_in_operation` (boolean, optional):** A flag to indicate if the query is part of an `IN` operation test.
* **`success_message` (string):** A message to display if the test case passes.
* **`failure_message` (string):** A message to display if the test case fails.

**Example Snippet:**

```json
{
"title": "Delete Operation with Timestamp, Cassandra should work fine but Bigtable should return error",
"description": "This test inserts a record into the 'test_table' table, attempts to delete it using a timestamp, and verifies that the delete operation fails with the expected error on Bigtable as USING TIMESTAMP could lead to data inconsistency due to the limitation of applying USING TIMESTAMP on scalar columns, we have not added the support of UT for delete operations but it will work on cassandra.",
"kind": "dml",
"operations": [
    {
    "query": "INSERT INTO bigtabledevinstance.user_info (name, age, code, credited) VALUES (?, ?, ?, ?)",
    "query_desc": "Insert a record to set up for the delete operation test.",
    "query_type": "INSERT",
    "params": [
        {
        "value": "Jhon",
        "datatype": "text"
        },
        {
        "value": 33,
        "datatype": "bigint"
        },
        {
        "value": 123,
        "datatype": "int"
        },
        {
        "value": 1500.5,
        "datatype": "double"
        }
    ],
    "expected_result": null
    },
    // ... other operations ...
    {
    "query": "DELETE FROM bigtabledevinstance.user_info USING TIMESTAMP ? WHERE name = ? AND age = ?",
    "query_desc": "Attempt to delete the record using a timestamp. Expect this to fail.",
    "query_type": "DELETE",
    "params": [
        {
        "value": "current",
        "datatype": "bigintTimestamp"
        },
        {
        "value": "Jhon",
        "datatype": "text"
        },
        {
        "value": 33,
        "datatype": "bigint"
        }
    ],
    "expected_result": [
        {
        "expect_error": true,
        "error_message": "error delete prepare query: delete using timestamp is not allowed"
        }
    ]
    }
],
"success_message": "Delete operation failed as expected with the correct error message.",
"failure_message": "Delete operation succeeded unexpectedly or returned an incorrect error message."
}
```

### Infrastructure Management (Proxy Tests)

For proxy tests, the framework automates the creation and teardown of the necessary Bigtable infrastructure, ensuring a clean and consistent testing environment.

**Bigtable Infrastructure Setup:**

1.  **Instance Creation:** The framework dynamically provisions a Bigtable instance in the specified region and zone. The required instance name, region, zone, and other configuration parameters are read from environment variables.
2.  **Schema and Table Creation:** Using the `schema.sql` file, the framework creates the essential `schema_mapping` table first. This table serves as a central registry, mapping Cassandra schemas to their corresponding Bigtable representations. Subsequently, all other tables required for the compliance test cases are created.
3.  **Schema Mapping Population:** The framework populates the `schema_mapping` table with details of all the created tables and their corresponding columns. This ensures that the proxy has the necessary information for translating CQL queries into Bigtable operations.
4.  **Automatic Teardown:** The framework leverages the `testcontainers-go` library, which automatically manages the lifecycle of the created Bigtable instance. Once the tests are completed, `testcontainers-go` ensures that the instance and all associated resources are automatically deleted, eliminating the need for manual cleanup.

**Cassandra Infrastructure Setup:**

1.  **Docker Image Retrieval:** For Cassandra tests, the framework utilizes Docker to create a Cassandra container. It fetches the official `cassandra:latest` Docker image from Docker Hub.
2.  **Container Provisioning:** A Cassandra container is provisioned using the retrieved image.
3.  **Schema and Table Creation:** Similar to Bigtable setup, the framework uses the `schema.sql` file to create the necessary keyspaces and tables within the Cassandra container.
4.  **Automatic Container Teardown:** `testcontainers-go` is also used to manage the Cassandra container's lifecycle. Upon completion of the tests, the container is automatically removed, ensuring a clean testing environment.
