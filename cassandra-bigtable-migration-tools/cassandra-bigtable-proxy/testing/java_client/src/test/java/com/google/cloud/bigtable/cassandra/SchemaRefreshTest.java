package com.google.cloud.bigtable.cassandra;

import com.datastax.oss.driver.api.core.CqlSession;
import com.datastax.oss.driver.api.core.cql.ResultSet;
import com.datastax.oss.driver.api.core.cql.Row;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import java.util.UUID;

import static org.junit.jupiter.api.Assertions.*;

public class SchemaRefreshTest {
  private static CqlSession session;

  @BeforeAll
  public static void setup() {
    session = Utils.createClient("bigtabledevinstance");
  }

  @AfterAll
  public static void teardown() {
    if (session != null) {
      session.close();
    }
  }

  @Test
  public void testSchemaAutoRefresh() throws InterruptedException {
    String tableName = "refresh_test_" + UUID.randomUUID().toString().replace("-", "_");
    session.execute("CREATE TABLE " + tableName + " (id text PRIMARY KEY, original_col text)");

    // Manually add a column to the schema_mapping table in Bigtable, bypassing the proxy.
    // This simulates another proxy instance making a schema change.
    String newColumn = "external_col";
    Utils.addSchemaMappingColumn(tableName, newColumn, "text");

    // Attempt to insert data into the new column until it succeeds (max 60 seconds).
    // The proxy should pick up the change within 30 seconds.
    boolean success = false;
    long startTime = System.currentTimeMillis();
    while (System.currentTimeMillis() - startTime < 60000) {
      try {
        session.execute("INSERT INTO " + tableName + " (id, original_col, " + newColumn + ") VALUES ('1', 'foo', 'bar')");
        success = true;
        break;
      } catch (Exception e) {
        // Expected until the proxy refreshes its schema cache.
        Thread.sleep(2000);
      }
    }

    assertTrue(success, "Proxy should have picked up the new column within the refresh period.");

    // Verify the data can be read back.
    ResultSet rs = session.execute("SELECT * FROM " + tableName + " WHERE id='1'");
    Row row = rs.one();
    assertNotNull(row);
    assertEquals("bar", row.getString(newColumn));

    // Cleanup
    session.execute("DROP TABLE " + tableName);
  }
}
