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

import com.google.cloud.kafka.connect.bigtable.autocreate.BigtableSchemaManager;
import com.google.cloud.kafka.connect.bigtable.config.BigtableSinkTaskConfig;
import com.google.cloud.kafka.connect.bigtable.mapping.KeyMapper;
import com.google.cloud.kafka.connect.bigtable.mapping.ValueMapper;
import com.google.cloud.kafka.connect.bigtable.version.PackageMetadata;
import com.google.cloud.kafka.connect.bigtable.writers.BigtableWriter;
import com.google.cloud.kafka.connect.bigtable.writers.BigtableWriterFactory;
import com.google.common.annotations.VisibleForTesting;
import java.util.Collection;
import java.util.Map;
import java.util.Optional;
import java.util.stream.Collectors;
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

  private RecordPreparer recordPreparer;
  private BigtableWriter writer;
  private RecordErrorHandler errorHandler;
  private BigtableSchemaManager autoResourceCreator;
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
    this.context = context;
  }

  @Override
  public void start(Map<String, String> props) {
    config = new BigtableSinkTaskConfig(props);
    logger =
        LoggerFactory.getLogger(
            BigtableSinkTask.class.getName()
                + config.getInt(BigtableSinkTaskConfig.TASK_ID_CONFIG));

    errorHandler = new RecordErrorHandler(logger, context, config.getBigtableErrorMode());

    writer = BigtableWriterFactory.GetWriter(config, config.getBigtableDataClient());

    autoResourceCreator = new BigtableSchemaManager(config.getBigtableAdminClient());

    var keyMapper = config.createKeymapper();
    var valueMapper = config.createValueMapper();
    recordPreparer = new RecordPreparer(
        config.getString(BigtableSinkTaskConfig.TABLE_NAME_FORMAT_CONFIG), keyMapper, valueMapper);
  }

  @Override
  public void stop() {
    logger.trace("stop()");
    try {
      writer.close();
    } catch (Exception e) {
      throw new ConnectException("Failed to close bigtable writer", e);
    }
    try {
      autoResourceCreator.close();
    } catch (Exception e) {
      throw new ConnectException("Failed to close resource creator", e);
    }
  }

  @Override
  public void flush(Map<TopicPartition, OffsetAndMetadata> currentOffsets) {
    try {
      writer.flush();
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

    var mutations = records.stream()
        // convert SinkRecords to Mutations
        .map(r -> recordPreparer.createRecordMutationTry(r))
        .flatMap(Optional::stream)
        // report any errors
        .map(t -> errorHandler.processTry(t))
        .flatMap(Optional::stream)
        .collect(Collectors.toList());

    if (config.getBoolean(BigtableSinkTaskConfig.AUTO_CREATE_TABLES_CONFIG)) {
      errorHandler.reportErrors(autoResourceCreator.ensureTablesExist(mutations));
    }

    if (config.getBoolean(BigtableSinkTaskConfig.AUTO_CREATE_COLUMN_FAMILIES_CONFIG)) {
      errorHandler.reportErrors(autoResourceCreator.ensureColumnFamiliesExist(mutations));
    }

    mutations.stream()
        // write the data
        .map(m -> writer.put(m))
        // todo getValue isn't safe
        .forEach(m -> errorHandler.handleDelayed(m.getRecord(), m.getValue()));
  }
}
