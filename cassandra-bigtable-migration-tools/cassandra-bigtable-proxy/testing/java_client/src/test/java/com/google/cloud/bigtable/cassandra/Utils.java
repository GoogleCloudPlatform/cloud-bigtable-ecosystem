package com.google.cloud.bigtable.cassandra;

import com.datastax.oss.driver.api.core.CqlSession;
import com.datastax.oss.driver.api.core.CqlSessionBuilder;
import com.google.cloud.bigtable.data.v2.BigtableDataClient;
import com.google.cloud.bigtable.data.v2.BigtableDataSettings;
import com.google.cloud.bigtable.data.v2.models.RowMutation;

import java.io.IOException;
import java.net.InetSocketAddress;

public class Utils {

  public static CqlSession createClient() {
    return createClient(null);
  }

  public static CqlSession createClient(String keyspace) {
    CqlSessionBuilder cqlSessionBuilder = CqlSession.builder()
        .addContactPoint(new InetSocketAddress("127.0.0.1", 9042))
        .withLocalDatacenter("datacenter1");

    if (keyspace != null) {
      cqlSessionBuilder
          .withKeyspace(keyspace);
    }

    return cqlSessionBuilder.build();
  }

  /**
   * A testing utility function for subverting the Proxy's schema cache.
   */
  public static void addSchemaMappingColumn(String tableName, String columnName, String columnType) {
    RowMutation mut = RowMutation.create("schema_mapping", String.format("%s#%s", tableName, columnName))
        .setCell("cf", "ColumnName", columnName)
        .setCell("cf", "ColumnType", columnType)
        .setCell("cf", "TableName", tableName);
    writeToBigtable(mut);
  }

  public static void writeToBigtable(RowMutation mut) {
    String projectId = System.getenv().get("PROJECT_ID");
    String instanceId = System.getenv().get("INSTANCE_ID");

    BigtableDataSettings.Builder settingsBuilder = BigtableDataSettings.newBuilder()
        .setProjectId(projectId)
        .setInstanceId(instanceId);

    try (BigtableDataClient dataClient = BigtableDataClient.create(settingsBuilder.build())) {
      dataClient.mutateRow(mut);
    } catch (IOException e) {
      throw new RuntimeException("Failed to write to Bigtable", e);
    }
  }
}
