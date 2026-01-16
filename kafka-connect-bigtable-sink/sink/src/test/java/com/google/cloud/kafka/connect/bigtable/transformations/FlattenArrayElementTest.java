package com.google.cloud.kafka.connect.bigtable.transformations;

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
    FlattenArrayElement<SourceRecord> xform = new FlattenArrayElement<>();
    Map<String, String> props = new HashMap<>();
    props.put("array.field", "products");
    props.put("array.inner.wrapper", "list");
    props.put("array.element.wrapper", "element");
    xform.configure(props);
    return xform;
  }

  @Test
  public void testApply_Success() {
    FlattenArrayElement<SourceRecord> xform = getTransformer();

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

    // 5. Run the transformation
    SourceRecord record = new SourceRecord(null, null, "test-topic", 0,
        null, null, value.schema(), value);

    SourceRecord transformedRecord = xform.apply(record);

    // 6. Assertions
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
  public void testApply_ThrowsExceptionOnMissingField() {
    FlattenArrayElement<SourceRecord> xform = getTransformer();
    Schema simpleSchema = SchemaBuilder.struct().field("wrongField", Schema.STRING_SCHEMA).build();
    Struct simpleValue = new Struct(simpleSchema).put("wrongField", "data");

    SourceRecord record = new SourceRecord(null, null, "test-topic", 0,
        null, null, simpleSchema, simpleValue);

    assertThrows(DataException.class, () -> xform.apply(record));
  }
}
