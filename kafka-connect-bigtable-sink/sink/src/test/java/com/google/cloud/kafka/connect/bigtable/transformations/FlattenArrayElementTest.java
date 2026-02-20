package com.google.cloud.kafka.connect.bigtable.transformations;

import com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig;
import org.apache.kafka.common.config.ConfigException;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.errors.DataException;
import org.apache.kafka.connect.source.SourceRecord;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

import java.util.*;

import static org.junit.Assert.*;


@RunWith(JUnit4.class)
public class FlattenArrayElementTest {

  private FlattenArrayElement<SourceRecord> getTransformer() {
    FlattenArrayElement<SourceRecord> smt = new FlattenArrayElement<>();
    Map<String, String> props = new HashMap<>();
    props.put("array.field", "products");
    props.put("array.inner.wrapper", "list");
    props.put("array.element.wrapper", "element");
    smt.configure(props);
    return smt;
  }

  @Test
  public void testApply_Success() {
    FlattenArrayElement<SourceRecord> smt = getTransformer();

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

    SourceRecord record = new SourceRecord(null, null, "test-topic", 0,
        null, null, value.schema(), value);

    SourceRecord transformedRecord = smt.apply(record);

    assertNotNull(transformedRecord);
    Struct resultValue = (Struct) transformedRecord.value();

    // Verify root level field still exists
    assertEquals("ORD-999", resultValue.get("orderId"));

    // Verify the array structure is flattened (unwrapped from "element")
    List<Struct> resultList = resultValue.getArray("products");
    assertEquals(3, resultList.size());

    assertEquals("Ball", resultList.get(0).getString("name"));
    assertEquals("PROD-123", resultList.get(0).getString("id"));
    assertEquals(5, resultList.get(0).getInt32("quantity").intValue());
    assertEquals("Car", resultList.get(1).getString("name"));
    assertEquals("PROD-456", resultList.get(1).getString("id"));
    assertEquals(1, resultList.get(1).getInt32("quantity").intValue());
    assertEquals("Tambourine", resultList.get(2).getString("name"));
    assertEquals("PROD-789", resultList.get(2).getString("id"));
    assertEquals(2, resultList.get(2).getInt32("quantity").intValue());

    // Verify schema change: The array's value schema should now be the elementSchema
    assertEquals(Schema.Type.ARRAY, resultValue.schema().field("products").schema().type());
    Schema productValueSchema = resultValue.schema().field("products").schema().valueSchema();
    assertNotNull(productValueSchema.field("name"));
    assertEquals(Schema.STRING_SCHEMA, productValueSchema.field("name").schema());
    assertEquals(Schema.STRING_SCHEMA, productValueSchema.field("id").schema());
    assertEquals(Schema.INT32_SCHEMA, productValueSchema.field("quantity").schema());
  }

  @Test
  public void testConvertSchema() {
    Schema productSchema = SchemaBuilder.struct()
        .field("name", Schema.STRING_SCHEMA)
        .field("id", Schema.STRING_SCHEMA)
        .field("quantity", Schema.INT32_SCHEMA)
        .build();

    Schema elementSchema = SchemaBuilder.struct().field("element", productSchema).build();

    Schema schema = SchemaBuilder.struct()
        .field("orderId", Schema.STRING_SCHEMA)
        .field("userId", Schema.INT32_SCHEMA)
        .field("products",
            SchemaBuilder.struct().field("list", SchemaBuilder.array(elementSchema)).build()
        )
        .build();
    Schema result = getTransformer().convertSchema(schema);
    assertEquals(Schema.Type.STRING, result.schema().field("orderId").schema().type());
    assertEquals(Schema.Type.INT32, result.schema().field("userId").schema().type());
    assertEquals(Schema.Type.ARRAY, result.schema().field("products").schema().type());
    Schema productValueSchema = result.schema().field("products").schema().valueSchema();
    assertNotNull(productValueSchema.field("name"));
    assertEquals(Schema.STRING_SCHEMA, productValueSchema.field("name").schema());
    assertEquals(Schema.STRING_SCHEMA, productValueSchema.field("id").schema());
    assertEquals(Schema.INT32_SCHEMA, productValueSchema.field("quantity").schema());
  }

  @Test
  public void testConvertSchemaWrongTypeRootLevelField() {
    Schema schema = SchemaBuilder.struct()
        .field("orderId", Schema.STRING_SCHEMA)
        .field("userId", Schema.INT32_SCHEMA)
        .field("products", Schema.STRING_SCHEMA)
        .build();
    Exception e = assertThrows(DataException.class, () -> getTransformer().convertSchema(schema));
    assertEquals("Root level field 'products' is not a struct", e.getMessage());
  }

  @Test
  public void testConvertSchemaWrongTypeArrayWrapperField() {
    Schema schema = SchemaBuilder.struct()
        .field("orderId", Schema.STRING_SCHEMA)
        .field("userId", Schema.INT32_SCHEMA)
        .field("products",
            SchemaBuilder.struct().field("list", Schema.STRING_SCHEMA).build()
        )
        .build();
    Exception e = assertThrows(DataException.class, () -> getTransformer().convertSchema(schema));
    assertEquals("Array field: 'list' is not an array", e.getMessage());
  }

  @Test
  public void testConvertSchemaWrongTypeArrayElementField() {
    Schema elementSchema = SchemaBuilder.struct().field("element", Schema.STRING_SCHEMA).build();

    Schema schema = SchemaBuilder.struct()
        .field("orderId", Schema.STRING_SCHEMA)
        .field("userId", Schema.INT32_SCHEMA)
        .field("products",
            SchemaBuilder.struct().field("list", SchemaBuilder.array(elementSchema)).build()
        )
        .build();
    Exception e = assertThrows(DataException.class, () -> getTransformer().convertSchema(schema));
    assertEquals("Array element wrapper: 'element' is not a struct", e.getMessage());
  }

  @Test
  public void testConvertSchemaMissingRootLevelField() {
    Schema schema = SchemaBuilder.struct()
        .field("orderId", Schema.STRING_SCHEMA)
        .field("userId", Schema.STRING_SCHEMA)
        .build();
    Exception e = assertThrows(DataException.class, () -> getTransformer().convertSchema(schema));
    assertEquals("Missing root level field: 'products'", e.getMessage());
  }

  @Test
  public void testConvertSchemaMissingArrayWrapper() {
    Schema schema = SchemaBuilder.struct().optional()
        .field("orderId", Schema.STRING_SCHEMA)
        .field("userId", Schema.STRING_SCHEMA)
        .field("products",
            SchemaBuilder.struct().field("foo", Schema.STRING_SCHEMA).build()
        )
        .build();

    Exception e = assertThrows(DataException.class, () -> getTransformer().convertSchema(schema));
    assertEquals("Missing inner field: 'list'", e.getMessage());
  }

  @Test
  public void testConvertSchemaMissingElementWrapper() {
    Schema productSchema = SchemaBuilder.struct()
        .field("name", Schema.STRING_SCHEMA)
        .field("id", Schema.STRING_SCHEMA)
        .field("quantity", Schema.INT32_SCHEMA)
        .build();
    Schema schema = SchemaBuilder.struct().optional()
        .field("orderId", Schema.STRING_SCHEMA)
        .field("userId", Schema.STRING_SCHEMA)
        .field("products",
            SchemaBuilder.struct().field("list", SchemaBuilder.array(productSchema)).build()
        )
        .build();
    Exception e = assertThrows(DataException.class, () -> getTransformer().convertSchema(schema));
    assertEquals("Missing array element field: 'element'", e.getMessage());
  }

  @Test
  public void testApplyNullRootArray() {
    FlattenArrayElement<SourceRecord> smt = getTransformer();

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
            SchemaBuilder.struct().field("list", SchemaBuilder.array(elementSchema)).optional().build()
        )
        .build();

    Struct value = new Struct(schema)
        .put("orderId", "ORD-999")
        .put("userId", "USER-42")
        .put("products", null);

    SourceRecord record = new SourceRecord(null, null, "test-topic", 0,
        null, null, value.schema(), value);

    SourceRecord transformedRecord = smt.apply(record);

    assertNotNull(transformedRecord);
    Struct resultValue = (Struct) transformedRecord.value();

    assertEquals("ORD-999", resultValue.get("orderId"));

    List<Struct> resultList = resultValue.getArray("products");
    assertEquals(0, resultList.size());
  }

  @Test
  public void testApplyNullNestedArray() {
    FlattenArrayElement<SourceRecord> smt = getTransformer();

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
            SchemaBuilder.struct().field("list", SchemaBuilder.array(elementSchema).optional()).build()
        )
        .build();

    Struct productsWrapper = new Struct(schema.field("products").schema())
        .put("list", null);

    Struct value = new Struct(schema)
        .put("orderId", "ORD-999")
        .put("userId", "USER-42")
        .put("products", productsWrapper);

    SourceRecord record = new SourceRecord(null, null, "test-topic", 0,
        null, null, value.schema(), value);

    SourceRecord transformedRecord = smt.apply(record);

    assertNotNull(transformedRecord);
    Struct resultValue = (Struct) transformedRecord.value();

    assertEquals("ORD-999", resultValue.get("orderId"));

    List<Struct> resultList = resultValue.getArray("products");
    assertEquals(0, resultList.size());
  }

  @Test
  public void testApplyNullArrayElement() {
    FlattenArrayElement<SourceRecord> smt = getTransformer();

    Schema productSchema = SchemaBuilder.struct()
        .field("name", Schema.STRING_SCHEMA)
        .field("id", Schema.STRING_SCHEMA)
        .field("quantity", Schema.INT32_SCHEMA)
        .build();

    Schema elementSchema = SchemaBuilder.struct().field("element", productSchema).optional().build();

    Schema schema = SchemaBuilder.struct()
        .field("orderId", Schema.STRING_SCHEMA)
        .field("userId", Schema.STRING_SCHEMA)
        .field("products",
            SchemaBuilder.struct().field("list", SchemaBuilder.array(elementSchema)).build()
        )
        .build();
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

    List<Struct> productList = Arrays.stream(new Struct[]{null, productElement2, productElement3}).toList();

    Struct productsWrapper = new Struct(schema.field("products").schema())
        .put("list", productList);

    Struct value = new Struct(schema)
        .put("orderId", "ORD-999")
        .put("userId", "USER-42")
        .put("products", productsWrapper);

    SourceRecord record = new SourceRecord(null, null, "test-topic", 0,
        null, null, value.schema(), value);

    SourceRecord transformedRecord = smt.apply(record);

    assertNotNull(transformedRecord);
    Struct resultValue = (Struct) transformedRecord.value();

    assertEquals("ORD-999", resultValue.get("orderId"));

    List<Struct> resultList = resultValue.getArray("products");
    assertEquals(3, resultList.size());

    assertNull(resultList.get(0));
    assertEquals("Car", resultList.get(1).getString("name"));
    assertEquals("PROD-456", resultList.get(1).getString("id"));
    assertEquals(1, resultList.get(1).getInt32("quantity").intValue());
    assertEquals("Tambourine", resultList.get(2).getString("name"));
    assertEquals("PROD-789", resultList.get(2).getString("id"));
    assertEquals(2, resultList.get(2).getInt32("quantity").intValue());

    assertEquals(Schema.Type.ARRAY, resultValue.schema().field("products").schema().type());
    Schema productValueSchema = resultValue.schema().field("products").schema().valueSchema();
    assertNotNull(productValueSchema.field("name"));
    assertEquals(Schema.STRING_SCHEMA, productValueSchema.field("name").schema());
    assertEquals(Schema.STRING_SCHEMA, productValueSchema.field("id").schema());
    assertEquals(Schema.INT32_SCHEMA, productValueSchema.field("quantity").schema());
  }

  @Test
  public void testApplyNullArrayElementValue() {
    FlattenArrayElement<SourceRecord> smt = getTransformer();

    Schema productSchema = SchemaBuilder.struct().optional()
        .field("name", Schema.STRING_SCHEMA)
        .field("id", Schema.STRING_SCHEMA)
        .field("quantity", Schema.INT32_SCHEMA)
        .build();

    Schema elementSchema = SchemaBuilder.struct().field("element", productSchema).build();

    Schema schema = SchemaBuilder.struct()
        .field("orderId", Schema.STRING_SCHEMA)
        .field("userId", Schema.STRING_SCHEMA)
        .field("products",
            SchemaBuilder.struct().field("list", SchemaBuilder.array(elementSchema)).build()
        )
        .build();
    Struct productElement1 = new Struct(elementSchema).put("element", null);
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

    SourceRecord record = new SourceRecord(null, null, "test-topic", 0,
        null, null, value.schema(), value);

    SourceRecord transformedRecord = smt.apply(record);

    assertNotNull(transformedRecord);
    Struct resultValue = (Struct) transformedRecord.value();

    assertEquals("ORD-999", resultValue.get("orderId"));

    List<Struct> resultList = resultValue.getArray("products");
    assertEquals(3, resultList.size());

    assertNull(resultList.get(0));
    assertEquals("Car", resultList.get(1).getString("name"));
    assertEquals("PROD-456", resultList.get(1).getString("id"));
    assertEquals(1, resultList.get(1).getInt32("quantity").intValue());
    assertEquals("Tambourine", resultList.get(2).getString("name"));
    assertEquals("PROD-789", resultList.get(2).getString("id"));
    assertEquals(2, resultList.get(2).getInt32("quantity").intValue());
  }

  @Test
  public void testApplyMixedNulls() {
    FlattenArrayElement<SourceRecord> smt = getTransformer();

    Schema productSchema = SchemaBuilder.struct().optional()
        .field("name", Schema.STRING_SCHEMA)
        .build();

    Schema elementSchema = SchemaBuilder.struct().field("element", productSchema).optional().build();

    Schema schema = SchemaBuilder.struct()
        .field("products",
            SchemaBuilder.struct().field("list", SchemaBuilder.array(elementSchema)).build()
        )
        .build();

    // 1. null element wrapper
    // 2. element wrapper with null value
    // 3. valid element
    Struct productElement2 = new Struct(elementSchema).put("element", null);
    Struct productElement3 = new Struct(elementSchema).put("element", new Struct(productSchema).put("name", "Ball"));

    List<Struct> productList = Arrays.asList(null, productElement2, productElement3);

    Struct productsWrapper = new Struct(schema.field("products").schema())
        .put("list", productList);

    Struct value = new Struct(schema).put("products", productsWrapper);

    SourceRecord record = new SourceRecord(null, null, "test-topic", 0,
        null, null, value.schema(), value);

    SourceRecord transformedRecord = smt.apply(record);

    assertNotNull(transformedRecord);
    Struct resultValue = (Struct) transformedRecord.value();

    List<Struct> resultList = resultValue.getArray("products");
    assertEquals(3, resultList.size());

    assertNull(resultList.get(0));
    assertNull(resultList.get(1));
    assertEquals("Ball", resultList.get(2).getString("name"));
  }
}
