package com.google.cloud.kafka.connect.bigtable.transformations;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.apache.kafka.common.config.ConfigDef;
import org.apache.kafka.connect.connector.ConnectRecord;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.errors.DataException;
import org.apache.kafka.connect.transforms.Transformation;
import org.apache.kafka.connect.transforms.util.SimpleConfig;

import java.io.IOException;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;
import java.util.Map;

public abstract class ApplyJsonSchema<R extends ConnectRecord<R>> implements Transformation<R> {

  public static final String OVERVIEW_DOC =
      "Applies a static schema defined in JSON format to a schemaless record.";

  public static final String SCHEMA_CONFIG = "schema.json";
  private static final String SCHEMA_DOC = "The schema definition in JSON format.";

  private static final ConfigDef CONFIG_DEF = new ConfigDef()
      .define(SCHEMA_CONFIG, ConfigDef.Type.STRING, ConfigDef.Importance.HIGH, SCHEMA_DOC);

  private Schema targetSchema;
  private static final ObjectMapper OBJECT_MAPPER = new ObjectMapper();

  @Override
  public void configure(Map<String, ?> props) {
    final SimpleConfig config = new SimpleConfig(CONFIG_DEF, props);
    String schemaJson = config.getString(SCHEMA_CONFIG);

    try {
      JsonNode schemaNode = OBJECT_MAPPER.readTree(schemaJson);
      this.targetSchema = parseSchemaNode(schemaNode);
    } catch (IOException e) {
      throw new DataException("Unable to parse schema JSON from configuration", e);
    }
  }

  @Override
  public R apply(R record) {
    if (record.value() == null) {
      return record;
    }

    // We assume the incoming record is schemaless (Value is a Map<String, Object>)
    Object newValue = convertToConnectData("(root)", record.value(), targetSchema);

    return record.newRecord(
        record.topic(),
        record.kafkaPartition(),
        record.keySchema(),
        record.key(),
        targetSchema,
        newValue,
        record.timestamp()
    );
  }

  /**
   * Recursively converts a Java Object (Map, List, or Primitive) into a Connect Struct/Array
   * based on the provided Schema.
   */
  private Object convertToConnectData(String name, Object input, Schema schema) {
    if (input == null && schema.isOptional()) {
      return null;
    } else if (input == null) {
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
        // Handle Map logic if necessary (omitted for brevity, similar to Array)
        return input;
      default:
        // For primitives (STRING, INT32, etc.), we rely on Connect's loose typing
        // or specific casting here if strictly required.
        return getPrimitiveValue(input, schema);
    }
  }

  private Object getPrimitiveValue(Object input, Schema schema) {
    switch (schema.type()) {
      case INT8:
      case INT16:
        return ((Number) input).shortValue();
      case INT32:
        return ((Number) input).intValue();
      case INT64:
        return ((Number) input).longValue();
      case FLOAT32:
        return ((Number) input).floatValue();
      case FLOAT64:
        return ((Number) input).doubleValue();
      case BOOLEAN:
      case ARRAY:
      case MAP:
      case STRUCT:
      case STRING:
      case BYTES:
      default:
        return input;
    }
  }

  /**
   * Parses a JSON representation of a Schema into a Schema object.
   * Expects format: { "type": "struct", "optional": true, "fields": [...] }
   */
  private Schema parseSchemaNode(JsonNode node) {
    String type = node.get("type").asText().toLowerCase();
    String name = node.has("name") ? node.get("name").asText() : "UNKNOWN";
    boolean isOptional = node.has("optional") && node.get("optional").asBoolean();

    SchemaBuilder builder;

    switch (type) {
      case "struct":
        builder = SchemaBuilder.struct();
        if (node.has("name")) {
          builder.name(node.get("name").asText());
        }
        JsonNode fields = node.get("fields");
        if (fields != null && fields.isArray()) {
          for (JsonNode field : fields) {
            String fieldName = field.get("field").asText();
            builder.field(fieldName, parseSchemaNode(field));
          }
        }
        break;
      case "array":
        JsonNode items = node.get("items");
        if (items == null) {
          throw new DataException("Array type must specify 'items' schema");
        }
        builder = SchemaBuilder.array(parseSchemaNode(items));
        break;
      case "string":
        builder = SchemaBuilder.string();
        break;
      case "int8":
        builder = SchemaBuilder.int8();
        break;
      case "int16":
        builder = SchemaBuilder.int16();
        break;
      case "int32":
        builder = SchemaBuilder.int32();
        break;
      case "int64":
        builder = SchemaBuilder.int64();
        break;
      case "float32":
        builder = SchemaBuilder.float32();
        break;
      case "float64":
        builder = SchemaBuilder.float64();
        break;
      case "boolean":
        builder = SchemaBuilder.bool();
        break;
      default:
        throw new DataException(String.format("Unsupported schema type in config for field %s: %s", name, type));
    }

    if (isOptional) {
      builder.optional();
    }
    return builder.build();
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
  }

  public static class Value<R extends ConnectRecord<R>> extends ApplyJsonSchema<R> {
  }
}