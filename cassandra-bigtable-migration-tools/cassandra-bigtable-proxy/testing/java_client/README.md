# Cassandra Proxy Java Client Tests

This directory contains a simple Java test suite for the Cassandra Bigtable Proxy.

## Why is this necessary?

We've seen the Java client fail to process responses from the Bigtable Proxy even when other clients, like gocql, don't. 
Additionally, we didn't have a test client that supported named markers, which the Java client does.  

## Prerequisites

- Java 11 or higher
- Maven
- A running instance of the Cassandra Bigtable Proxy on `localhost:9042` with a `bigtabledevinstance` keyspace

## Running the Tests

To run the tests, execute the following command in this directory:

```bash
mvn test
```
