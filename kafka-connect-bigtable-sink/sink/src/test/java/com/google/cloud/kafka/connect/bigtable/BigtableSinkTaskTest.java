/*
 * Copyright 2024 Google LLC
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
package com.google.cloud.kafka.connect.bigtable;

import static com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig.AUTO_CREATE_COLUMN_FAMILIES_CONFIG;
import static com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig.AUTO_CREATE_TABLES_CONFIG;
import static com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig.ERROR_MODE_CONFIG;
import static com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig.INSERT_MODE_CONFIG;
import static com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig.TABLE_NAME_FORMAT_CONFIG;
import static com.google.cloud.kafka.connect.bigtable.util.FutureUtil.completedApiFuture;
import static com.google.cloud.kafka.connect.bigtable.util.MockUtil.assertTotalNumberOfInvocations;
import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertNotNull;
import static org.junit.Assert.assertThrows;
import static org.junit.Assert.assertTrue;
import static org.mockito.Mockito.any;
import static org.mockito.Mockito.anyLong;
import static org.mockito.Mockito.anyString;
import static org.mockito.Mockito.argThat;
import static org.mockito.Mockito.doAnswer;
import static org.mockito.Mockito.doReturn;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.reset;
import static org.mockito.Mockito.spy;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoMoreInteractions;
import static org.mockito.MockitoAnnotations.openMocks;

import com.google.api.gax.batching.Batcher;
import com.google.api.gax.rpc.ApiException;
import com.google.bigtable.admin.v2.Table;
import com.google.cloud.bigtable.data.v2.BigtableDataClient;
import com.google.cloud.bigtable.data.v2.models.Mutation;
import com.google.cloud.bigtable.data.v2.models.RowMutationEntry;
import com.google.cloud.kafka.connect.bigtable.autocreate.BigtableSchemaManager;
import com.google.cloud.kafka.connect.bigtable.autocreate.ResourceCreationResult;
import com.google.cloud.kafka.connect.bigtable.config.BigtableErrorMode;
import com.google.cloud.kafka.connect.bigtable.config.BigtableSinkTaskConfig;
import com.google.cloud.kafka.connect.bigtable.config.InsertMode;
import com.google.cloud.kafka.connect.bigtable.config.NullValueMode;
import com.google.cloud.kafka.connect.bigtable.exception.BigtableSinkLogicError;
import com.google.cloud.kafka.connect.bigtable.exception.InvalidBigtableSchemaModificationException;
import com.google.cloud.kafka.connect.bigtable.mapping.KeyMapper;
import com.google.cloud.kafka.connect.bigtable.mapping.MutationData;
import com.google.cloud.kafka.connect.bigtable.mapping.MutationDataBuilder;
import com.google.cloud.kafka.connect.bigtable.mapping.ValueMapper;
import com.google.cloud.kafka.connect.bigtable.util.ApiExceptionFactory;
import com.google.cloud.kafka.connect.bigtable.util.BasicPropertiesFactory;
import com.google.cloud.kafka.connect.bigtable.util.FutureUtil;
import com.google.cloud.kafka.connect.bigtable.utils.Utils;
import com.google.cloud.kafka.connect.bigtable.wrappers.BigtableTableAdminClientInterface;
import com.google.common.collect.Collections2;
import com.google.protobuf.ByteString;
import java.nio.charset.StandardCharsets;
import java.util.AbstractMap;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Future;
import java.util.stream.Collectors;
import java.util.stream.IntStream;
import org.apache.kafka.common.record.TimestampType;
import org.apache.kafka.connect.errors.ConnectException;
import org.apache.kafka.connect.sink.ErrantRecordReporter;
import org.apache.kafka.connect.sink.SinkRecord;
import org.apache.kafka.connect.sink.SinkTaskContext;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;
import org.mockito.Mock;
import org.slf4j.Logger;

@RunWith(JUnit4.class)
public class BigtableSinkTaskTest {

  TestBigtableSinkTask task;
  BigtableSinkTaskConfig config;
  @Mock
  KeyMapper keyMapper;
  @Mock
  ValueMapper valueMapper;
  @Mock
  BigtableSchemaManager schemaManager;
  @Mock
  SinkTaskContext context;
  @Mock
  ErrantRecordReporter errorReporter;

  @Before
  public void setUp() {
    openMocks(this);
    config = new BigtableSinkTaskConfig(BasicPropertiesFactory.getTaskProps());
  }

  @Test
  public void testVersion() {
    task = spy(new TestBigtableSinkTask(null, null, null, null, null, null, null));
    assertNotNull(task.version());
  }

  @Test
  public void testGetTableName() {
    String tableFormat = "table";
    SinkRecord record = new SinkRecord("topic", 1, null, null, null, null, 1);
    Map<String, String> props = BasicPropertiesFactory.getTaskProps();
    props.put(TABLE_NAME_FORMAT_CONFIG, tableFormat);
    task =
        new TestBigtableSinkTask(
            new BigtableSinkTaskConfig(props), null, null, null, null, null, null);
    assertEquals(tableFormat, task.getTableName(record));
  }

  @Test
  public void testCreateRecordMutationDataEmptyKey() {
    task = new TestBigtableSinkTask(config, null, null, keyMapper, null, null, null);
    doReturn(new byte[0]).when(keyMapper).getKey(any());
    SinkRecord record = new SinkRecord("topic", 1, null, new Object(), null, null, 1);
    assertThrows(ConnectException.class, () -> task.createRecordMutationData(record));
  }

  @Test
  public void testCreateRecordMutationDataNonemptyKey() {
    SinkRecord emptyRecord = new SinkRecord("topic", 1, null, "key", null, null, 1);
    SinkRecord okRecord = new SinkRecord("topic", 1, null, "key", null, "value", 2);
    keyMapper = new KeyMapper("#", List.of());
    valueMapper = new ValueMapper("default", "KAFKA_VALUE", NullValueMode.IGNORE);
    task = new TestBigtableSinkTask(config, null, null, keyMapper, valueMapper, null, null);

    assertTrue(task.createRecordMutationData(emptyRecord).isEmpty());
    assertTrue(task.createRecordMutationData(okRecord).isPresent());
  }

  @Test
  public void testErrorReporterWithDLQ() {
    doReturn(errorReporter).when(context).errantRecordReporter();
    task = new TestBigtableSinkTask(null, null, null, null, null, null, context);
    SinkRecord record = new SinkRecord(null, 1, null, null, null, null, 1);
    Throwable t = new Exception("testErrorReporterWithDLQ");
    verifyNoMoreInteractions(task.getLogger());
    task.reportError(record, t);
    verify(errorReporter, times(1)).report(record, t);
  }

  @Test
  public void testErrorReporterNoDLQIgnoreMode() {
    Map<String, String> props = BasicPropertiesFactory.getTaskProps();
    props.put(ERROR_MODE_CONFIG, BigtableErrorMode.IGNORE.name());
    BigtableSinkTaskConfig config = new BigtableSinkTaskConfig(props);

    doThrow(new NoSuchMethodError()).when(context).errantRecordReporter();
    task = new TestBigtableSinkTask(config, null, null, null, null, null, context);
    SinkRecord record = new SinkRecord(null, 1, null, null, null, null, 1);
    verifyNoMoreInteractions(task.getLogger());
    verifyNoMoreInteractions(errorReporter);
    task.reportError(record, new Exception("testErrorReporterWithDLQ"));
  }

  @Test
  public void testErrorReporterNoDLQWarnMode() {
    Map<String, String> props = BasicPropertiesFactory.getTaskProps();
    props.put(ERROR_MODE_CONFIG, BigtableErrorMode.WARN.name());
    BigtableSinkTaskConfig config = new BigtableSinkTaskConfig(props);

    doReturn(null).when(context).errantRecordReporter();
    task = new TestBigtableSinkTask(config, null, null, null, null, null, context);
    SinkRecord record = new SinkRecord(null, 1, null, "key", null, null, 1);
    Throwable t = new Exception("testErrorReporterNoDLQWarnMode");
    verifyNoMoreInteractions(errorReporter);
    task.reportError(record, t);
    verify(task.getLogger(), times(1)).warn(anyString(), eq(record.key()), eq(t));
  }

  @Test
  public void testErrorReporterNoDLQFailMode() {
    Map<String, String> props = BasicPropertiesFactory.getTaskProps();
    props.put(ERROR_MODE_CONFIG, BigtableErrorMode.FAIL.name());
    BigtableSinkTaskConfig config = new BigtableSinkTaskConfig(props);

    doReturn(null).when(context).errantRecordReporter();
    task = new TestBigtableSinkTask(config, null, null, null, null, null, context);
    SinkRecord record = new SinkRecord(null, 1, null, "key", null, null, 1);
    Throwable t = new Exception("testErrorReporterNoDLQFailMode");
    verifyNoMoreInteractions(errorReporter);
    verifyNoMoreInteractions(task.getLogger());
    assertThrows(ConnectException.class, () -> task.reportError(record, t));
  }

  @Test
  public void testGetTimestamp() {
    task = new TestBigtableSinkTask(null, null, null, null, null, null, null);
    long timestampMillis = 123L;
    SinkRecord recordWithTimestamp =
        new SinkRecord(
            null, 1, null, null, null, null, 1, timestampMillis, TimestampType.CREATE_TIME);
    SinkRecord recordWithNullTimestamp = new SinkRecord(null, 1, null, null, null, null, 2);

    assertEquals(
        (Long) (1000L * timestampMillis), (Long) Utils.getTimestampMicros(recordWithTimestamp));
    assertTrue("null timestamp should be set to current clock time",
        Math.abs(Utils.getTimestampMicros(recordWithNullTimestamp)) - System.currentTimeMillis()
            < 1000);

    // Assertion that the Java Bigtable client doesn't support microsecond timestamp granularity.
    // When it starts supporting it, getTimestamp() will need to get modified.
    assertEquals(
        Arrays.stream(Table.TimestampGranularity.values()).collect(Collectors.toSet()),
        Set.of(
            Table.TimestampGranularity.TIMESTAMP_GRANULARITY_UNSPECIFIED,
            Table.TimestampGranularity.MILLIS,
            Table.TimestampGranularity.UNRECOGNIZED));
  }

  @Test
  public void testHandleResults() {
    SinkRecord errorSinkRecord = new SinkRecord("", 1, null, null, null, null, 1);
    SinkRecord successSinkRecord = new SinkRecord("", 1, null, null, null, null, 2);
    Map<SinkRecord, Future<Void>> perRecordResults =
        Map.of(
            errorSinkRecord, CompletableFuture.failedFuture(new Exception("testHandleResults")),
            successSinkRecord, CompletableFuture.completedFuture(null));
    doReturn(errorReporter).when(context).errantRecordReporter();
    task = new TestBigtableSinkTask(null, null, null, null, null, null, context);
    task.handleResults(perRecordResults);
    verify(errorReporter, times(1)).report(eq(errorSinkRecord), any());
    assertTotalNumberOfInvocations(errorReporter, 1);
  }

  @Test
  public void testPrepareRecords() {
    task = spy(new TestBigtableSinkTask(null, null, null, null, null, null, context));
    doReturn(errorReporter).when(context).errantRecordReporter();

    MutationData okMutationData = mock(MutationData.class);
    Exception exception = new RuntimeException();
    doThrow(exception)
        .doReturn(Optional.empty())
        .doReturn(Optional.of(okMutationData))
        .when(task)
        .createRecordMutationData(any());

    SinkRecord exceptionRecord = new SinkRecord("", 1, null, null, null, null, 1);
    SinkRecord emptyRecord = new SinkRecord("", 1, null, null, null, null, 3);
    SinkRecord okRecord = new SinkRecord("", 1, null, null, null, null, 2);

    Map<SinkRecord, MutationData> result =
        task.prepareRecords(List.of(exceptionRecord, emptyRecord, okRecord));
    assertEquals(Map.of(okRecord, okMutationData), result);
    verify(errorReporter, times(1)).report(exceptionRecord, exception);
    assertTotalNumberOfInvocations(errorReporter, 1);
  }



  @Test
  public void testAutoCreateTablesAndHandleErrors() {
    task = spy(new TestBigtableSinkTask(null, null, null, null, null, schemaManager, context));
    doReturn(errorReporter).when(context).errantRecordReporter();

    doReturn(errorReporter).when(context).errantRecordReporter();
    SinkRecord okRecord = new SinkRecord("", 1, null, null, null, null, 1);
    SinkRecord bigtableErrorRecord = new SinkRecord("", 1, null, null, null, null, 2);
    SinkRecord dataErrorRecord = new SinkRecord("", 1, null, null, null, null, 3);
    MutationData okMutationData = mock(MutationData.class);
    MutationData bigtableErrorMutationData = mock(MutationData.class);
    MutationData dataErrorMutationData = mock(MutationData.class);

    Map<SinkRecord, MutationData> mutations = new HashMap<>();
    mutations.put(okRecord, okMutationData);
    mutations.put(bigtableErrorRecord, bigtableErrorMutationData);
    mutations.put(dataErrorRecord, dataErrorMutationData);

    ResourceCreationResult resourceCreationResult =
        new ResourceCreationResult(Set.of(bigtableErrorRecord), Set.of(dataErrorRecord));
    doReturn(resourceCreationResult).when(schemaManager).ensureTablesExist(any());
    Map<SinkRecord, MutationData> mutationsToApply =
        task.autoCreateTablesAndHandleErrors(mutations);

    assertEquals(Map.of(okRecord, okMutationData), mutationsToApply);
    verify(errorReporter, times(1))
        .report(eq(bigtableErrorRecord), argThat(e -> e instanceof ConnectException));
    verify(errorReporter, times(1))
        .report(
            eq(dataErrorRecord),
            argThat(e -> e instanceof InvalidBigtableSchemaModificationException));
    assertTotalNumberOfInvocations(errorReporter, 2);
  }

  @Test
  public void testAutoCreateColumnFamiliesAndHandleErrors() {
    task = spy(new TestBigtableSinkTask(null, null, null, null, null, schemaManager, context));
    doReturn(errorReporter).when(context).errantRecordReporter();

    doReturn(errorReporter).when(context).errantRecordReporter();
    SinkRecord okRecord = new SinkRecord("", 1, null, null, null, null, 1);
    SinkRecord bigtableErrorRecord = new SinkRecord("", 1, null, null, null, null, 2);
    SinkRecord dataErrorRecord = new SinkRecord("", 1, null, null, null, null, 3);
    MutationData okMutationData = mock(MutationData.class);
    MutationData bigtableErrorMutationData = mock(MutationData.class);
    MutationData dataErrorMutationData = mock(MutationData.class);

    Map<SinkRecord, MutationData> mutations = new HashMap<>();
    mutations.put(okRecord, okMutationData);
    mutations.put(bigtableErrorRecord, bigtableErrorMutationData);
    mutations.put(dataErrorRecord, dataErrorMutationData);

    ResourceCreationResult resourceCreationResult =
        new ResourceCreationResult(Set.of(bigtableErrorRecord), Set.of(dataErrorRecord));
    doReturn(resourceCreationResult).when(schemaManager).ensureColumnFamiliesExist(any());
    Map<SinkRecord, MutationData> mutationsToApply =
        task.autoCreateColumnFamiliesAndHandleErrors(mutations);

    assertEquals(Map.of(okRecord, okMutationData), mutationsToApply);
    verify(errorReporter, times(1))
        .report(eq(bigtableErrorRecord), argThat(e -> e instanceof ConnectException));
    verify(errorReporter, times(1))
        .report(
            eq(dataErrorRecord),
            argThat(e -> e instanceof InvalidBigtableSchemaModificationException));
    assertTotalNumberOfInvocations(errorReporter, 2);
  }
}
