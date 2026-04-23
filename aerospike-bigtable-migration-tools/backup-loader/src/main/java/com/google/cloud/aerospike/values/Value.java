/*
 * Copyright 2025 Google LLC
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
package com.google.cloud.aerospike.values;

import com.google.cloud.aerospike.RowBuilder;
import com.squareup.moshi.JsonAdapter;
import com.squareup.moshi.JsonReader;
import com.squareup.moshi.JsonWriter;
import com.squareup.moshi.Moshi;
import java.io.IOException;
import java.io.Serializable;
import java.util.Base64;
import java.util.Optional;

/** Represents a value read from an Aerospike backup. */
public abstract sealed class Value implements Serializable
    permits BooleanValue, BytesValue, FloatValue, IntegerValue, ListValue, MapValue, StringValue {
  private static final Moshi MOSHI =
      new Moshi.Builder()
          .add(BooleanValue.class, new BooleanValue.MoshiJsonAdapter())
          .add(BytesValue.class, new BytesValue.MoshiJsonAdapter())
          .add(FloatValue.class, new FloatValue.MoshiJsonAdapter())
          .add(IntegerValue.class, new IntegerValue.MoshiJsonAdapter())
          .add(ListValue.class, new ListValue.MoshiJsonAdapter())
          .add(MapValue.class, new MapValue.MoshiJsonAdapter())
          .add(StringValue.class, new StringValue.MoshiJsonAdapter())
          .add(Double.class, new DoubleStringifier())
          .add(byte[].class, new ByteArrayBase64Encoder())
          .build();

  public static <T> JsonAdapter<T> getJsonAdapter(Class<T> klass) {
    return MOSHI.adapter(klass);
  }

  /**
   * Sets the value of a specific bin in an Aerospike row mutation.
   *
   * @param binName the name of the bin
   * @param rowBuilder the {@link RowBuilder} to modify
   */
  public abstract void setCell(String binName, RowBuilder<?> rowBuilder);

  private static class DoubleStringifier extends JsonAdapter<Double> {
    @Override
    public Double fromJson(JsonReader jsonReader) throws IOException {
      String value = jsonReader.nextString();
      return Optional.ofNullable(value).map(Double::parseDouble).orElse(null);
    }

    @Override
    public void toJson(JsonWriter jsonWriter, Double value) throws IOException {
      Value.getJsonAdapter(String.class)
          .toJson(jsonWriter, Optional.ofNullable(value).map(v -> Double.toString(v)).orElse(null));
    }
  }

  private static class ByteArrayBase64Encoder extends JsonAdapter<byte[]> {
    @Override
    public byte[] fromJson(JsonReader jsonReader) throws IOException {
      String value = jsonReader.nextString();
      return Optional.ofNullable(value).map(s -> Base64.getDecoder().decode(s)).orElse(null);
    }

    @Override
    public void toJson(JsonWriter jsonWriter, byte[] value) throws IOException {
      Value.getJsonAdapter(String.class)
          .toJson(
              jsonWriter,
              Optional.ofNullable(value)
                  .map(v -> Base64.getEncoder().encodeToString(v))
                  .orElse(null));
    }
  }
}
