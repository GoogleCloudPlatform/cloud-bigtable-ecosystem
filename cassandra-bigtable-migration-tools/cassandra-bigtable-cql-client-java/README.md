# Introduction

The Bigtable CQL Client for Java allows your Java applications using Apache Cassandra, to connect seamlessly to a Bigtable instance.

This client acts as a local tcp proxy, intercepting the raw Cassandra protocol bytes sent by a your cassandra driver or cqlsh. Responses from Bigtable are translated back into the Cassandra wire format and sent back to the originating driver or tool.

See [README here](./google-cloud-bigtable-cassandra-proxy-lib/README.md).
