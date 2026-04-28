/*
 * Copyright 2025 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package com.google.cloud.aerospike;

import com.google.cloud.aerospike.exceptions.JNIException;
import com.google.cloud.aerospike.exceptions.MalformedBackupDataException;
import com.google.cloud.aerospike.exceptions.UnsupportedBackupEntryException;
import com.google.cloud.aerospike.values.Value;
import com.google.cloud.bigtable.admin.v2.BigtableTableAdminClient;
import com.google.cloud.bigtable.admin.v2.BigtableTableAdminSettings;
import com.google.cloud.bigtable.admin.v2.models.CreateTableRequest;
import com.google.cloud.bigtable.data.v2.BigtableDataClient;
import com.google.cloud.bigtable.data.v2.BigtableDataSettings;
import com.google.cloud.bigtable.data.v2.models.Query;
import com.google.cloud.bigtable.data.v2.models.Row;
import com.google.cloud.bigtable.data.v2.models.RowCell;
import com.google.protobuf.ByteString;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.util.Base64;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Example of loading a backup file into Bigtable.
 *
 * <p>This example demonstrates how to create a Bigtable table, stream a backup file, and load the
 * data into the table. It uses a named pipe to stream the backup file and reads the data line by
 * line.
 */
public class LoadBackupExample {

  private static final String PROJECT_ID = "unoperate-test";
  private static final String INSTANCE_ID =
      "instance" + System.getenv("BIGTABLE_EMULATOR_HOST") != null
          ? String.valueOf(System.currentTimeMillis())
          : "1";
  private static final String TABLE_ID = "table1";
  private static final Logger LOG = LoggerFactory.getLogger(LoadBackupExample.class);

  public static class SubProcess {
    private final String[] command;
    private Process process;

    public SubProcess(String[] command) {
      this.command = command;
    }

    /**
     * Starts the process.
     *
     * @param wait If true, waits for the process to finish before returning.
     * @throws IOException If an I/O error occurs.
     * @throws InterruptedException If the current thread is interrupted while waiting.
     */
    public void start(boolean wait) throws IOException, InterruptedException {
      ProcessBuilder pb = new ProcessBuilder(command);
      process = pb.start();
      if (wait) {
        process.waitFor();
      }
    }

    public Process getProcess() {
      return process;
    }
  }

  public static void main(String[] args) throws IOException, InterruptedException {
    BigtableDataSettings settings =
        BigtableDataSettings.newBuilder()
            .setProjectId(PROJECT_ID)
            .setInstanceId(INSTANCE_ID)
            .build();
    BigtableTableAdminSettings adminSettings =
        BigtableTableAdminSettings.newBuilder()
            .setProjectId(PROJECT_ID)
            .setInstanceId(INSTANCE_ID)
            .build();
    try (BigtableDataClient dataClient = BigtableDataClient.create(settings);
        BigtableTableAdminClient adminClient = BigtableTableAdminClient.create(adminSettings)) {

      // Checks if table exists, creates it if it does not.
      if (!adminClient.exists(TABLE_ID)) {
        LOG.info("Creating table: {}", TABLE_ID);
        CreateTableRequest createTableRequest =
            CreateTableRequest.of(TABLE_ID)
                .addFamily(RowBuilder.SCALAR_FAMILY)
                .addFamily(RowBuilder.COMPLEX_OBJECT_FAMILY);
        adminClient.createTable(createTableRequest);
        System.err.printf("Table %s created successfully%n", TABLE_ID);
      }
      LOG.info("Starting...");

      String pipePath = "my_fifo";

      SubProcess createPipe = new SubProcess(new String[] {"mkfifo", pipePath});
      createPipe.start(true);
      LOG.info("pipe created");

      String backupPath = "backup-loader-example/aerospike/example_files/backup_advanced_types.asb";
      CompletableFuture<Void> fileStreaming =
          CompletableFuture.runAsync(
              () -> {
                try (InputStream input = new FileInputStream(backupPath);
                    FileOutputStream pipe = new FileOutputStream(pipePath)) {
                  LOG.info("file streaming");
                  input.transferTo(pipe);
                } catch (IOException e) {
                  throw new RuntimeException(e);
                }
              });

      try (BackupReader br = new BackupReader(pipePath, BackupReader.CompressionAlgorithm.NONE)) {
        while (true) {
          LOG.info("Reading a record");
          try {
            ReadRecordResult result = br.readRecord("test");
            if (result == null) {
              LOG.info("EOF DETECTED!");
              break;
            } else {
              LOG.info("RESULT {}", result.toJson());
              Map<String, Value> map = result.bins;
              BigtableMutationBuilder bigtableMutationBuilder =
                  new BigtableMutationBuilder(TABLE_ID, ByteString.copyFrom(result.digest), 0L);

              for (String key : map.keySet()) {
                LOG.info("BinName: {} : {}", key, map.get(key));
                try {
                  map.get(key).setCell(key, bigtableMutationBuilder);
                } catch (Throwable t) {
                  LOG.error("Failed to set mutation cell using Aerospike value.", t);
                  LOG.error("Dropping value: {}", map.get(key));
                }
              }

              dataClient.mutateRow(bigtableMutationBuilder.getRowMutation());
            }
          } catch (MalformedBackupDataException e) {
            LOG.error("Failed to read a malformed record.", e);
          } catch (UnsupportedBackupEntryException e) {
            LOG.error("Unsupported record type encountered.", e);
          } catch (JNIException e) {
            LOG.error("Internal error while reading a record.", e);
          } catch (IOException e) {
            LOG.error("I/O error while reading a record.", e);
          }
        }
        LOG.info("Successful end of reading! Here are the Bigtable rows:");
        List<Row> rows = dataClient.readRows(Query.create(TABLE_ID)).stream().toList();
        for (Row row : rows) {
          System.out.println(
              "Row: " + Base64.getEncoder().encodeToString(row.getKey().toByteArray()));
          for (RowCell cell : row.getCells()) {
            System.out.println(
                "Cell: "
                    + cell.getFamily()
                    + "/"
                    + cell.getQualifier().toStringUtf8()
                    + "@"
                    + cell.getTimestamp());
          }
        }
        fileStreaming.get();
      } catch (Throwable t) {
        LOG.error("Unexpected exception caught.", t);
      } finally {
        SubProcess removePipe = new SubProcess(new String[] {"/usr/bin/rm", pipePath});
        removePipe.start(true);
        LOG.info("pipe removed");
      }
    }
  }
}
