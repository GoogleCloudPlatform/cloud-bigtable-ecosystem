package com.google.cloud.kafka.connect.bigtable.writers;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertNotNull;
import static org.junit.Assert.assertThrows;


import com.google.api.core.ApiFuture;
import com.google.cloud.bigtable.admin.v2.BigtableTableAdminClient;
import com.google.cloud.bigtable.admin.v2.BigtableTableAdminSettings;
import com.google.cloud.bigtable.admin.v2.models.CreateTableRequest;
import com.google.cloud.bigtable.data.v2.BigtableDataClient;
import com.google.cloud.bigtable.data.v2.BigtableDataSettings;
import com.google.cloud.bigtable.data.v2.models.Mutation;
import com.google.cloud.bigtable.data.v2.models.Row;
import com.google.cloud.bigtable.data.v2.models.RowMutation;
import com.google.cloud.bigtable.data.v2.models.TableId;
import com.google.cloud.bigtable.emulator.v2.BigtableEmulatorRule;
import com.google.cloud.kafka.connect.bigtable.mapping.MutationData;
import com.google.protobuf.ByteString;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.HashSet;
import java.util.concurrent.ExecutionException;
import org.apache.kafka.connect.errors.ConnectException;
import org.apache.kafka.connect.sink.SinkRecord;
import org.junit.Assert;
import org.junit.Before;
import org.junit.Rule;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public class BigtableInsertWriterTest {

  // Initialize the emulator Rule
  @Rule
  public final BigtableEmulatorRule bigtableEmulator = BigtableEmulatorRule.create();

  private BigtableDataClient dataClient;
  private final String tableIdString = "table1";
  private final TableId tableId = TableId.of(tableIdString);
  private final String cf = "cf";

  @Before
  public void setUp() throws IOException {
    // Initialize the clients to connect to the emulator
    BigtableTableAdminSettings.Builder tableAdminSettings = BigtableTableAdminSettings.newBuilderForEmulator(
            bigtableEmulator.getPort())
        .setInstanceId("my-instance")
        .setProjectId("my-project");
    var tableAdminClient = BigtableTableAdminClient.create(tableAdminSettings.build());

    BigtableDataSettings.Builder dataSettings = BigtableDataSettings.newBuilderForEmulator(
            bigtableEmulator.getPort())
        .setInstanceId("my-instance")
        .setProjectId("my-project");
    dataClient = BigtableDataClient.create(dataSettings.build());

    // Create a test table that can be used in tests
    tableAdminClient.createTable(
        CreateTableRequest.of(tableIdString)
            .addFamily(cf)
    );
  }

  @Test
  public void testInsertRows() throws ExecutionException, InterruptedException {
    var writer = new BigtableInsertWriter(dataClient);

    SinkRecord record = new SinkRecord("", 1, null, null, null, null, 1);

    var mut = Mutation.create()
        .setCell(cf, "col1", 1)
        .setCell(cf, "col2", 2);

    var key1 = ByteString.copyFrom("key1".getBytes(StandardCharsets.UTF_8));
    var md1 = new MutationData(tableId, record, key1, mut, new HashSet<>());
    writer.put(md1).getValue().get();

    var key2 = ByteString.copyFrom("key2".getBytes(StandardCharsets.UTF_8));
    var md2 = new MutationData(tableId, record, key2, mut, new HashSet<>());
    writer.put(md2).getValue().get();

    var duplicateKeyMutation = new MutationData(tableId, record, key1, mut, new HashSet<>());
    var fut = writer.put(duplicateKeyMutation).getValue();

    ExecutionException exception = assertThrows(ExecutionException.class, fut::get);
    var cause = exception.getCause();
    assertEquals(ConnectException.class, cause.getClass());
    assertEquals("Insert failed since the row already existed.", cause.getMessage());
  }
}
