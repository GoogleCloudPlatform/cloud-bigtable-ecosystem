package com.google.cloud.kafka.connect.bigtable.transformations;

import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.errors.DataException;
import org.apache.kafka.connect.source.SourceRecord;
import org.junit.After;
import org.junit.Before;
import org.junit.Test;

import java.io.File;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.*;

import static org.junit.Assert.*;

public class ApplyJsonSchemaTest {

  private ApplyJsonSchema.Value<SourceRecord> smt;

  private static final String BASIC_SCHEMA_FILE = "basic-schema.json";
  private static final String COMPLEX_SCHEMA_FILE = "complex-order-schema.json";
  private static final String DEFAULT_VALUES_SCHEMA_FILE = "default-values-schema.json";

  private String readResource(String fileName) throws IOException {
    try (var is = getClass().getClassLoader().getResourceAsStream(fileName)) {
      if (is == null) {
        throw new IOException("Resource not found: " + fileName);
      }
      return new String(is.readAllBytes(), StandardCharsets.UTF_8);
    }
  }

  @Before
  public void setUp() throws IOException {
    smt = new ApplyJsonSchema.Value<>();
    Map<String, String> configs = new HashMap<>();
    configs.put(ApplyJsonSchema.SCHEMA_CONFIG, readResource(BASIC_SCHEMA_FILE));
    smt.configure(configs);
  }

  @After
  public void tearDown() {
    smt.close();
  }

  @Test
  public void testApplyJsonSchemalessToStruct() {
    // 1. Construct the schemaless input (Nested Map)
    Map<String, Object> metaMap = new HashMap<>();
    metaMap.put("source", "api-v1");
    metaMap.put("retry_count", 5);

    Map<String, Object> rootMap = new HashMap<>();
    rootMap.put("id", 101);
    rootMap.put("meta", metaMap);
    rootMap.put("tags", Arrays.asList("production", "urgent"));

    // 2. Create the SourceRecord with NO schema
    SourceRecord record = new SourceRecord(
        null, null, "test-topic", 0,
        null, rootMap
    );

    // 3. Apply the SMT
    SourceRecord transformed = smt.apply(record);

    // 4. Assertions
    assertNotNull(transformed.valueSchema());
    assertEquals(Schema.Type.STRUCT, transformed.valueSchema().type());
    assertTrue(transformed.value() instanceof Struct);

    Struct rootStruct = (Struct) transformed.value();
    assertEquals((Integer) 101, rootStruct.getInt32("id"));

    // Check Nested Struct
    Struct metaStruct = rootStruct.getStruct("meta");
    assertEquals("api-v1", metaStruct.getString("source"));
    assertEquals((Integer) 5, metaStruct.getInt32("retry_count"));

    // Check Array
    java.util.List<String> tags = rootStruct.getArray("tags");
    assertEquals(2, tags.size());
    assertEquals("production", tags.get(0));
  }

  @Test
  public void testOptionalFieldIsNull() {
    // 'retry_count' is optional in our JSON config. Let's omit it.
    Map<String, Object> metaMap = new HashMap<>();
    metaMap.put("source", "api-v2");
    // retry_count missing -> should be null

    Map<String, Object> rootMap = new HashMap<>();
    rootMap.put("id", 202);
    rootMap.put("meta", metaMap);
    rootMap.put("tags", Collections.emptyList());

    SourceRecord record = new SourceRecord(null, null, "topic", null, rootMap);
    SourceRecord transformed = smt.apply(record);

    Struct rootStruct = (Struct) transformed.value();
    Struct metaStruct = rootStruct.getStruct("meta");

    // Assert that the optional field is null
    assertNull(metaStruct.get("retry_count"));
  }

  @Test
  public void testComplexObject() throws IOException {
    Map<String, Object> product1 = new HashMap<>();
    product1.put("name", "Wireless Headphones");
    product1.put("id", "PROD-123");
    product1.put("quantity", 2);

    Map<String, Object> product2 = new HashMap<>();
    product2.put("name", "HDMI Cable");
    product2.put("id", "PROD-456");
    product2.put("quantity", 5);

    // 2. Wrap them in the 'Element' map (orderElementSchema)
    // Schema requires: field("element", orderProductSchema)
    Map<String, Object> elementWrapper1 = new HashMap<>();
    elementWrapper1.put("element", product1);

    Map<String, Object> elementWrapper2 = new HashMap<>();
    elementWrapper2.put("element", product2);

    // 3. Create the List for 'products.list'
    // Schema requires: SchemaBuilder.array(orderElementSchema)
    List<Map<String, Object>> productList = new ArrayList<>();
    productList.add(elementWrapper1);
    productList.add(elementWrapper2);

    // 4. Create the 'products' wrapper map
    // Schema requires: field("products", struct().field("list", ...))
    Map<String, Object> productsMap = new HashMap<>();
    productsMap.put("list", productList);

    // 5. Create the Root 'Order' map
    Map<String, Object> orderMap = new HashMap<>();
    orderMap.put("orderId", "ORD-2023-999");
    orderMap.put("userId", "USER-007");
    orderMap.put("products", productsMap);

    SourceRecord record = new SourceRecord(null, null, "topic", null, orderMap);
    ApplyJsonSchema.Value<SourceRecord> smt = new ApplyJsonSchema.Value<>();
    Map<String, String> configs = new HashMap<>();
    configs.put(ApplyJsonSchema.SCHEMA_CONFIG, readResource(COMPLEX_SCHEMA_FILE));
    smt.configure(configs);
    SourceRecord transformed = smt.apply(record);

    Struct rootStruct = (Struct) transformed.value();

    assertEquals("ORD-2023-999", rootStruct.getString("orderId"));
    assertEquals("USER-007", rootStruct.getString("userId"));

    Struct products = rootStruct.getStruct("products");
    assertNotNull(products);

    List<Object> list = products.getArray("list");
    assertEquals(2, list.size());
  }

  @Test(expected = DataException.class)
  public void testMissingRequiredFieldThrowsException() {
    // 'source' inside 'meta' is REQUIRED. Let's omit it.
    Map<String, Object> metaMap = new HashMap<>();
    // source missing

    Map<String, Object> rootMap = new HashMap<>();
    rootMap.put("id", 303);
    rootMap.put("meta", metaMap);

    SourceRecord record = new SourceRecord(null, null, "topic", null, rootMap);

    // Should throw DataException because "source" is missing
    smt.apply(record);
  }

  @Test
  public void testTombstoneRecord() {
    // A tombstone record has a null value
    SourceRecord record = new SourceRecord(
        null, null, "test-topic", 0,
        null, null
    );

    SourceRecord transformed = smt.apply(record);

    assertNull(transformed.value());
    assertNull(transformed.valueSchema());
  }

  @Test
  public void testBytesField() {
    String schemaJson = "{" +
        "  \"type\": \"struct\"," +
        "  \"fields\": [" +
        "    { \"field\": \"raw_bytes\", \"type\": \"bytes\" }," +
        "    { \"field\": \"base64_bytes\", \"type\": \"bytes\" }" +
        "  ]" +
        "}";

    ApplyJsonSchema.Value<SourceRecord> bytesSmt = new ApplyJsonSchema.Value<>();
    bytesSmt.configure(Collections.singletonMap(ApplyJsonSchema.SCHEMA_CONFIG, schemaJson));

    byte[] rawData = new byte[]{1, 2, 3, 4};
    String base64Data = Base64.getEncoder().encodeToString(new byte[]{5, 6, 7, 8});

    Map<String, Object> valueMap = new HashMap<>();
    valueMap.put("raw_bytes", rawData);
    valueMap.put("base64_bytes", base64Data);

    SourceRecord record = new SourceRecord(null, null, "topic", null, valueMap);
    SourceRecord transformed = bytesSmt.apply(record);

    Struct struct = (Struct) transformed.value();
    assertArrayEquals(rawData, (byte[]) struct.get("raw_bytes"));
    assertArrayEquals(new byte[]{5, 6, 7, 8}, (byte[]) struct.get("base64_bytes"));
  }

  @Test
  public void testNumericToStringConversion() {
    String schemaJson = "{\"type\": \"struct\", \"fields\": [{\"field\": \"string_field\", \"type\": \"string\"}]}";

    ApplyJsonSchema.Value<SourceRecord> stringSmt = new ApplyJsonSchema.Value<>();
    stringSmt.configure(Collections.singletonMap(ApplyJsonSchema.SCHEMA_CONFIG, schemaJson));

    Map<String, Object> valueMap = new HashMap<>();
    valueMap.put("string_field", 123.45);

    SourceRecord record = new SourceRecord(null, null, "topic", null, valueMap);
    SourceRecord transformed = stringSmt.apply(record);

    Struct struct = (Struct) transformed.value();
    assertEquals("123.45", struct.getString("string_field"));
    stringSmt.close();
  }

  @Test
  public void testDefaultValues() throws IOException {
    ApplyJsonSchema.Value<SourceRecord> defaultSmt = new ApplyJsonSchema.Value<>();
    defaultSmt.configure(Collections.singletonMap(ApplyJsonSchema.SCHEMA_CONFIG, readResource(DEFAULT_VALUES_SCHEMA_FILE)));

    // Input map is empty, so all fields should take their default values
    Map<String, Object> valueMap = new HashMap<>();

    SourceRecord record = new SourceRecord(null, null, "topic", null, valueMap);
    SourceRecord transformed = defaultSmt.apply(record);

    Struct struct = (Struct) transformed.value();
    assertEquals(42, struct.get("id"));
    assertEquals("unnamed", struct.get("name"));
    assertEquals(true, struct.get("active"));

    // Verify that explicitly provided values override defaults
    valueMap.put("id", 100);
    record = new SourceRecord(null, null, "topic", null, valueMap);
    transformed = defaultSmt.apply(record);
    struct = (Struct) transformed.value();
    assertEquals(100, struct.get("id"));
    assertEquals("unnamed", struct.get("name"));

    defaultSmt.close();
  }
}
