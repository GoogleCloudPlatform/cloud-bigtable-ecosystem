package com.google.cloud.kafka.connect.bigtable.integration;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.google.cloud.bigtable.data.v2.models.Row;
import com.google.cloud.bigtable.data.v2.models.RowCell;
import com.google.cloud.kafka.connect.bigtable.config.BigtableErrorMode;
import com.google.cloud.kafka.connect.bigtable.config.InsertMode;
import com.google.cloud.kafka.connect.bigtable.transformations.FlattenArrayElement;
import com.google.protobuf.ByteString;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.json.JsonConverter;
import org.apache.kafka.connect.storage.StringConverter;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

import java.nio.charset.StandardCharsets;
import java.util.*;
import java.util.concurrent.ExecutionException;

import static com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig.*;
import static org.apache.kafka.connect.runtime.WorkerConfig.KEY_CONVERTER_CLASS_CONFIG;
import static org.apache.kafka.connect.runtime.WorkerConfig.VALUE_CONVERTER_CLASS_CONFIG;
import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertNotNull;

@RunWith(JUnit4.class)

public class FlattenArrayElementIT extends BaseKafkaConnectBigtableIT {
  private static final String KEY1 = "key1";

  @Test
  public void testFlattenArrayElementSmt() throws InterruptedException, ExecutionException, JsonProcessingException {
    Map<String, String> props = baseConnectorProps();
    props.put(INSERT_MODE_CONFIG, InsertMode.UPSERT.name());
    props.put("transforms", "flattenElements");
    props.put("transforms.flattenElements.type", FlattenArrayElement.class.getName());
    props.put("transforms.flattenElements." + FlattenArrayElement.ARRAY_FIELD_NAME, "products");
    props.put("transforms.flattenElements." + FlattenArrayElement.ARRAY_INNER_WRAPPER_FIELD_NAME, "list");
    props.put("transforms.flattenElements." + FlattenArrayElement.ARRAY_ELEMENT_WRAPPER_FIELD_NAME, "element");
    props.put(DEFAULT_COLUMN_FAMILY_CONFIG, "cf");
    props.put(ERROR_MODE_CONFIG, BigtableErrorMode.FAIL.name());
    props.put(KEY_CONVERTER_CLASS_CONFIG, StringConverter.class.getName());
    props.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of(testId, "cf", "products")));

    Schema productSchema = SchemaBuilder.struct()
        .field("name", Schema.STRING_SCHEMA)
        .field("id", Schema.STRING_SCHEMA)
        .field("quantity", Schema.INT32_SCHEMA)
        .build();

    Schema elementSchema = SchemaBuilder.struct().field("element", productSchema).build();

    Schema schema = SchemaBuilder.struct().optional()
        .field("orderId", Schema.STRING_SCHEMA)
        .field("userId", Schema.STRING_SCHEMA)
        .field("products",
            SchemaBuilder.struct().field("list", SchemaBuilder.array(elementSchema)).build()
        )
        .build();

    JsonConverter converter = new JsonConverter();
    converter.configure(Collections.singletonMap("schemas.enable", "true"), false);

    Struct productElement1 = new Struct(elementSchema).put("element", new Struct(productSchema)
        .put("name", "Ball")
        .put("id", "PROD-123")
        .put("quantity", 5)
    );
    Struct productElement2 = new Struct(elementSchema).put("element", new Struct(productSchema)
        .put("name", "Car")
        .put("id", "PROD-456")
        .put("quantity", 1)
    );
    Struct productElement3 = new Struct(elementSchema).put("element", new Struct(productSchema)
        .put("name", "Tambourine")
        .put("id", "PROD-789")
        .put("quantity", 2)
    );

    List<Struct> productList = Arrays.stream(new Struct[]{productElement1, productElement2, productElement3}).toList();

    Struct productsWrapper = new Struct(schema.field("products").schema())
        .put("list", productList);

    Struct value = new Struct(schema)
        .put("orderId", "ORD-999")
        .put("userId", "USER-42")
        .put("products", productsWrapper);


    byte[] schemaAsJson = converter.fromConnectData(testId, schema, value);

    connect.kafka().produce(testId, KEY1, new String(schemaAsJson));

    waitUntilBigtableContainsNumberOfRows(testId, 1);
    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    ByteString key = ByteString.copyFrom(KEY1.getBytes(StandardCharsets.UTF_8));
    Row row1 = rows.get(key);
    assertNotNull(row1);
    assertEquals(3, row1.getCells().size());

    List<RowCell> orderIdCells = row1.getCells("cf", "orderId");
    assertEquals(1, orderIdCells.size());
    assertEquals("ORD-999", orderIdCells.get(0).getValue().toString(StandardCharsets.UTF_8));

    List<RowCell> userIdCells = row1.getCells("cf", "userId");
    assertEquals(1, userIdCells.size());
    assertEquals("USER-42", userIdCells.get(0).getValue().toString(StandardCharsets.UTF_8));

    List<RowCell> productCells = row1.getCells("cf", "products");
    assertEquals(1, productCells.size());
    String rawProductsJson = productCells.get(0).getValue().toString(StandardCharsets.UTF_8);

    ObjectMapper mapper = new ObjectMapper();
    ArrayNode productsJson = (ArrayNode) mapper.readTree(rawProductsJson);
    assertEquals(3, productsJson.size());

    // product 1
    assertEquals("Ball", productsJson.get(0).get("name").asText());
    assertEquals("PROD-123", productsJson.get(0).get("id").asText());
    assertEquals(5, productsJson.get(0).get("quantity").asInt());

    // product 2
    assertEquals("Car", productsJson.get(1).get("name").asText());
    assertEquals("PROD-456", productsJson.get(1).get("id").asText());
    assertEquals(1, productsJson.get(1).get("quantity").asInt());

    // product 3
    assertEquals("Tambourine", productsJson.get(2).get("name").asText());
    assertEquals("PROD-789", productsJson.get(2).get("id").asText());
    assertEquals(2, productsJson.get(2).get("quantity").asInt());
  }
}
