package com.google.cloud.bigtable.cassandra;

import com.datastax.oss.driver.api.core.CqlSession;
import com.datastax.oss.driver.api.core.cql.BatchStatement;
import com.datastax.oss.driver.api.core.cql.BatchType;
import com.datastax.oss.driver.api.core.cql.PreparedStatement;
import com.datastax.oss.driver.api.core.cql.ResultSet;
import com.datastax.oss.driver.api.core.cql.Row;
import com.datastax.oss.driver.api.core.cql.SimpleStatement;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import java.util.UUID;

import static org.junit.jupiter.api.Assertions.*;

public class BatchTests {

  private static CqlSession session;
  private static final String TABLE = "java_batch_test_table";

  @BeforeAll
  public static void setup() {
    session = Utils.createClient("bigtabledevinstance");
    session.execute("CREATE TABLE IF NOT EXISTS " + TABLE + " (id text PRIMARY KEY, value text)");
  }

  @AfterAll
  public static void teardown() {
    if (session != null) {
      session.close();
    }
  }

  @Test
  public void testLoggedBatchWithPreparedStatements() {
    String id1 = UUID.randomUUID().toString();
    String id2 = UUID.randomUUID().toString();

    PreparedStatement ps = session.prepare("INSERT INTO " + TABLE + " (id, value) VALUES (?, ?)");
    BatchStatement batch = BatchStatement.newInstance(BatchType.LOGGED)
        .add(ps.bind(id1, "val1"))
        .add(ps.bind(id2, "val2"));

    session.execute(batch);

    ResultSet rs = session.execute("SELECT * FROM " + TABLE + " WHERE id IN (?, ?)", id1, id2);
    int count = 0;
    for (Row row : rs) {
      count++;
    }
    assertEquals(2, count);
  }

  @Test
  public void testUnloggedBatchWithPreparedStatements() {
    String id3 = UUID.randomUUID().toString();
    String id4 = UUID.randomUUID().toString();

    PreparedStatement ps = session.prepare("INSERT INTO " + TABLE + " (id, value) VALUES (?, ?)");
    BatchStatement batch = BatchStatement.newInstance(BatchType.UNLOGGED)
        .add(ps.bind(id3, "val3"))
        .add(ps.bind(id4, "val4"));

    session.execute(batch);

    ResultSet rs = session.execute("SELECT * FROM " + TABLE + " WHERE id IN (?, ?)", id3, id4);
    int count = 0;
    for (Row row : rs) {
      count++;
    }
    assertEquals(2, count);
  }

  @Test
  public void testMixedOperationsInBatch() {
    String id5 = UUID.randomUUID().toString();
    session.execute("INSERT INTO " + TABLE + " (id, value) VALUES (?, ?)", id5, "initial");

    PreparedStatement psInsert = session.prepare("INSERT INTO " + TABLE + " (id, value) VALUES (?, ?)");
    PreparedStatement psUpdate = session.prepare("UPDATE " + TABLE + " SET value = ? WHERE id = ?");
    PreparedStatement psDelete = session.prepare("DELETE FROM " + TABLE + " WHERE id = ?");

    String id6 = UUID.randomUUID().toString();
    BatchStatement batch = BatchStatement.newInstance(BatchType.LOGGED)
        .add(psInsert.bind(id6, "val6"))
        .add(psUpdate.bind("updated", id5))
        .add(psDelete.bind(id6));

    session.execute(batch);

    Row row5 = session.execute("SELECT * FROM " + TABLE + " WHERE id = ?", id5).one();
    assertNotNull(row5);
    assertEquals("updated", row5.getString("value"));

    Row row6 = session.execute("SELECT * FROM " + TABLE + " WHERE id = ?", id6).one();
    assertNull(row6, "id6 should have been deleted if it was inserted and then deleted in the same batch");
  }

  @Test
  public void testBatchWithSimpleStatementsFails() {
    // According to proxy documentation and analysis, raw query strings in batches are not supported.
    BatchStatement batch = BatchStatement.newInstance(BatchType.LOGGED)
        .add(SimpleStatement.newInstance("INSERT INTO " + TABLE + " (id, value) VALUES ('simple1', 'val1')"));

    assertThrows(Exception.class, () -> {
      session.execute(batch);
    }, "Batch with SimpleStatement should fail as it's not supported by the proxy");
  }
}
