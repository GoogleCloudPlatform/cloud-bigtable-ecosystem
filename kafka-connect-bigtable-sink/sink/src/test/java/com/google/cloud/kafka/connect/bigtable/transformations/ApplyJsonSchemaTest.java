package com.example.kafka.connect.smt;

import com.google.cloud.kafka.connect.bigtable.transformations.ApplyJsonSchema;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.errors.DataException;
import org.apache.kafka.connect.sink.SinkRecord;
import org.apache.kafka.connect.source.SourceRecord;
import org.junit.After;
import org.junit.Before;
import org.junit.Test;

import java.util.*;

import static org.junit.Assert.*;

public class ApplyJsonSchemaTest {

  private ApplyJsonSchema.Value<SourceRecord> smt;

  // A complex schema with a nested struct and an array
  private static final String CONFIG_SCHEMA_JSON = "{" +
      "  \"type\": \"struct\"," +
      "  \"fields\": [" +
      "    { \"field\": \"id\", \"type\": \"int32\" }," +
      "    { \"field\": \"meta\", \"type\": \"struct\", \"fields\": [" +
      "        { \"field\": \"source\", \"type\": \"string\" }," +
      "        { \"field\": \"retry_count\", \"type\": \"int32\", \"optional\": true }" +
      "      ]" +
      "    }," +
      "    { \"field\": \"tags\", \"type\": \"array\", \"items\": { \"type\": \"string\" } }" +
      "  ]" +
      "}";

  @Before
  public void setUp() {
    smt = new ApplyJsonSchema.Value<>();
    Map<String, String> configs = new HashMap<>();
    configs.put(ApplyJsonSchema.SCHEMA_CONFIG, CONFIG_SCHEMA_JSON);
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
  public void testComplexObject() {
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
    configs.put(ApplyJsonSchema.SCHEMA_CONFIG, "{\"type\":\"struct\",\"fields\":[{\"type\":\"string\",\"optional\":false,\"field\":\"orderId\"},{\"type\":\"string\",\"optional\":false,\"field\":\"userId\"},{\"type\":\"struct\",\"fields\":[{\"type\":\"array\",\"items\":{\"type\":\"struct\",\"fields\":[{\"type\":\"struct\",\"fields\":[{\"type\":\"string\",\"optional\":false,\"field\":\"name\"},{\"type\":\"string\",\"optional\":false,\"field\":\"id\"},{\"type\":\"int32\",\"optional\":false,\"field\":\"quantity\"}],\"optional\":false,\"field\":\"element\"}],\"optional\":true},\"optional\":true,\"field\":\"list\"}],\"optional\":false,\"field\":\"products\"}],\"optional\":true}");
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
}