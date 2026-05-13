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

import static org.apache.kafka.connect.runtime.ConnectorConfig.CONNECTOR_CLASS_CONFIG;
import static org.apache.kafka.connect.runtime.ConnectorConfig.TASKS_MAX_CONFIG;
import static org.apache.kafka.connect.runtime.WorkerConfig.KEY_CONVERTER_CLASS_CONFIG;
import static org.apache.kafka.connect.runtime.WorkerConfig.VALUE_CONVERTER_CLASS_CONFIG;
import static org.apache.kafka.test.TestUtils.RANDOM;
import static org.apache.kafka.test.TestUtils.waitForCondition;
import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.google.api.gax.rpc.FailedPreconditionException;
import com.google.api.gax.rpc.NotFoundException;
import com.google.cloud.aerospike.config.Config;
import com.google.cloud.aerospike.exception.InvalidMessageException;
import com.google.cloud.aerospike.util.AerospikeConnectXdrJson;
import com.google.cloud.aerospike.util.TestId;
import com.google.cloud.bigtable.admin.v2.BigtableTableAdminClient;
import com.google.cloud.bigtable.admin.v2.models.CreateTableRequest;
import com.google.cloud.bigtable.admin.v2.models.Table;
import com.google.cloud.bigtable.data.v2.BigtableDataClient;
import com.google.cloud.bigtable.data.v2.models.Query;
import com.google.cloud.bigtable.data.v2.models.Row;
import com.google.cloud.bigtable.data.v2.models.RowCell;
import com.google.cloud.bigtable.data.v2.models.RowMutation;
import com.google.protobuf.ByteString;
import java.io.File;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.Arrays;
import java.util.Base64;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import java.util.Properties;
import java.util.Set;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.function.Supplier;
import java.util.stream.Collectors;
import org.apache.commons.lang.StringUtils;
import org.apache.hadoop.hbase.util.Bytes;
import org.apache.kafka.clients.admin.Admin;
import org.apache.kafka.clients.consumer.ConsumerConfig;
import org.apache.kafka.clients.consumer.ConsumerRecord;
import org.apache.kafka.clients.consumer.ConsumerRecords;
import org.apache.kafka.clients.producer.ProducerConfig;
import org.apache.kafka.common.utils.Utils;
import org.apache.kafka.connect.converters.ByteArrayConverter;
import org.apache.kafka.connect.errors.DataException;
import org.apache.kafka.connect.json.JsonConverter;
import org.apache.kafka.connect.json.JsonConverterConfig;
import org.apache.kafka.connect.runtime.ConnectorConfig;
import org.apache.kafka.connect.runtime.SinkConnectorConfig;
import org.apache.kafka.connect.runtime.WorkerConfig;
import org.apache.kafka.connect.runtime.errors.DeadLetterQueueReporter;
import org.apache.kafka.connect.runtime.errors.ToleranceType;
import org.apache.kafka.connect.runtime.isolation.PluginDiscoveryMode;
import org.apache.kafka.connect.util.clusters.EmbeddedConnectCluster;
import org.apache.kafka.test.TestCondition;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class MapAerospikeConnectJsonToBigtableSinkInputIT {
  public static final String CONNECTOR_CLASS_NAME =
      "com.google.cloud.kafka.connect.bigtable.BigtableSinkConnector";

  // https://cloud.google.com/bigtable/docs/reference/admin/rpc/google.bigtable.admin.v2#createtablerequest
  public static final int MAX_BIGTABLE_TABLE_NAME_LENGTH = 50;
  public static final String PLUGIN_PATH_ENV_VAR_NAME = "INTEGRATION_TEST_PLUGINS_PATH";

  public static final long BIGTABLE_RETRY_TIMEOUT_MILLIS = 90000L;

  public static final long DEFAULT_MINIMAL_LUT = 0L;

  public static final String BIGTABLE_PROJECT_ID = "emulator";
  public static final String BIGTABLE_INSTANCE_ID = "emulator";

  public static final String TRANSFORMATION_SMT_CONFIG_NAME = "transformation";
  public static final String TRANSFORMATION_SMT_CLASS_NAME =
      MapAerospikeConnectJsonToBigtableSinkInput.class.getSimpleName();

  private static final String VALUE = "value1";
  private static final ByteString VALUE_BYTES =
      ByteString.copyFrom(VALUE.getBytes(StandardCharsets.UTF_8));
  private static final String FAMILY = "cf";
  private static final ByteString QUALIFIER_BYTES = ByteString.copyFromUtf8("q");

  private final Logger logger =
      LoggerFactory.getLogger(MapAerospikeConnectJsonToBigtableSinkInputIT.class);

  public int numTasks = 1;
  public int maxKafkaMessageSizeBytes = 300 * 1024 * 1024;

  public BigtableDataClient bigtableData;
  public BigtableTableAdminClient bigtableAdmin;

  public EmbeddedConnectCluster connect;
  public Admin kafkaAdminClient;
  public int numWorkers = 1;
  public int numBrokers = 1;

  @BeforeEach
  public void setUpConnectAndBigtable() throws IOException {
    startConnect();
    bigtableData = getBigtableDataClient();
    bigtableAdmin = getBigtableAdminClient();
  }

  @AfterEach
  public void tearDownConnectAndBigtable() {
    if (bigtableData != null) {
      bigtableData.close();
    }
    if (bigtableAdmin != null) {
      bigtableAdmin.close();
    }
    if (connect != null) {
      stopConnect();
    }
  }

  public Map<String, String> baseConnectorProps() {
    Map<String, String> result = new HashMap<>();

    result.put(CONNECTOR_CLASS_CONFIG, CONNECTOR_CLASS_NAME);
    result.put(TASKS_MAX_CONFIG, Integer.toString(numTasks));
    result.put(KEY_CONVERTER_CLASS_CONFIG, ByteArrayConverter.class.getName());
    result.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());
    result.put(
        ConnectorConfig.VALUE_CONVERTER_CLASS_CONFIG
            + "."
            + JsonConverterConfig.SCHEMAS_ENABLE_CONFIG,
        String.valueOf(false));
    // Needed so that all messages we send to the input topics can be also sent to the DLQ
    // by DeadLetterQueueReporter.
    result.put(
        ConnectorConfig.CONNECTOR_CLIENT_PRODUCER_OVERRIDES_PREFIX
            + ProducerConfig.MAX_REQUEST_SIZE_CONFIG,
        String.valueOf(maxKafkaMessageSizeBytes));
    result.put(
        ConnectorConfig.CONNECTOR_CLIENT_PRODUCER_OVERRIDES_PREFIX
            + ProducerConfig.BUFFER_MEMORY_CONFIG,
        String.valueOf(maxKafkaMessageSizeBytes));
    result.put(ConnectorConfig.TRANSFORMS_CONFIG, TRANSFORMATION_SMT_CONFIG_NAME);
    result.put(
        ConnectorConfig.TRANSFORMS_CONFIG
            + "."
            + TRANSFORMATION_SMT_CONFIG_NAME
            + "."
            + Config.START_LUT_MILLIS,
        String.valueOf(DEFAULT_MINIMAL_LUT));
    result.put(
        ConnectorConfig.TRANSFORMS_CONFIG + "." + TRANSFORMATION_SMT_CONFIG_NAME + "." + "type",
        TRANSFORMATION_SMT_CLASS_NAME);

    result.put("gcp.bigtable.project.id", BIGTABLE_PROJECT_ID);
    result.put("gcp.bigtable.instance.id", BIGTABLE_INSTANCE_ID);
    result.put("insert.mode", "replace_if_newest");
    result.put("value.null.mode", "ignore");
    // We use this config option to enforce order of record processing.
    result.put("max.batch.size", "1");

    return result;
  }

  public BigtableDataClient getBigtableDataClient() throws IOException {
    return BigtableDataClient.create(BIGTABLE_PROJECT_ID, BIGTABLE_INSTANCE_ID);
  }

  public BigtableTableAdminClient getBigtableAdminClient() throws IOException {
    return BigtableTableAdminClient.create(BIGTABLE_PROJECT_ID, BIGTABLE_INSTANCE_ID);
  }

  public String getTestClassId() {
    return TestId.getTestClassId(this.getClass());
  }

  public String getTestCaseId() {
    return TestId.getTestCaseId(this.getClass());
  }

  public void startConnect() {
    logger.info("Starting embedded Kafka Connect cluster...");

    Properties brokerProps = new Properties();
    brokerProps.put("socket.request.max.bytes", maxKafkaMessageSizeBytes);
    brokerProps.put("message.max.bytes", maxKafkaMessageSizeBytes);
    brokerProps.put("auto.create.topics.enable", "false");
    brokerProps.put("delete.topic.enable", "true");

    Map<String, String> clientConfigs = new HashMap<>();
    clientConfigs.put(
        ProducerConfig.MAX_REQUEST_SIZE_CONFIG, String.valueOf(maxKafkaMessageSizeBytes));
    clientConfigs.put(
        ProducerConfig.BUFFER_MEMORY_CONFIG, String.valueOf(maxKafkaMessageSizeBytes));
    clientConfigs.put(
        ConsumerConfig.MAX_PARTITION_FETCH_BYTES_CONFIG, String.valueOf(maxKafkaMessageSizeBytes));

    Map<String, String> workerProps = new HashMap<>();
    String pluginPath = Objects.requireNonNull(System.getenv(PLUGIN_PATH_ENV_VAR_NAME));
    assertTrue(new File(pluginPath).isDirectory());
    workerProps.put(WorkerConfig.PLUGIN_PATH_CONFIG, pluginPath);
    workerProps.put(WorkerConfig.OFFSET_COMMIT_INTERVAL_MS_CONFIG, Long.toString(10000));
    workerProps.put(WorkerConfig.PLUGIN_DISCOVERY_CONFIG, PluginDiscoveryMode.HYBRID_WARN.name());

    connect =
        new EmbeddedConnectCluster.Builder()
            .name("kcbt-connect-cluster-" + getTestClassId())
            .numWorkers(numWorkers)
            .numBrokers(numBrokers)
            .brokerProps(brokerProps)
            .workerProps(workerProps)
            .clientProps(clientConfigs)
            .build();

    // Start the clusters
    connect.start();
    try {
      connect
          .assertions()
          .assertExactlyNumWorkersAreUp(1, "Initial group of workers did not start in time.");
    } catch (InterruptedException e) {
      throw new RuntimeException(e);
    }
    kafkaAdminClient = connect.kafka().createAdminClient();
    logger.info(
        "Started embedded Kafka Connect cluster using bootstrap servers: {}",
        connect.kafka().bootstrapServers());
  }

  public void stopConnect() {
    logger.info(
        "Stopping embedded Kafka Connect cluster using bootstrap servers: {}",
        connect.kafka().bootstrapServers());
    if (kafkaAdminClient != null) {
      Utils.closeQuietly(kafkaAdminClient, "Admin client for embedded Kafka cluster");
      kafkaAdminClient = null;
    }

    // Stop all Connect, Kafka and Zk threads.
    if (connect != null) {
      Utils.closeQuietly(connect::stop, "Embedded Connect, Kafka, and Zookeeper clusters");
      connect = null;
    }
  }

  public String startSingleTopicConnector(Map<String, String> configProps)
      throws InterruptedException {
    return startConnector(configProps, Collections.emptySet());
  }

  private String startConnector(Map<String, String> configProps, Set<String> topicNameSuffixes)
      throws InterruptedException {
    int longestSuffix = topicNameSuffixes.stream().mapToInt(String::length).max().orElse(0);
    String id =
        StringUtils.right(
            getTestCaseId() + System.currentTimeMillis(),
            MAX_BIGTABLE_TABLE_NAME_LENGTH - longestSuffix);
    if (topicNameSuffixes.isEmpty()) {
      configProps.put(SinkConnectorConfig.TOPICS_CONFIG, id);
      connect.kafka().createTopic(id, numTasks);
    } else {
      configProps.put(SinkConnectorConfig.TOPICS_REGEX_CONFIG, id + ".*");
      for (String suffix : topicNameSuffixes) {
        connect.kafka().createTopic(id + suffix, numTasks);
      }
    }
    connect.configureConnector(id, configProps);
    connect
        .assertions()
        .assertConnectorAndExactlyNumTasksAreRunning(id, numTasks, "Connector start timeout");
    return id;
  }

  public String createDlq() {
    String dlqTopic = getTestCaseId() + System.currentTimeMillis();
    connect.kafka().createTopic(dlqTopic, numBrokers);
    return dlqTopic;
  }

  public void configureDlq(Map<String, String> props, String dlqTopic) {
    props.put(SinkConnectorConfig.DLQ_TOPIC_NAME_CONFIG, dlqTopic);
    props.put(SinkConnectorConfig.DLQ_CONTEXT_HEADERS_ENABLE_CONFIG, String.valueOf(true));
    props.put(SinkConnectorConfig.DLQ_TOPIC_REPLICATION_FACTOR_CONFIG, String.valueOf(numBrokers));
    props.put(SinkConnectorConfig.ERRORS_TOLERANCE_CONFIG, ToleranceType.ALL.value());
  }

  public void assertConnectorAndAllTasksAreRunning(String connectorId) throws InterruptedException {
    connect
        .assertions()
        .assertConnectorAndExactlyNumTasksAreRunning(
            connectorId, numTasks, "Wrong number of tasks is running.");
  }

  public void assertSingleDlqEntry(
      String dlqTopic, String key, String value, Class<?> exceptionClass) {
    assertSingleDlqEntry(
        dlqTopic,
        Optional.ofNullable(key).map(s -> s.getBytes(StandardCharsets.UTF_8)).orElse(null),
        Optional.ofNullable(value).map(s -> s.getBytes(StandardCharsets.UTF_8)).orElse(null),
        exceptionClass);
  }

  public void assertSingleDlqEntry(
      String dlqTopic, byte[] key, byte[] value, Class<?> exceptionClass) {
    ConsumerRecords<byte[], byte[]> dlqRecords =
        connect.kafka().consume(1, Duration.ofSeconds(120).toMillis(), dlqTopic);
    assertEquals(1, dlqRecords.count());
    ConsumerRecord<byte[], byte[]> record = dlqRecords.iterator().next();
    if (key != null) {
      assertArrayEquals(record.key(), key);
    }
    if (value != null) {
      assertArrayEquals(record.value(), value);
    }
    if (exceptionClass != null) {
      assertTrue(
          Arrays.stream(record.headers().toArray())
              .anyMatch(
                  h ->
                      h.key().equals(DeadLetterQueueReporter.ERROR_HEADER_EXCEPTION)
                          && Arrays.equals(
                              h.value(),
                              exceptionClass.getName().getBytes(StandardCharsets.UTF_8))));
    }
  }

  public void createTablesAndColumnFamilies(Map<String, Set<String>> tablesAndColumnFamilies)
      throws ExecutionException, InterruptedException {
    List<Future<Table>> futures =
        tablesAndColumnFamilies.entrySet().parallelStream()
            .map(
                e -> {
                  CreateTableRequest ctr = CreateTableRequest.of(e.getKey());
                  e.getValue().forEach(ctr::addFamily);
                  return bigtableAdmin.createTableAsync(ctr);
                })
            .collect(Collectors.toList());
    for (Future<Table> f : futures) {
      f.get();
    }
  }

  public Map<ByteString, Row> readAllRows(BigtableDataClient bigtable, String table) {
    Integer numRecords = null;
    try {
      Query query = Query.create(table);
      Map<ByteString, Row> result =
          bigtable.readRows(query).stream().collect(Collectors.toMap(Row::getKey, r -> r));
      numRecords = result.size();
      return result;
    } finally {
      logger.info("readAllRows({}): #records={}", table, numRecords);
    }
  }

  public void setCell(
      BigtableDataClient bigtable,
      String table,
      ByteString row,
      String family,
      ByteString qualifier,
      ByteString value,
      long timestampMillis) {
    bigtable.mutateRow(
        RowMutation.create(table, row)
            .setCell(family, qualifier, TimeUnit.MILLISECONDS.toMicros(timestampMillis), value));
    logger.info(
        "setCell(table={}, row={}, family={}, qualifier={}, value={}, timestampMillis={}) success",
        table,
        row.toStringUtf8(),
        family,
        qualifier.toStringUtf8(),
        value.toStringUtf8(),
        timestampMillis);
  }

  public long cellCount(Map<ByteString, Row> rows) {
    return rows.values().stream().mapToLong(r -> r.getCells().size()).sum();
  }

  public void waitUntilBigtableContainsNumberOfRows(String tableId, long numberOfRows)
      throws InterruptedException {
    waitForCondition(
        testConditionIgnoringTransientErrors(
            () -> readAllRows(bigtableData, tableId).size() == numberOfRows),
        BIGTABLE_RETRY_TIMEOUT_MILLIS,
        "Records not consumed in time.");
  }

  // These exceptions are thrown around the moment of a table or column family creation.
  private TestCondition testConditionIgnoringTransientErrors(Supplier<Boolean> supplier) {
    return () -> {
      try {
        return supplier.get();
      } catch (NotFoundException | FailedPreconditionException e) {
        return false;
      }
    };
  }

  private static ByteString createRandomRowKey() {
    byte[] result = new byte[20];
    RANDOM.nextBytes(result);
    return ByteString.copyFrom(result);
  }

  // TODO: check NaN/+Inf/-Inf once we receive an Aerospike Enterprise license key.
  @Test
  public void WriteDifferentTypesIT() throws InterruptedException, ExecutionException {
    Map<String, String> props = baseConnectorProps();
    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(
        Map.of(testId, Set.of(RowBuilder.SCALAR_FAMILY, RowBuilder.COMPLEX_OBJECT_FAMILY)));

    // Example adapted from [0] with unsupported `geojson` type bin removed and some scalar bins
    // added.
    // [0]https://aerospike.com/docs/connectors/streaming/kafka/outbound/formats/json-serialization-format#examples
    String stringBin = "myString";
    String blobBin = "myBlob";
    String listBin = "myList";
    String mapBin = "myMap";
    String boolBin = "myBoolean";
    String intBin = "myInt";
    String floatBin = "myFloat";

    Long lastUpdateTimestamp = 1617167159548L;
    ByteString key = ByteString.copyFromUtf8("abcdefghijklmnopqrst");

    String stringValue = "a string value";
    ByteString blobValue = ByteString.copyFromUtf8("ABCDEFGHIJKLMNOPQRSTUVWXYZ");
    List<Object> listValue = List.of("abc", "def", "ghi", "jkl");
    Map<String, Object> mapValue = Map.of("i", 42L, "f", 3.1415D, "l", List.of(3L, 2L, 1L, 0L));
    Boolean boolValue = true;
    Long intValue = 7312L;
    Double floatValue = 1.23;

    List<AerospikeConnectXdrJson.Bin> bins =
        List.of(
            new AerospikeConnectXdrJson.Bin(
                stringBin,
                MapAerospikeConnectJsonToBigtableSinkInput.ValueType.STR.name(),
                stringValue),
            new AerospikeConnectXdrJson.Bin(
                blobBin,
                MapAerospikeConnectJsonToBigtableSinkInput.ValueType.BLOB.name(),
                Base64.getEncoder().encodeToString(blobValue.toByteArray())),
            new AerospikeConnectXdrJson.Bin(
                listBin,
                MapAerospikeConnectJsonToBigtableSinkInput.ValueType.LIST.name(),
                listValue),
            new AerospikeConnectXdrJson.Bin(
                mapBin, MapAerospikeConnectJsonToBigtableSinkInput.ValueType.MAP.name(), mapValue),
            new AerospikeConnectXdrJson.Bin(
                boolBin,
                MapAerospikeConnectJsonToBigtableSinkInput.ValueType.BOOL.name(),
                boolValue),
            new AerospikeConnectXdrJson.Bin(
                intBin, MapAerospikeConnectJsonToBigtableSinkInput.ValueType.INT.name(), intValue),
            new AerospikeConnectXdrJson.Bin(
                floatBin,
                MapAerospikeConnectJsonToBigtableSinkInput.ValueType.FLOAT.name(),
                floatValue));
    String value = AerospikeConnectXdrJson.writeMessage(lastUpdateTimestamp, key, bins);

    connect.kafka().produce(testId, null, value);

    waitUntilBigtableContainsNumberOfRows(testId, 1);
    assertConnectorAndAllTasksAreRunning(testId);

    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    assertEquals(Set.of(key), rows.keySet());
    AerospikeRecord aerospikeRecord = new AerospikeRecord(rows.get(key));

    assertEquals(
        Set.of(stringBin, blobBin, listBin, mapBin, boolBin, intBin, floatBin),
        aerospikeRecord.bins.keySet());
    assertEquals(stringValue, aerospikeRecord.bins.get(stringBin));
    assertEquals(blobValue, ByteString.copyFrom((byte[]) aerospikeRecord.bins.get(blobBin)));
    assertEquals(listValue, aerospikeRecord.bins.get(listBin));
    assertEquals(mapValue, aerospikeRecord.bins.get(mapBin));
    assertEquals(boolValue, aerospikeRecord.bins.get(boolBin));
    assertEquals(intValue, aerospikeRecord.bins.get(intBin));
    assertEquals(floatValue, aerospikeRecord.bins.get(floatBin));
  }

  @Test
  public void testTimestampFiltering() throws InterruptedException, ExecutionException {
    ByteString passedKey = createRandomRowKey();
    ByteString filteredKey = createRandomRowKey();
    long minimalAllowedTimestamp = System.currentTimeMillis();

    Map<String, String> props = baseConnectorProps();
    props.put(
        ConnectorConfig.TRANSFORMS_CONFIG
            + "."
            + TRANSFORMATION_SMT_CONFIG_NAME
            + "."
            + Config.START_LUT_MILLIS,
        String.valueOf(minimalAllowedTimestamp));

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(
        Map.of(testId, Set.of(RowBuilder.SCALAR_FAMILY, RowBuilder.COMPLEX_OBJECT_FAMILY)));

    List<AerospikeConnectXdrJson.Bin> bins =
        List.of(
            new AerospikeConnectXdrJson.Bin(
                "o", MapAerospikeConnectJsonToBigtableSinkInput.ValueType.BOOL, true));
    connect
        .kafka()
        .produce(
            testId,
            null,
            AerospikeConnectXdrJson.writeMessage(minimalAllowedTimestamp - 1, filteredKey, bins));
    connect
        .kafka()
        .produce(
            testId,
            null,
            AerospikeConnectXdrJson.writeMessage(minimalAllowedTimestamp, passedKey, bins));

    waitUntilBigtableContainsNumberOfRows(testId, 1);
    assertConnectorAndAllTasksAreRunning(testId);
    assertEquals(Set.of(passedKey), readAllRows(bigtableData, testId).keySet());
  }

  @Test
  public void testDeleteWorksIrregardlessOfRowExistence()
      throws InterruptedException, ExecutionException {
    ByteString preexistingKey = createRandomRowKey();
    ByteString nonexistentKey = createRandomRowKey();

    Map<String, String> props = baseConnectorProps();
    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of(FAMILY)));

    setCell(
        bigtableData,
        testId,
        preexistingKey,
        FAMILY,
        ByteString.copyFromUtf8("q"),
        VALUE_BYTES,
        0L);
    waitUntilBigtableContainsNumberOfRows(testId, 1);

    connect
        .kafka()
        .produce(testId, null, AerospikeConnectXdrJson.deleteMessage(0L, nonexistentKey));
    connect
        .kafka()
        .produce(testId, null, AerospikeConnectXdrJson.deleteMessage(0L, preexistingKey));

    waitUntilBigtableContainsNumberOfRows(testId, 0);
    assertConnectorAndAllTasksAreRunning(testId);
  }

  @Test
  public void testWriteDependingOnTimestampOfPreviousCells()
      throws InterruptedException, ExecutionException {
    ByteString preexistingKey1 = createRandomRowKey();
    ByteString preexistingKey2 = createRandomRowKey();
    ByteString nonexistentKey3 = createRandomRowKey();

    Map<String, String> props = baseConnectorProps();
    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(
        Map.of(testId, Set.of(FAMILY, RowBuilder.SCALAR_FAMILY, RowBuilder.COMPLEX_OBJECT_FAMILY)));

    long preexistingCellTimestamp = 10000L;

    setCell(
        bigtableData,
        testId,
        preexistingKey1,
        FAMILY,
        QUALIFIER_BYTES,
        VALUE_BYTES,
        preexistingCellTimestamp);
    setCell(
        bigtableData,
        testId,
        preexistingKey2,
        FAMILY,
        QUALIFIER_BYTES,
        VALUE_BYTES,
        preexistingCellTimestamp);
    // Row `nonexistentKey3` does not exist yet.
    waitUntilBigtableContainsNumberOfRows(testId, 2);

    List<AerospikeConnectXdrJson.Bin> bins =
        List.of(
            new AerospikeConnectXdrJson.Bin(
                "str", MapAerospikeConnectJsonToBigtableSinkInput.ValueType.STR, VALUE));

    String oldValueKey1 =
        AerospikeConnectXdrJson.writeMessage(preexistingCellTimestamp - 1, preexistingKey1, bins);
    String freshValueKey2 =
        AerospikeConnectXdrJson.writeMessage(preexistingCellTimestamp, preexistingKey2, bins);
    String oldValueKey3 =
        AerospikeConnectXdrJson.writeMessage(preexistingCellTimestamp - 1, nonexistentKey3, bins);

    // Don't replace a fresher existing row.
    connect.kafka().produce(testId, null, oldValueKey1);
    // Don't replace an existing row with fresher cells.
    connect.kafka().produce(testId, null, freshValueKey2);
    // Write data to a previously nonexistent row.
    connect.kafka().produce(testId, null, oldValueKey3);

    waitUntilBigtableContainsNumberOfRows(testId, 3);
    assertConnectorAndAllTasksAreRunning(testId);

    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    assertEquals(Set.of(preexistingKey1, preexistingKey2, nonexistentKey3), rows.keySet());
    // Unsuccessful replacement.

    Row row1 = rows.get(preexistingKey1);
    assertEquals(1, row1.getCells().size());
    assertEquals(FAMILY, row1.getCells().get(0).getFamily());
    // Successful replacement.
    Row row2 = rows.get(preexistingKey2);
    assertEquals(1, row2.getCells().size());
    assertEquals(RowBuilder.SCALAR_FAMILY, row2.getCells().get(0).getFamily());
    // Successful write.
    Row row3 = rows.get(nonexistentKey3);
    assertEquals(1, row3.getCells().size());
    assertEquals(RowBuilder.SCALAR_FAMILY, row3.getCells().get(0).getFamily());
  }

  @Test
  public void testDeleteDependingOnTimestampOfPreviousCells()
      throws InterruptedException, ExecutionException {
    ByteString nonexistentKey1 = createRandomRowKey();
    ByteString preexistingKey2 = createRandomRowKey();
    ByteString preexistingKey3 = createRandomRowKey();

    Map<String, String> props = baseConnectorProps();
    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of(FAMILY)));

    long preexistingCellTimestamp = 10000L;
    // Row `nonexistentKey1` does not exist.
    setCell(
        bigtableData,
        testId,
        preexistingKey2,
        FAMILY,
        QUALIFIER_BYTES,
        VALUE_BYTES,
        preexistingCellTimestamp);
    setCell(
        bigtableData,
        testId,
        preexistingKey3,
        FAMILY,
        QUALIFIER_BYTES,
        VALUE_BYTES,
        preexistingCellTimestamp);

    waitUntilBigtableContainsNumberOfRows(testId, 2);

    String oldValueNonexistentKey1 = AerospikeConnectXdrJson.deleteMessage(0L, nonexistentKey1);
    String newValueNonexistentKey1 =
        AerospikeConnectXdrJson.deleteMessage(preexistingCellTimestamp, nonexistentKey1);
    String oldValuePreexistingKey2 =
        AerospikeConnectXdrJson.deleteMessage(preexistingCellTimestamp - 1, preexistingKey2);
    String freshValuePreexistingKey3 =
        AerospikeConnectXdrJson.deleteMessage(preexistingCellTimestamp, preexistingKey3);

    connect.kafka().produce(testId, null, oldValueNonexistentKey1);
    connect.kafka().produce(testId, null, newValueNonexistentKey1);
    connect.kafka().produce(testId, null, oldValuePreexistingKey2);
    connect.kafka().produce(testId, null, freshValuePreexistingKey3);

    waitUntilBigtableContainsNumberOfRows(testId, 1);
    assertConnectorAndAllTasksAreRunning(testId);

    assertEquals(Set.of(preexistingKey2), readAllRows(bigtableData, testId).keySet());
  }

  @Test
  public void testInvalidMessageHandling() throws InterruptedException {
    String dlqTopic = createDlq();
    Map<String, String> props = baseConnectorProps();
    configureDlq(props, dlqTopic);

    String testId = startSingleTopicConnector(props);

    String messageLackingLastUpdateTime = "{}";

    connect.kafka().produce(testId, null, messageLackingLastUpdateTime);
    assertSingleDlqEntry(
        dlqTopic, null, messageLackingLastUpdateTime, InvalidMessageException.class);

    assertConnectorAndAllTasksAreRunning(testId);
  }

  @Test
  public void testNonDeserializableMessageHandling() throws InterruptedException {
    String dlqTopic = createDlq();
    Map<String, String> props = baseConnectorProps();
    configureDlq(props, dlqTopic);

    String testId = startSingleTopicConnector(props);

    String messageThatIsNotAValidJson = "{";

    connect.kafka().produce(testId, null, messageThatIsNotAValidJson);
    assertSingleDlqEntry(dlqTopic, null, messageThatIsNotAValidJson, DataException.class);

    assertConnectorAndAllTasksAreRunning(testId);
  }

  @Test
  public void testUsingRealJsons() throws InterruptedException, ExecutionException {
    Map<String, String> props = baseConnectorProps();
    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of(RowBuilder.SCALAR_FAMILY)));

    connect
        .kafka()
        .produce(
            testId,
            null,
            "{\"msg\":\"write\",\"key\":[\"test\",\"myset\",\"8Tvgv1f5PaBuA2Zuh7w767AciFA=\",null],\"gen\":3,\"exp\":0,\"lut\":1742850250806,\"bins\":[{\"name\":\"age\",\"type\":\"int\",\"value\":25}]}");
    waitUntilBigtableContainsNumberOfRows(testId, 1);

    ByteString key =
        ByteString.copyFrom(Base64.getDecoder().decode("8Tvgv1f5PaBuA2Zuh7w767AciFA="));

    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    assertEquals(Set.of(key), rows.keySet());
    Row row1 = rows.get(key);
    assertEquals(1, row1.getCells().size());
    RowCell cell = row1.getCells().get(0);
    assertEquals(RowBuilder.SCALAR_FAMILY, cell.getFamily());
    assertEquals(ByteString.copyFromUtf8("age__INT64"), cell.getQualifier());
    assertEquals(ByteString.copyFrom(Bytes.toBytes(25L)), cell.getValue());

    connect
        .kafka()
        .produce(
            testId,
            null,
            "{\"msg\":\"delete\",\"key\":[\"test\",\"myset\",\"8Tvgv1f5PaBuA2Zuh7w767AciFA=\",null],\"gen\":4,\"lut\":1742850253809}");
    waitUntilBigtableContainsNumberOfRows(testId, 0);

    assertConnectorAndAllTasksAreRunning(testId);
  }
}
