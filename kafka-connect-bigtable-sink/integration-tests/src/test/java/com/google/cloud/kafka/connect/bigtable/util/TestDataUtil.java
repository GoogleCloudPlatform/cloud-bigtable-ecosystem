package com.google.cloud.kafka.connect.bigtable.util;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.cloud.bigtable.data.v2.models.Row;
import com.google.cloud.bigtable.data.v2.models.RowCell;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.json.JsonConverter;
import org.apache.kafka.connect.util.clusters.EmbeddedConnectCluster;

import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.List;

public class TestDataUtil {
  private static final ObjectMapper objectMapper = new ObjectMapper();
  public static final Schema orderProductSchema = SchemaBuilder.struct()
      .field("name", Schema.STRING_SCHEMA)
      .field("id", Schema.STRING_SCHEMA)
      .field("quantity", Schema.INT32_SCHEMA)
      .build();


  public static final Schema orderElementSchema = SchemaBuilder.struct().field("element", orderProductSchema).optional().build();

  public static final Schema orderSchema = SchemaBuilder.struct().optional()
      .field("orderId", Schema.STRING_SCHEMA)
      .field("userId", Schema.STRING_SCHEMA)
      .field("products",
          SchemaBuilder.struct().field("list", SchemaBuilder.array(orderElementSchema).optional()).build()
      )
      .build();

  public static void writeOrder(EmbeddedConnectCluster connect, String topic, String key, Order order) {
    JsonConverter converter = new JsonConverter();
    converter.configure(Collections.singletonMap("schemas.enable", "true"), false);

    List<Struct> productList = null;
    if (order.products != null) {
      productList = new ArrayList<>(order.products.length);
      for (OrderProduct product : order.products) {
        if (product == null) {
          productList.add(null);
          continue;
        }
        Struct productStruct = new Struct(orderElementSchema).put("element", new Struct(orderProductSchema)
            .put("name", product.name)
            .put("id", product.id)
            .put("quantity", product.quantity)
        );
        productList.add(productStruct);
      }
    }

    Struct productsWrapper = new Struct(orderSchema.field("products").schema())
        .put("list", productList);

    Struct value = new Struct(orderSchema)
        .put("orderId", order.orderId)
        .put("userId", order.userId)
        .put("products", productsWrapper);

    byte[] schemaAsJson = converter.fromConnectData(topic, orderSchema, value);
    connect.kafka().produce(topic, key, new String(schemaAsJson));
  }

  public static TestDataUtil.Order extractExpandedOrderFromRow(Row row) throws JsonProcessingException {
    String orderId = getValue(row, "cf", "orderId");
    String userId = getValue(row, "cf", "userId");

    List<RowCell> productCells = new ArrayList<>(row.getCells("products"));
    productCells.sort(Comparator.comparing(c -> c.getQualifier().toStringUtf8()));

    List<TestDataUtil.OrderProduct> productList = new ArrayList<>();
    for (RowCell cell : productCells) {
      String json = cell.getValue().toStringUtf8();
      TestDataUtil.OrderProduct product = objectMapper.readValue(json, TestDataUtil.OrderProduct.class);
      productList.add(product);
    }

    return new TestDataUtil.Order(
        orderId,
        userId,
        productList.toArray(new TestDataUtil.OrderProduct[0])
    );
  }

  private static String getValue(Row row, String family, String qualifier) {
    List<RowCell> cells = row.getCells(family, qualifier);
    if (cells == null || cells.isEmpty()) {
      return null;
    }
    // Return the latest cell (Bigtable returns them ordered by timestamp descending)
    return cells.get(0).getValue().toStringUtf8();
  }

  public record Order(String orderId, String userId, OrderProduct[] products) {
  }

  public record OrderProduct(String name, String id, int quantity) {
  }
}
