package com.google.cloud.kafka.connect.bigtable.util;

import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.json.JsonConverter;
import org.apache.kafka.connect.util.clusters.EmbeddedConnectCluster;

import java.util.*;

public class TestDataUtil {
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
          SchemaBuilder.struct().field("list", SchemaBuilder.array(orderElementSchema)).build()
      )
      .build();

  public static void writeOrder(EmbeddedConnectCluster connect, String topic, String key, Order order) {
    JsonConverter converter = new JsonConverter();
    converter.configure(Collections.singletonMap("schemas.enable", "true"), false);

    List<Struct> productList = new ArrayList<>(order.products.length);
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

    Struct productsWrapper = new Struct(orderSchema.field("products").schema())
        .put("list", productList);

    Struct value = new Struct(orderSchema)
        .put("orderId", order.orderId)
        .put("userId", order.userId)
        .put("products", productsWrapper);

    byte[] schemaAsJson = converter.fromConnectData(topic, orderSchema, value);
    connect.kafka().produce(topic, key, new String(schemaAsJson));
  }

  public record Order(String orderId, String userId, OrderProduct[] products) {
  }

  public record OrderProduct(String name, String id, int quantity) {
  }
}
