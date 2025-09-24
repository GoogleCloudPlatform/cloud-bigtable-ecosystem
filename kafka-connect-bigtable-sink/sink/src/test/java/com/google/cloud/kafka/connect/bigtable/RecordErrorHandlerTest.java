package com.google.cloud.kafka.connect.bigtable;

import static com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig.ERROR_MODE_CONFIG;
import static org.junit.Assert.assertThrows;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doReturn;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoMoreInteractions;

import com.google.cloud.kafka.connect.bigtable.config.BigtableErrorMode;
import com.google.cloud.kafka.connect.bigtable.config.BigtableSinkTaskConfig;
import com.google.cloud.kafka.connect.bigtable.util.BasicPropertiesFactory;
import java.util.Map;
import org.apache.kafka.connect.errors.ConnectException;
import org.apache.kafka.connect.sink.SinkRecord;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public class RecordErrorHandlerTest {

  // @Test
  // public void testErrorReporterWithDLQ() {
  //   doReturn(errorReporter).when(context).errantRecordReporter();
  //   task = new TestBigtableSinkTask(null, null, null, null, null, null, context);
  //   SinkRecord record = new SinkRecord(null, 1, null, null, null, null, 1);
  //   Throwable t = new Exception("testErrorReporterWithDLQ");
  //   verifyNoMoreInteractions(task.getLogger());
  //   task.reportError(record, t);
  //   verify(errorReporter, times(1)).report(record, t);
  // }


  // @Test
  // public void testErrorReporterNoDLQFailMode() {
  //   Map<String, String> props = BasicPropertiesFactory.getTaskProps();
  //   props.put(ERROR_MODE_CONFIG, BigtableErrorMode.FAIL.name());
  //   BigtableSinkTaskConfig config = new BigtableSinkTaskConfig(props);
  //
  //   doReturn(null).when(context).errantRecordReporter();
  //   task = new TestBigtableSinkTask(config, null, null, null, null, null, context);
  //   SinkRecord record = new SinkRecord(null, 1, null, "key", null, null, 1);
  //   Throwable t = new Exception("testErrorReporterNoDLQFailMode");
  //   verifyNoMoreInteractions(errorReporter);
  //   verifyNoMoreInteractions(task.getLogger());
  //   assertThrows(ConnectException.class, () -> task.reportError(record, t));
  // }


  // @Test
  // public void testErrorReporterNoDLQIgnoreMode() {
  //   Map<String, String> props = BasicPropertiesFactory.getTaskProps();
  //   props.put(ERROR_MODE_CONFIG, BigtableErrorMode.IGNORE.name());
  //   BigtableSinkTaskConfig config = new BigtableSinkTaskConfig(props);
  //
  //   doThrow(new NoSuchMethodError()).when(context).errantRecordReporter();
  //   task = new TestBigtableSinkTask(config, null, null, null, null, null, context);
  //   SinkRecord record = new SinkRecord(null, 1, null, null, null, null, 1);
  //   verifyNoMoreInteractions(task.getLogger());
  //   verifyNoMoreInteractions(errorReporter);
  //   task.reportError(record, new Exception("testErrorReporterWithDLQ"));
  // }
  //
  // @Test
  // public void testErrorReporterNoDLQWarnMode() {
  //   Map<String, String> props = BasicPropertiesFactory.getTaskProps();
  //   props.put(ERROR_MODE_CONFIG, BigtableErrorMode.WARN.name());
  //   BigtableSinkTaskConfig config = new BigtableSinkTaskConfig(props);
  //
  //   doReturn(null).when(context).errantRecordReporter();
  //   task = new TestBigtableSinkTask(config, null, null, null, null, null, context);
  //   SinkRecord record = new SinkRecord(null, 1, null, "key", null, null, 1);
  //   Throwable t = new Exception("testErrorReporterNoDLQWarnMode");
  //   verifyNoMoreInteractions(errorReporter);
  //   task.reportError(record, t);
  //   verify(task.getLogger(), times(1)).warn(anyString(), eq(record.key()), eq(t));
  // }


}
