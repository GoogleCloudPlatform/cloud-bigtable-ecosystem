# Kafka Connect Bigtable Sink Connector

This sink connector is for writing Kafka messages to the Google Bigtable database.

## Features

* Flexible key mapping
* Automatic table and column family creation
* Deletes
* At least once delivery
* Dead Letter Queue

## Configuration

To use this sink connector, set the connector class to the following:

`connector.class = com.google.cloud.kafka.connect.bigtable.BigtableSinkConnector`


### Connectivity

`gcp.bigtable.instance.id`

The ID of the Cloud Bigtable instance.

* Type: string
* Default:
* Valid Values: non-null string, non-empty string
* Importance: high


`gcp.bigtable.project.id`

The ID of the GCP project.

* Type: string
* Default:
* Valid Values: non-null string, non-empty string
* Importance: high


`gcp.bigtable.credentials.json`

The path to the JSON service key file. Configure at most one of `gcp.bigtable.credentials.path` and `gcp.bigtable.credentials.json`. If neither is provided, Application Default Credentials will be used.

* Type: string
* Default: null
* Valid Values: 
* Importance: high


`gcp.bigtable.credentials.path`

The path to the JSON service key file. Configure at most one of `gcp.bigtable.credentials.path` and `gcp.bigtable.credentials.json`. If neither is provided, Application Default Credentials will be used.

* Type: string
* Default: null
* Valid Values: 
* Importance: high

`gcp.bigtable.app.profile.id`

The application profile that the connector should use. If none is supplied, the default app profile will be used.

* Type: string
* Default: null
* Valid Values:
* Importance: medium


### Writes

`insert.mode`

Defines the insertion mode to use. Supported modes are:
- insert - Insert new record only. If the row to be written already exists in the table, an error is thrown.
- upsert - If the row to be written already exists, then its column values are overwritten with the ones provided.

* Type: string
* Default: INSERT
* Valid Values: (case insensitive) [UPSERT, INSERT]
* Importance: high


`auto.create.column.families`

Whether to automatically create missing columns families in the table relative to the record schema.
Does not imply auto-creation of tables.
When enabled, the records for which the auto-creation fails, are failed.
When enabled, column families will be created also for deletions of nonexistent column families and cells within them.
Recreation of column families deleted by other Cloud Bigtable users is not supported.
Note that column family auto-creation is slow. It may slow down not only the records targeting nonexistent column families, but also other records batched with them. To facilitate predictable latency leave this option disabled.

* Type: boolean
* Default: false
* Valid Values: non-null string
* Importance: medium


`auto.create.tables`

Whether to automatically create the destination table if it is found to be missing.
When enabled, the records for which the auto-creation fails, are failed.
Recreation of tables deleted by other Cloud Bigtable users is not supported.
Note that table auto-creation is slow (multiple seconds). It may slow down not only the records targeting nonexistent tables, but also other records batched with them. To facilitate predictable latency leave this option disabled.

* Type: boolean
* Default: false
* Valid Values: non-null string
* Importance: medium


`default.column.family`

Any root-level fields on the SinkRecord that aren't objects will be added to this column family. If empty, the fields will be ignored. Use `${topic}` within the column family name to specify the originating topic name.

* Type: string
* Default: ${topic}
* Valid Values: 
* Importance: medium


`default.column.qualifier`

Any root-level values on the SinkRecord that aren't objects will be added to this column within default column family. If empty, the value will be ignored.

* Type: string
* Default: KAFKA_VALUE
* Valid Values: 
* Importance: medium


`max.batch.size`

The maximum number of records that can be batched into a batch of upserts. Note that since only a batch size of 1 for inserts is supported, `max.batch.size` must be exactly `1` when `insert.mode` is set to `INSERT`.

* Type: int
* Default: 1
* Valid Values: [1,...]
* Importance: medium


`row.key.definition`

A comma separated list of Kafka Record key field names that specifies the order of Kafka key fields to be concatenated to form the row key.

For example the list: `username, post_id, time_stamp` when applied to a Kafka key: `{'username': 'bob','post_id': '213', 'time_stamp': '123123'}` and with delimiter `#` gives the row key `bob#213#123123`. You can also access terms nested in the key by using `.` as a delimiter. If this configuration is empty or unspecified and the Kafka Message Key is a
- struct, all the fields in the struct are used to construct the row key.
- byte array, the row key is set to the byte array as is.
- primitive, the row key is set to the primitive stringified. If prefixes, more complicated delimiters, and string constants are required in your Row Key, consider configuring an SMT to add relevant fields to the Kafka Record key.

* Type: list
* Default: ""
* Valid Values: 
* Importance: medium


`table.name.format`

Name of the destination table. Use `${topic}` within the table name to specify the originating topic name.

For example, `user_${topic}` for the topic `stats` will map to the table name `user_stats`.

* Type: string
* Default: ${topic}
* Valid Values: non-null string, non-empty string
* Importance: medium


`value.null.mode`

Defines what to do with `null`s within Kafka values. Supported modes are:
- write - Serialize `null`s to empty byte arrays.
- ignore - Ignore `null`s.
- delete - Use them to issue DELETE commands. Root-level `null` deletes a row. `null` nested one level deletes a column family named after the `null`-valued field. `null` nested two levels deletes a column named after the `null`-valued field in column family named after the `null-valued` field parent field. `null` values nested more than two levels are serialized like other values and don't result in any DELETE commands.

* Type: string
* Default: WRITE
* Valid Values: (case insensitive) [DELETE, IGNORE, WRITE]
* Importance: medium


`row.key.delimiter`

The delimiter used in concatenating Kafka key fields in the row key. If this configuration is empty or unspecified, the key fields will be concatenated together directly.

* Type: string
* Default: ""
* Valid Values: 
* Importance: low


### Error Handling

`error.mode`

Specifies how to handle errors that result from writes, after retries. It is ignored if DLQ is configured. Supported modes are:
- fail - The connector fails and must be manually restarted.
- warn - The connector logs a warning and continues operating normally.
- ignore - The connector does not log a warning but continues operating normally.

* Type: string
* Default: FAIL
* Valid Values: (case insensitive) [IGNORE, FAIL, WARN]
* Importance: medium

`retry.timeout.ms`

Maximum time in milliseconds allocated for retrying database operations before trying other error handling mechanisms.

* Type: long
* Default: 90000 (90 seconds)
* Valid Values: [0,...]
* Importance: medium


## Usage



## Code organization
The maven project is split into two modules:
- [sink](sink) - the sink and its unit tests
- [integration-tests](integration-tests) - the integration tests

This split enables two desirable properties for the integration tests:
- the versions of dependencies used by the integration tests and the sink may be different (since Kafka Connect isolates connectors' class loaders automatically),
- the sink is provided to the integration tests with a directory of jars just like in a real Kafka Connect deployment.

## Tests
For details on running the tests, please see [doc/tests.md](doc/tests.md).

## Performance test setup
The performance test setup is described in detail in [doc/performance/README.md](doc/performance/README.md).
