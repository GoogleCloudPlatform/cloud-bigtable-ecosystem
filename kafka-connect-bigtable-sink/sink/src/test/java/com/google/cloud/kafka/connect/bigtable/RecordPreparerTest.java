package com.google.cloud.kafka.connect.bigtable;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertThrows;
import static org.junit.Assert.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.doReturn;

import com.google.cloud.kafka.connect.bigtable.config.NullValueMode;
import com.google.cloud.kafka.connect.bigtable.mapping.KeyMapper;
import com.google.cloud.kafka.connect.bigtable.mapping.ValueMapper;
import java.util.Arrays;
import java.util.List;
import org.apache.kafka.connect.errors.ConnectException;
import org.apache.kafka.connect.errors.DataException;
import org.apache.kafka.connect.sink.SinkRecord;
import org.junit.Test;

public class RecordPreparerTest {

  @Test
  public void testGetTableName() {
    String tableFormat = "table";
    SinkRecord record = new SinkRecord("topic", 1, null, null, null, null, 1);

    var preparer = new RecordPreparer(tableFormat, new KeyMapper("#", Arrays.asList("a", "b", "c")),
        new ValueMapper("cf", "c", NullValueMode.IGNORE));
    preparer.getTableName(record);
    assertEquals(tableFormat, preparer.getTableName(record).toString());
  }

  @Test
  public void testEmptyValue() {
    String tableFormat = "table";
    SinkRecord record = new SinkRecord("topic", 1, null, new Object(), null, null, 1);

    var preparer = new RecordPreparer(tableFormat, new KeyMapper("#", Arrays.asList("a", "b", "c")),
        new ValueMapper("cf", "c", NullValueMode.IGNORE));
    var resultMaybe = preparer.createRecordMutationTry(record);
    assertFalse(resultMaybe.isEmpty());
    var result = resultMaybe.get();
    assertFalse(result.isSuccess());
    assertEquals(DataException.class, result.getThrowable().getClass());
    assertEquals("The record's key converts into an illegal empty Cloud Bigtable row key.",
        result.getThrowable().getMessage());
  }

  // @Test
  // public void testCreateRecordMutationDataNonemptyKey() {
  //   SinkRecord emptyRecord = new SinkRecord("topic", 1, null, "key", null, null, 1);
  //   SinkRecord okRecord = new SinkRecord("topic", 1, null, "key", null, "value", 2);
  //   keyMapper = new KeyMapper("#", List.of());
  //   valueMapper = new ValueMapper("default", "KAFKA_VALUE", NullValueMode.IGNORE);
  //   task = new TestBigtableSinkTask(config, null, null, keyMapper, valueMapper, null, null);
  //
  //   assertTrue(task.createRecordMutationData(emptyRecord).isEmpty());
  //   assertTrue(task.createRecordMutationData(okRecord).isPresent());
  // }
}
