package com.google.cloud.bigtable.cassandra;

import com.datastax.oss.driver.api.core.CqlSession;
import com.datastax.oss.driver.api.core.cql.ResultSet;
import com.datastax.oss.driver.api.core.cql.Row;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import java.net.InetSocketAddress;
import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

public class SystemTableTest {

  private static CqlSession session;

  @BeforeAll
  public static void setup() {
    session = Utils.createClient();
  }

  @AfterAll
  public static void teardown() {
    if (session != null) {
      session.close();
    }
  }

  @Test
  public void testQuerySystemLocal() {
    ResultSet rs = session.execute("SELECT cluster_name, release_version, dse_version FROM system.local");
    Row row = rs.one();
    assertNotNull(row, "system.local should return at least one row");

    String clusterName = row.getString("cluster_name");
    assertNotNull(clusterName);
    assertTrue(clusterName.contains("cassandra-bigtable-proxy") || clusterName.contains("Test Cluster"),
        "Cluster name should indicate it's the proxy or a test cluster");

    assertNotNull(row.getString("release_version"));
  }

  @Test
  public void testQuerySystemSchemaKeyspaces() {
    ResultSet rs = session.execute("SELECT keyspace_name FROM system_schema.keyspaces");
    List<String> keyspaces = new ArrayList<>();
    for (Row row : rs) {
      keyspaces.add(row.getString("keyspace_name"));
    }

    assertFalse(keyspaces.isEmpty(), "Should have at least one keyspace");
    assertTrue(keyspaces.contains("system") || keyspaces.contains("system_schema"),
        "Should contain standard system keyspaces");
  }

  @Test
  public void testQuerySystemSchemaTables() {
    ResultSet rs = session.execute("SELECT table_name FROM system_schema.tables WHERE keyspace_name = 'system'");
    List<String> tables = new ArrayList<>();
    for (Row row : rs) {
      tables.add(row.getString("table_name"));
    }

    assertFalse(tables.isEmpty(), "Should have tables in 'system' keyspace");
    assertTrue(tables.contains("local"), "Should contain 'local' table in 'system' keyspace");
  }

  @Test
  public void testQuerySystemSchemaColumns() {
    ResultSet rs = session.execute("SELECT column_name, type FROM system_schema.columns " +
        "WHERE keyspace_name = 'system' AND table_name = 'local'");
    List<String> columns = new ArrayList<>();
    for (Row row : rs) {
      columns.add(row.getString("column_name"));
    }

    assertFalse(columns.isEmpty(), "Should have columns for system.local");
    assertTrue(columns.contains("cluster_name"), "system.local should have 'cluster_name' column");
    assertTrue(columns.contains("release_version"), "system.local should have 'release_version' column");
  }
}
