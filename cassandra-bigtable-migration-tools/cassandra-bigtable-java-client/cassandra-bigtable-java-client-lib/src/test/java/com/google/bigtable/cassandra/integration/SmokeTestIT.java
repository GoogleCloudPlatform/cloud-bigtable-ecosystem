// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package com.google.bigtable.cassandra.integration;

import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import com.datastax.oss.driver.api.core.CqlSession;
import com.datastax.oss.driver.api.core.cql.BoundStatement;
import com.datastax.oss.driver.api.core.cql.PreparedStatement;
import com.datastax.oss.driver.api.core.cql.ResultSet;
import com.datastax.oss.driver.api.core.cql.Row;
import com.google.bigtable.cassandra.BigtableCqlConfiguration;
import com.google.bigtable.cassandra.BigtableCqlSessionFactory;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class SmokeTestIT {

  private static final Logger LOGGER = LoggerFactory.getLogger(SmokeTestIT.class);

  private String projectId;
  private String instanceId;
  private String smokeTestTable;
  private String schemaMappingTable;

  @BeforeEach
  public void setup() {
    projectId = System.getProperty("cassandra.bigtable.projectid");
    instanceId = System.getProperty("cassandra.bigtable.instanceid");
    smokeTestTable = System.getProperty("cassandra.bigtable.smoketesttable", "smoke_test_table");
    schemaMappingTable = System.getProperty("cassandra.bigtable.schemamappingtable",
        "smoke_test_schema_mapping_table");
  }

  @Test
  public void smokeTest() {
    // Create BigtableCqlConfiguration
    BigtableCqlConfiguration bigtableCqlConfiguration = createBigtableCqlConfiguration();

    LOGGER.info("Creating BigtableCqlSessionFactory with bigtableCqlConfiguration: "
        + bigtableCqlConfiguration.toString());

    // Create CqlSession with BigtableCqlConfiguration
    BigtableCqlSessionFactory bigtableCqlSessionFactory = new BigtableCqlSessionFactory(
        bigtableCqlConfiguration);

    LOGGER.info("Creating CqlSession");

    // Create CqlSession
    try (CqlSession session = bigtableCqlSessionFactory.newSession()) {

      String qualifiedTestTableName = instanceId + "." + smokeTestTable;

      // Create table
      String createTableQuery = String.format(
          "CREATE TABLE IF NOT EXISTS %s (column1 varchar PRIMARY KEY, column2 float, column3 int, column4 double, column5 bigint);",
          qualifiedTestTableName);
      LOGGER.info("creating table with query: " + createTableQuery);
      session.execute(createTableQuery);

      // Prepare an insert statement
      String insertQuery = String.format(
          "INSERT INTO %s (column1, column2, column3, column4, column5) VALUES (?, ?, ?, ?, ?)",
          qualifiedTestTableName);
      LOGGER.info("Preparing query: " + insertQuery);
      PreparedStatement preparedInsert = session.prepare(insertQuery);

      // Insert
      BoundStatement boundInsert1 = preparedInsert
          .bind()
          .setString("column1", "string1")
          .setFloat("column2", 1234.5f)
          .setInt("column3", 12345)
          .setDouble("column4", 1234.5d)
          .setLong("column5", 12345L);
      LOGGER.info("Inserting row 1");
      session.execute(boundInsert1);

      BoundStatement boundInsert2 = preparedInsert
          .bind()
          .setString("column1", "string2")
          .setFloat("column2", 6789.0f)
          .setInt("column3", 67890)
          .setDouble("column4", 6789.0d)
          .setLong("column5", 67890L);
      LOGGER.info("Inserting row 2");
      session.execute(boundInsert2);

      // Query for all entries
      String selectQuery = String.format("SELECT * FROM %s;", qualifiedTestTableName);
      LOGGER.info("Selecting data with query: " + selectQuery);
      ResultSet resultSet = session.execute(selectQuery);

      LOGGER.info("Inspecting result set");
      Assertions.assertNotNull(resultSet);

      List<Row> rows = getRows(resultSet);

      // Print rows to examine
      printRows(rows);

      // Verify row contents
      assertEquals(2, rows.size());
      assertEquals("string1", rows.get(0).getString("column1"));
      assertEquals(1234.5f, rows.get(0).getFloat("column2"));
      assertEquals(12345, rows.get(0).getInt("column3"));
      assertEquals(1234.5d, rows.get(0).getDouble("column4"));
      assertEquals(12345L, rows.get(0).getLong("column5"));

      assertEquals("string2", rows.get(1).getString("column1"));
      assertEquals(6789.0f, rows.get(1).getFloat("column2"));
      assertEquals(67890, rows.get(1).getInt("column3"));
      assertEquals(6789.0d, rows.get(1).getDouble("column4"));
      assertEquals(67890L, rows.get(1).getLong("column5"));
    }
  }

  private List<Row> getRows(ResultSet resultSet) {
    List<Row> rows = new ArrayList<>();
    for (Row row : resultSet) {
      rows.add(row);
    }
    return rows;
  }

  private void printRows(List<Row> rows) {
    StringBuilder resultSetString = new StringBuilder()
        .append(System.lineSeparator())
        .append("Rows:")
        .append(System.lineSeparator());
    rows.forEach(
        row -> resultSetString.append(row.getFormattedContents()).append(System.lineSeparator()));
    LOGGER.info(resultSetString.toString());
  }

  private BigtableCqlConfiguration createBigtableCqlConfiguration() {
    return BigtableCqlConfiguration.builder()
        .setProjectId(projectId)
        .setInstanceId(instanceId)
        .setSchemaMappingTable(schemaMappingTable)
        .build();
  }

}
