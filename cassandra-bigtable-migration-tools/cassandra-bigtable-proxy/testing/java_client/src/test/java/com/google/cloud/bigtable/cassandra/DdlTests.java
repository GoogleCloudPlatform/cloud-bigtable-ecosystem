package com.google.cloud.bigtable.cassandra;

import com.datastax.oss.driver.api.core.CqlSession;
import com.datastax.oss.driver.api.core.cql.BoundStatement;
import com.datastax.oss.driver.api.core.cql.ResultSet;
import com.datastax.oss.driver.api.core.cql.Row;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import java.time.Instant;
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

  @Test
  public void testReadUndefinedScalars() {
    // 1. Write row with only partial columns (primitives left out)
    session.execute(
        "INSERT INTO bigtabledevinstance.user_info (name, age, extra_info) " +
            "VALUES ('undefined_primitives_test', 1, {'foo': 'bar'})"
    );

    // 2. Select the columns that weren't explicitly inserted
    Row row = session.execute(
        "SELECT code, credited, text_col, balance, is_active, birth_date, zip_code " +
            "FROM bigtabledevinstance.user_info " +
            "WHERE name='undefined_primitives_test' AND age=1"
    ).one();

    assertNotNull(row);

    assertEquals(0, row.getInt("code"));
    assertEquals(0.0, row.getDouble("credited"), 0.001);

    assertNull(row.getString("text_col"));

    assertEquals(0.0f, row.getFloat("balance"), 0.001f);
    assertFalse(row.getBoolean("is_active"));

    assertNull(row.getInstant("birth_date"));

    assertEquals(0L, row.getLong("zip_code"));
  }

  @Test
  public void testReadDefaultScalarValues() {
    session.execute(
        "INSERT INTO bigtabledevinstance.user_info (name, age, code, credited, text_col, balance, is_active, birth_date, zip_code) " +
            "VALUES ('undefined_primitives_test', 1, 0, 0, '', 0, false, 0, 0)"
    );

    // 2. Select the columns that weren't explicitly inserted
    Row row = session.execute(
        "SELECT code, credited, text_col, balance, is_active, birth_date, zip_code " +
            "FROM bigtabledevinstance.user_info " +
            "WHERE name='undefined_primitives_test' AND age=1"
    ).one();

    assertNotNull(row);

    assertEquals(0, row.getInt("code"));
    assertEquals(0.0, row.getDouble("credited"), 0.001);

    assertEquals("", row.getString("text_col"));

    assertEquals(0.0f, row.getFloat("balance"), 0.001f);
    assertFalse(row.getBoolean("is_active"));

    assertEquals(Instant.EPOCH, row.getInstant("birth_date"));

    assertEquals(0L, row.getLong("zip_code"));
  }
}

