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

import com.google.cloud.kafka.connect.bigtable.autocreate.AutoResourceCreator;
import com.google.cloud.kafka.connect.bigtable.autocreate.AutoResourceCreatorImpl;
import com.google.cloud.kafka.connect.bigtable.autocreate.BigtableSchemaManager;
import com.google.cloud.kafka.connect.bigtable.config.BigtableSinkTaskConfig;
import com.google.cloud.kafka.connect.bigtable.mapping.KeyMapper;
import com.google.cloud.kafka.connect.bigtable.mapping.MutationData;
import com.google.cloud.kafka.connect.bigtable.mapping.ValueMapper;
import com.google.cloud.kafka.connect.bigtable.version.PackageMetadata;
import com.google.cloud.kafka.connect.bigtable.writers.BigtableWriter;
import com.google.cloud.kafka.connect.bigtable.writers.BigtableWriterFactory;
import com.google.common.annotations.VisibleForTesting;
import java.util.Collection;
import java.util.LinkedList;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.ExecutionException;
import org.apache.kafka.clients.consumer.OffsetAndMetadata;
import org.apache.kafka.common.TopicPartition;
import org.apache.kafka.connect.errors.ConnectException;
import org.apache.kafka.connect.sink.SinkRecord;
import org.apache.kafka.connect.sink.SinkTask;
import org.apache.kafka.connect.sink.SinkTaskContext;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * {@link SinkTask} class used by {@link org.apache.kafka.connect.sink.SinkConnector} to write to
 * Cloud Bigtable.
 */
public class BigtableSinkTask extends SinkTask {

  private BigtableSinkTaskConfig config;
  private KeyMapper keyMapper;
  private ValueMapper valueMapper;
  private RecordPreparer recordPreparer;
  private BigtableWriter writer;
  private RecordErrorHandler errorHandler;
  private AutoResourceCreator autoResourceCreator;
  @VisibleForTesting
  protected Logger logger = LoggerFactory.getLogger(BigtableSinkTask.class);

  /**
   * A default empty constructor. Initialization methods such as {@link BigtableSinkTask#start(Map)}
   * or {@link SinkTask#initialize(SinkTaskContext)} must be called before
   * {@link BigtableSinkTask#put(Collection)} can be called. Kafka Connect handles it well.
   */
  public BigtableSinkTask() {
    this(null, null, null, null);
  }

  // A constructor only used by the tests.
  @VisibleForTesting
  protected BigtableSinkTask(
      BigtableSinkTaskConfig config,
      KeyMapper keyMapper,
      ValueMapper valueMapper,
      SinkTaskContext context) {
    this.config = config;
    this.keyMapper = keyMapper;
    this.valueMapper = valueMapper;
    this.context = context;
  }

  @Override
  public void start(Map<String, String> props) {
    config = new BigtableSinkTaskConfig(props);
    logger =
        LoggerFactory.getLogger(
            BigtableSinkTask.class.getName()
                + config.getInt(BigtableSinkTaskConfig.TASK_ID_CONFIG));

    errorHandler = new RecordErrorHandler(context, config.getBigtableErrorMode());

    var bigtableData = config.getBigtableDataClient();
    writer = BigtableWriterFactory.GetWriter(config, bigtableData);
    var bigtableAdmin = config.getBigtableAdminClient();

    autoResourceCreator = new AutoResourceCreatorImpl(new BigtableSchemaManager(bigtableAdmin),
        errorHandler, bigtableAdmin,
        config.getBoolean(BigtableSinkTaskConfig.AUTO_CREATE_TABLES_CONFIG),
        config.getBoolean(BigtableSinkTaskConfig.AUTO_CREATE_COLUMN_FAMILIES_CONFIG));

    keyMapper =
        new KeyMapper(
            config.getString(BigtableSinkTaskConfig.ROW_KEY_DELIMITER_CONFIG),
            config.getList(BigtableSinkTaskConfig.ROW_KEY_DEFINITION_CONFIG));
    valueMapper =
        new ValueMapper(
            config.getString(BigtableSinkTaskConfig.DEFAULT_COLUMN_FAMILY_CONFIG),
            config.getString(BigtableSinkTaskConfig.DEFAULT_COLUMN_QUALIFIER_CONFIG),
            config.getNullValueMode());
  }

  @Override
  public void stop() {
    logger.trace("stop()");
    try {
      writer.Close();
    } catch (ExecutionException | InterruptedException e) {
      throw new ConnectException("Failed to close bigtable writer", e);
    }
    autoResourceCreator.Close();
  }

  @Override
  public void flush(Map<TopicPartition, OffsetAndMetadata> currentOffsets) {
    try {
      writer.Flush();
    } catch (InterruptedException e) {
      throw new RuntimeException(e);
    }
  }

  @Override
  public String version() {
    logger.trace("version()");
    return PackageMetadata.getVersion();
  }

  @Override
  public void put(Collection<SinkRecord> records) {
    logger.trace("put(#records={})", records.size());
    if (records.isEmpty()) {
      return;
    }

    // 1. prepare the records
    Collection<MutationData> mutations = new LinkedList<>();
    for (SinkRecord record : records) {
      try {
        Optional<MutationData> maybeRecordMutationData = recordPreparer.createRecordMutationData(record);
        if (maybeRecordMutationData.isPresent()) {
          mutations.add(maybeRecordMutationData.get());
        } else {
          logger.debug("Skipped a record that maps to an empty value.");
        }
      } catch (Throwable t) {
        errorHandler.reportError(record, t);
      }
    }

    // 2. create any required tables or column families
    mutations = autoResourceCreator.CreateResources(mutations);

    // 3. write the data
    for (MutationData mut : mutations) {
      var result = writer.Put(mut);
      errorHandler.handleDelayed(mut.getRecord(), result);
    }
  }
}
