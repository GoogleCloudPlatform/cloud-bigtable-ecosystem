package com.google.cloud.kafka.connect.bigtable.writers;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertNotNull;
import static org.junit.Assert.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.doReturn;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;

import com.google.cloud.bigtable.admin.v2.BigtableTableAdminClient;
import com.google.cloud.bigtable.admin.v2.BigtableTableAdminSettings;
import com.google.cloud.bigtable.admin.v2.models.CreateTableRequest;
import com.google.cloud.bigtable.data.v2.BigtableDataClient;
import com.google.cloud.bigtable.data.v2.BigtableDataSettings;
import com.google.cloud.bigtable.data.v2.models.Mutation;
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
  private final TableId tableId = TableId.of("table1");
  private final String cf = "cf";

  @Before
  public void setUp() throws IOException {
    // Initialize the clients to connect to the emulator
    BigtableTableAdminSettings.Builder tableAdminSettings = BigtableTableAdminSettings.newBuilderForEmulator(
            bigtableEmulator.getPort())
        .setProjectId("my-project")
        .setInstanceId("my-instance");
    var tableAdminClient = BigtableTableAdminClient.create(tableAdminSettings.build());

    BigtableDataSettings.Builder dataSettings = BigtableDataSettings.newBuilderForEmulator(
        bigtableEmulator.getPort())
        .setProjectId("my-project")
        .setInstanceId("my-instance");
    dataClient = BigtableDataClient.create(dataSettings.build());

    // Create a test table that can be used in tests
    tableAdminClient.createTable(
        CreateTableRequest.of(tableId.toString())
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
    Exception exception = null;
    try {
      writer.put(duplicateKeyMutation).getValue().get();
    } catch (Exception e) {
      exception = e;
    }
    assertNotNull(exception);
    assertEquals(ConnectException.class, exception.getClass());
    assertEquals("Insert failed since the row already existed.", exception.getMessage());
  }
}
