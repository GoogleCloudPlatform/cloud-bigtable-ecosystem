package com.google.cloud.bigtable.cassandra;

import com.datastax.oss.driver.api.core.CqlSession;
import com.datastax.oss.driver.api.core.cql.ResultSet;
import com.datastax.oss.driver.api.core.cql.Row;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import java.util.UUID;

import static org.junit.jupiter.api.Assertions.*;

public class DdlTests {
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
  public void testCreateTable() {
    String newTable = "test_create_" + UUID.randomUUID().toString().replace("-", "_");
    session.execute("CREATE TABLE " + newTable + " (id text PRIMARY KEY, value text)");
    session.execute("INSERT INTO " + newTable + " (id, value) VALUES ('1', 'foo')");
    ResultSet rs = session.execute("SELECT * FROM " + newTable + " WHERE id='1'");
    Row row = rs.one();
    assertNotNull(row, "Row should not be null after insertion into new table");
    assertEquals("foo", row.getString("value"));
    // clean up
    session.execute("DROP TABLE " + newTable);
  }

  @Test
  public void testAlterTable() {
    String tableToAlter = "test_alter_" + UUID.randomUUID().toString().replace("-", "_");
    session.execute("CREATE TABLE " + tableToAlter + " (id text PRIMARY KEY, value text)");
    session.execute("ALTER TABLE " + tableToAlter + " ADD new_col text");
    session.execute("INSERT INTO " + tableToAlter + " (id, value, new_col) VALUES ('1', 'foo', 'bar')");
    ResultSet rs = session.execute("SELECT * FROM " + tableToAlter + " WHERE id='1'");
    Row row = rs.one();
    assertNotNull(row, "Row should not be null after alter and insert");
    assertEquals("bar", row.getString("new_col"));
    // clean up
    session.execute("DROP TABLE " + tableToAlter);
  }

  @Test
  public void testDropTable() {
    String tableToDrop = "test_drop_" + UUID.randomUUID().toString().replace("-", "_");
    session.execute("CREATE TABLE " + tableToDrop + " (id text PRIMARY KEY, value text)");
    session.execute("DROP TABLE " + tableToDrop);

    // Selecting from a non-existent table should throw an exception
    assertThrows(Exception.class, () -> {
      session.execute("SELECT * FROM " + tableToDrop);
    }, "Selecting from a dropped table should throw an exception");
  }
}

