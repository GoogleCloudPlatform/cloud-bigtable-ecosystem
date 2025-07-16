# Cassandra to Bigtable Client for Java

The Cassandra to Bigtable Client for Java lets you integrate your Cassandra-based Java applications with Bigtable using CQL.

# How to build

- Ensure you have the installed [these pre-requisites](../../cassandra-bigtable-proxy/README.md#pre-requisites)
- Navigate to the parent directory (`cassandra-bigtable-java-client`)
- Run the following Maven command, replacing `<SPECIFY-PROFILE-HERE>` with one from [this list of classifiers](#classifiers)

```shell
mvn install -P<SPECIFY-PROFILE-HERE>
```

# How to include this dependency in your code

Add the following dependency to the `<dependencyManagement><dependencies>` section of your Maven `pom.xml`:

```xml
<dependency>
  <groupId>com.google.cloud</groupId>
  <artifactId>cassandra-bigtable-java-client-bom</artifactId>
  <version>0.1.0-SNAPSHOT</version>
  <type>pom</type>
  <scope>import</scope>
</dependency>
```

Add the dependencies below to the `<dependencies>` section of your Maven `pom.xml` file, replacing `SPECIFY-CLASSIFIER-HERE` with one from [this list of classifiers](#classifiers)

```xml
<dependency>
  <groupId>com.google.cloud</groupId>
  <artifactId>cassandra-bigtable-java-client-lib</artifactId>
  <classifier>SPECIFY-CLASSIFIER-HERE</classifier>
</dependency>
<dependency>
  <groupId>org.apache.cassandra</groupId>
  <artifactId>java-driver-core</artifactId>
  <version>4.19.0</version>
</dependency>
```

## Classifiers

For the `cassandra-bigtable-java-client-core` package, specify the relevant `<classifier>` corresponding to the target platform/architecture:

- Linux Builds
  - `linux-amd64`
  - `linux-arm64`
- Mac Builds
  - `darwin-amd64`
  - `darwin-arm64`

Note: Windows is currently not supported.

# How to use

First ensure that these [setup steps](../../cassandra-bigtable-proxy/README.md##setting-up-bigtable-instance-and-schema-configuration) have been completed.

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
