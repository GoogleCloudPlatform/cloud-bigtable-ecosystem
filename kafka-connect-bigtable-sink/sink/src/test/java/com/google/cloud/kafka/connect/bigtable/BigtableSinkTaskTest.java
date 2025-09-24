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

import static com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig.ERROR_MODE_CONFIG;
import static com.google.cloud.kafka.connect.bigtable.util.MockUtil.assertTotalNumberOfInvocations;
import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertNotNull;
import static org.junit.Assert.assertThrows;
import static org.junit.Assert.assertTrue;
import static org.mockito.Mockito.any;
import static org.mockito.Mockito.anyString;
import static org.mockito.Mockito.argThat;
import static org.mockito.Mockito.doReturn;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.spy;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoMoreInteractions;
import static org.mockito.MockitoAnnotations.openMocks;

import com.google.bigtable.admin.v2.Table;
import com.google.cloud.kafka.connect.bigtable.autocreate.BigtableSchemaManager;
import com.google.cloud.kafka.connect.bigtable.autocreate.ResourceCreationResult;
import com.google.cloud.kafka.connect.bigtable.config.BigtableErrorMode;
import com.google.cloud.kafka.connect.bigtable.config.BigtableSinkTaskConfig;
import com.google.cloud.kafka.connect.bigtable.config.NullValueMode;
import com.google.cloud.kafka.connect.bigtable.exception.InvalidBigtableSchemaModificationException;
import com.google.cloud.kafka.connect.bigtable.mapping.KeyMapper;
import com.google.cloud.kafka.connect.bigtable.mapping.MutationData;
import com.google.cloud.kafka.connect.bigtable.mapping.ValueMapper;
import com.google.cloud.kafka.connect.bigtable.util.BasicPropertiesFactory;
import com.google.cloud.kafka.connect.bigtable.utils.Utils;
import java.util.Arrays;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.Future;
import java.util.stream.Collectors;
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

@RunWith(JUnit4.class)
public class BigtableSinkTaskTest {

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




}
