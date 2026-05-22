/*
 * Copyright 2026 Google LLC
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
package com.google.cloud.kafka.connect.bigtable.util;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.cloud.bigtable.data.v2.models.Row;
import com.google.cloud.bigtable.data.v2.models.RowCell;
import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.Comparator;
import java.util.List;
import java.util.Objects;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.json.JsonConverter;
import org.apache.kafka.connect.util.clusters.EmbeddedConnectCluster;

public class TestDataUtil {
  private static final ObjectMapper objectMapper = new ObjectMapper();
  public static final Schema orderProductSchema =
      SchemaBuilder.struct()
          .field("name", Schema.STRING_SCHEMA)
          .field("id", Schema.STRING_SCHEMA)
          .field("quantity", Schema.INT32_SCHEMA)
          .build();

  public static final Schema orderElementSchema =
      SchemaBuilder.struct().field("element", orderProductSchema).optional().build();

  public static final Schema orderSchema =
      SchemaBuilder.struct()
          .optional()
          .field("orderId", Schema.STRING_SCHEMA)
          .field("userId", Schema.STRING_SCHEMA)
          .field(
              "products",
              SchemaBuilder.struct()
                  .field("list", SchemaBuilder.array(orderElementSchema).optional())
                  .build())
          .build();

  public static void writeOrder(
      EmbeddedConnectCluster connect, String topic, String key, Order order) {
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
        Struct productStruct =
            new Struct(orderElementSchema)
                .put(
                    "element",
                    new Struct(orderProductSchema)
                        .put("name", product.name)
                        .put("id", product.id)
                        .put("quantity", product.quantity));
        productList.add(productStruct);
      }
    }

    Struct productsWrapper =
        new Struct(orderSchema.field("products").schema()).put("list", productList);

    Struct value =
        new Struct(orderSchema)
            .put("orderId", order.orderId)
            .put("userId", order.userId)
            .put("products", productsWrapper);

    byte[] schemaAsJson = converter.fromConnectData(topic, orderSchema, value);
    connect.kafka().produce(topic, key, new String(schemaAsJson));
  }

  public static TestDataUtil.Order extractExpandedOrderFromRow(Row row)
      throws JsonProcessingException {
    String orderId = getValue(row, "cf", "orderId");
    String userId = getValue(row, "cf", "userId");

    List<RowCell> productCells = new ArrayList<>(row.getCells("products"));
    productCells.sort(Comparator.comparing(c -> c.getQualifier().toStringUtf8()));

    List<TestDataUtil.OrderProduct> productList = new ArrayList<>();
    for (RowCell cell : productCells) {
      String json = cell.getValue().toStringUtf8();
      TestDataUtil.OrderProduct product =
          objectMapper.readValue(json, TestDataUtil.OrderProduct.class);
      productList.add(product);
    }

    return new TestDataUtil.Order(
        orderId, userId, productList.toArray(new TestDataUtil.OrderProduct[0]));
  }

  private static String getValue(Row row, String family, String qualifier) {
    List<RowCell> cells = row.getCells(family, qualifier);
    if (cells == null || cells.isEmpty()) {
      return null;
    }
    // Return the latest cell (Bigtable returns them ordered by timestamp descending)
    return cells.get(0).getValue().toStringUtf8();
  }

  public static class Order {
    private final String orderId;
    private final String userId;
    private final OrderProduct[] products;

    public Order(String orderId, String userId, OrderProduct[] products) {
      this.orderId = orderId;
      this.userId = userId;
      this.products = products;
    }

    public String orderId() {
      return orderId;
    }

    public String userId() {
      return userId;
    }

    public OrderProduct[] products() {
      return products;
    }

    @Override
    public boolean equals(Object o) {
      if (this == o) return true;
      if (o == null || getClass() != o.getClass()) return false;
      Order order = (Order) o;
      return Objects.equals(orderId, order.orderId)
          && Objects.equals(userId, order.userId)
          && Arrays.equals(products, order.products);
    }

    @Override
    public int hashCode() {
      int result = Objects.hash(orderId, userId);
      result = 31 * result + Arrays.hashCode(products);
      return result;
    }

    @Override
    public String toString() {
      return "Order{"
          + "orderId='"
          + orderId
          + '\''
          + ", userId='"
          + userId
          + '\''
          + ", products="
          + Arrays.toString(products)
          + '}';
    }
  }

  public static class OrderProduct {
    private String name;
    private String id;
    private int quantity;

    public OrderProduct() {}

    public OrderProduct(String name, String id, int quantity) {
      this.name = name;
      this.id = id;
      this.quantity = quantity;
    }

    public String name() {
      return name;
    }

    public String id() {
      return id;
    }

    public int quantity() {
      return quantity;
    }

    public void setName(String name) {
      this.name = name;
    }

    public void setId(String id) {
      this.id = id;
    }

    public void setQuantity(int quantity) {
      this.quantity = quantity;
    }

    @Override
    public boolean equals(Object o) {
      if (this == o) return true;
      if (o == null || getClass() != o.getClass()) return false;
      OrderProduct that = (OrderProduct) o;
      return quantity == that.quantity
          && Objects.equals(name, that.name)
          && Objects.equals(id, that.id);
    }

    @Override
    public int hashCode() {
      return Objects.hash(name, id, quantity);
    }

    @Override
    public String toString() {
      return "OrderProduct{"
          + "name='"
          + name
          + '\''
          + ", id='"
          + id
          + '\''
          + ", quantity="
          + quantity
          + '}';
    }
  }

  public static String readResource(String path) {
    try (InputStream is =
        Thread.currentThread().getContextClassLoader().getResourceAsStream(path)) {
      if (is == null) {
        throw new IllegalArgumentException("Resource not found: " + path);
      }
      return new String(is.readAllBytes(), StandardCharsets.UTF_8);
    } catch (IOException e) {
      throw new RuntimeException(e);
    }
  }
}
