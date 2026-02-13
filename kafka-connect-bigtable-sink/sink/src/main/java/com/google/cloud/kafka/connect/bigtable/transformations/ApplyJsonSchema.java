package com.google.cloud.kafka.connect.bigtable.transformations;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.apache.kafka.common.config.ConfigDef;
import org.apache.kafka.connect.connector.ConnectRecord;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.errors.DataException;
import org.apache.kafka.connect.json.JsonConverter;
import org.apache.kafka.connect.transforms.Transformation;
import org.apache.kafka.connect.transforms.util.SimpleConfig;

import java.io.IOException;
import java.util.*;

public abstract class ApplyJsonSchema<R extends ConnectRecord<R>> implements Transformation<R> {

  public static final String OVERVIEW_DOC =
      "Applies a static schema defined in JSON format to a schemaless record.";

  public static final String SCHEMA_CONFIG = "schema.json";
  private static final String SCHEMA_DOC = "The schema definition in JSON format.";

  private static final ConfigDef CONFIG_DEF = new ConfigDef()
      .define(SCHEMA_CONFIG, ConfigDef.Type.STRING, ConfigDef.NO_DEFAULT_VALUE, new org.apache.kafka.common.config.ConfigDef.NonEmptyString(), ConfigDef.Importance.HIGH, SCHEMA_DOC);

  private Schema targetSchema;
  private static final ObjectMapper OBJECT_MAPPER = new ObjectMapper();

  @Override
  public void configure(Map<String, ?> props) {
    final SimpleConfig config = new SimpleConfig(CONFIG_DEF, props);
    String schemaJson = config.getString(SCHEMA_CONFIG);

    try (JsonConverter converter = new JsonConverter()) {
      converter.configure(Collections.singletonMap("schemas.enable", "true"), false);
      this.targetSchema = converter.asConnectSchema(OBJECT_MAPPER.readTree(schemaJson));
    } catch (IOException e) {
      throw new DataException("Unable to parse schema JSON from configuration", e);
    }
  }

  @Override
  public R apply(R record) {
    final Object value = operatingValue(record);
    if (value == null) {
      return record;
    }

    Object newValue = convertToConnectData("(root)", value, targetSchema);
    return newRecord(record, targetSchema, newValue);
  }

  protected abstract Object operatingValue(R record);

  protected abstract R newRecord(R record, Schema schema, Object value);

  /**
   * Recursively converts a Java Object (Map, List, or Primitive) into a Connect Struct/Array
   * based on the provided Schema.
   */
  private Object convertToConnectData(String name, Object input, Schema schema) {
    if (input == null) {
      if (schema.defaultValue() != null) {
        return schema.defaultValue();
      }
      if (schema.isOptional()) {
        return null;
      }
      throw new DataException("Found null value for non-optional schema field: " + name);
    }

    switch (schema.type()) {
      case STRUCT:
        if (!(input instanceof Map)) {
          throw new DataException(String.format("Expected Map for STRUCT schema for field %s, found %s", name, input.getClass()));
        }
        Struct struct = new Struct(schema);
        Map<?, ?> inputMap = (Map<?, ?>) input;

        for (org.apache.kafka.connect.data.Field field : schema.fields()) {
          Object fieldVal = inputMap.get(field.name());
          struct.put(field, convertToConnectData(field.name(), fieldVal, field.schema()));
        }
        return struct;
      case ARRAY:
        if (!(input instanceof List)) {
          throw new DataException(String.format("Expected List for ARRAY schema for field %s, found %s", name, input.getClass()));
        }
        List<?> inputList = (List<?>) input;
        List<Object> outputList = new ArrayList<>();
        Schema valueSchema = schema.valueSchema();

        for (int i = 0; i < inputList.size(); i++) {
          Object elem = inputList.get(i);
          outputList.add(convertToConnectData(name + "[" + i + "]", elem, valueSchema));
        }
        return outputList;
      case MAP:
        if (!(input instanceof Map)) {
          throw new DataException(String.format("Expected Map for MAP schema for field %s, found %s", name, input.getClass()));
        }
        Map<?, ?> inputMapForMap = (Map<?, ?>) input;
        Map<Object, Object> outputMap = new HashMap<>();
        Schema mapKeySchema = schema.keySchema();
        Schema mapValueSchema = schema.valueSchema();
        for (Map.Entry<?, ?> entry : inputMapForMap.entrySet()) {
          outputMap.put(
              convertToConnectData(name + "(key)", entry.getKey(), mapKeySchema),
              convertToConnectData(name + "(value)", entry.getValue(), mapValueSchema)
          );
        }
        return outputMap;
      default:
        // For primitives (STRING, INT32, etc.), we rely on Connect's loose typing
        // or specific casting here if strictly required.
        return getPrimitiveValue(input, schema);
    }
  }

  private Object getPrimitiveValue(Object input, Schema schema) {
    switch (schema.type()) {
      case INT8:
        if (!(input instanceof Number)) {
          throw new DataException("Expected Number for INT8, found " + input.getClass());
        }
        return ((Number) input).byteValue();
      case INT16:
        if (!(input instanceof Number)) {
          throw new DataException("Expected Number for INT16, found " + input.getClass());
        }
        return ((Number) input).shortValue();
      case INT32:
        if (!(input instanceof Number)) {
          throw new DataException("Expected Number for INT32, found " + input.getClass());
        }
        return ((Number) input).intValue();
      case INT64:
        if (!(input instanceof Number)) {
          throw new DataException("Expected Number for INT64, found " + input.getClass());
        }
        return ((Number) input).longValue();
      case FLOAT32:
        if (!(input instanceof Number)) {
          throw new DataException("Expected Number for FLOAT32, found " + input.getClass());
        }
        return ((Number) input).floatValue();
      case FLOAT64:
        if (!(input instanceof Number)) {
          throw new DataException("Expected Number for FLOAT64, found " + input.getClass());
        }
        return ((Number) input).doubleValue();
      case BOOLEAN:
        if (!(input instanceof Boolean)) {
          throw new DataException("Expected Boolean for BOOLEAN, found " + input.getClass());
        }
        return input;
      case BYTES:
        if (input instanceof String) {
          try {
            return java.util.Base64.getDecoder().decode((String) input);
          } catch (IllegalArgumentException e) {
            throw new DataException("Invalid Base64 string for BYTES field", e);
          }
        }
        if (input instanceof byte[]) {
          return input;
        }
        throw new DataException("Expected a Base64-encoded String or byte[] for BYTES field, but found " + input.getClass().getName());
      case STRING:
        if (input instanceof Map || input instanceof List) {
          throw new DataException("Expected a primitive for STRING field, but found " + input.getClass().getName());
        }
        return input.toString();
      default:
        return input;
    }
  }

  @Override
  public ConfigDef config() {
    return CONFIG_DEF;
  }

  @Override
  public void close() {
    // No resources to close
  }

  // Boilerplate for Key/Value distinct implementations
  public static class Key<R extends ConnectRecord<R>> extends ApplyJsonSchema<R> {
    @Override
    protected Object operatingValue(R record) {
      return record.key();
    }

    @Override
    protected R newRecord(R record, Schema schema, Object value) {
      return record.newRecord(
          record.topic(), record.kafkaPartition(),
          schema, value,
          record.valueSchema(), record.value(), record.timestamp());
    }
  }

  public static class Value<R extends ConnectRecord<R>> extends ApplyJsonSchema<R> {
    @Override
    protected Object operatingValue(R record) {
      return record.value();
    }

    @Override
    protected R newRecord(R record, Schema schema, Object value) {
      return record.newRecord(
          record.topic(), record.kafkaPartition(),
          record.keySchema(), record.key(),
          schema, value, record.timestamp());
    }
  }
}