# Introduction

The **Cassandra Bigtable Java Client** allows your Cassandra-based Java applications to integrate with Bigtable using Cassandra Query Language (CQL). The library is a drop-in replacement for Cassandra drivers in your Java applications.

# How to include this dependency in your code

Add the following dependency to your Maven `pom.xml`, replacing `VERSION-HERE` with the latest version.

> **The latest version is: ` 0.1.8 `** <!-- {x-version-update:cassandra-bigtable-java-client-parent:current} -->

```xml
<dependencyManagement>
  <dependencies>
    <dependency>
      <groupId>com.google.cloud</groupId>
      <artifactId>cassandra-bigtable-java-client-bom</artifactId>
      <version>VERSION-HERE</version>
      <type>pom</type>
      <scope>import</scope>
    </dependency>
  </dependencies>
</dependencyManagement>
```

Then, add the dependencies below to your Maven `pom.xml` file (replacing `SPECIFY-CLASSIFIER-HERE` with one from [this list of classifiers](#classifiers)):

```xml
<dependencies>
  <dependency>
    <groupId>com.google.cloud</groupId>
    <artifactId>cassandra-bigtable-java-client-lib</artifactId>
    <classifier>SPECIFY-CLASSIFIER-HERE</classifier>
    <version>VERSION-HERE</version>
  </dependency>
  <dependency>
    <groupId>org.apache.cassandra</groupId>
    <artifactId>java-driver-core</artifactId>
    <version>4.19.0</version>
  </dependency>
</dependencies>
```

`org.apache.cassandra:java-driver-core:4.19.0` is the latest supported version.

## Classifiers

Below are the values of `<classifier>` corresponding to different target platform/architectures:

- Linux Builds
  - `linux-amd64`
  - `linux-arm64`
- Mac Builds
  - `darwin-amd64`
  - `darwin-arm64`

Note: Windows is currently not supported.

# How to use

Example usage below:

```java
// Imports

import com.google.bigtable.cassandra.BigtableCqlConfiguration;
import com.google.bigtable.cassandra.BigtableCqlSessionFactory;
import com.datastax.oss.driver.api.core.CqlSession;

class MyClass {

  void MyMethod() {
    // Specify Bigtable schema configuration
    BigtableCqlConfiguration bigtableCqlConfiguration = BigtableCqlConfiguration.builder()
            .setProjectId("someProjectId")
            .setInstanceId("someInstanceId")
            .setDefaultColumnFamily("someDefaultColumnFamily")
            .setBigtableChannelPoolSize(4)
            .disableOpenTelemetry()
            .build();

    BigtableCqlSessionFactory bigtableCqlSessionFactory = new BigtableCqlSessionFactory(bigtableCqlConfiguration);

    // Create CqlSession
    try (CqlSession session = bigtableCqlSessionFactory.newSession()) {
      // Execute query
      PreparedStatement preparedInsert = session.prepare("<query here>");
      // ...
    }
  }
}
```

Additional examples can be found [here](../example) and [here](./src/test/java/com/google/bigtable/cassandra/integration/SmokeTestIT.java).

# Supported Cassandra versions

See [here](../../cassandra-bigtable-proxy/README.md)

# Configuring CQL session

To further configure the CQL session, add an `application.conf` file to your classpath with the relevant settings.

For example, to increase request timeout:

```properties
datastax-java-driver {
  basic {
    request {
      timeout = 5 seconds
    }
  }
}
```

See [here](https://docs.datastax.com/en/developer/java-driver/latest/manual/core/configuration/index.html) for details.
