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
import java.io.IOException;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import java.util.StringJoiner;

/** Represents a map value read from an Aerospike backup. */
public final class MapValue extends Value {
  private final Map<Object, Object> map;

  /** Constructs a MapValue from a map. */
  public MapValue(Map<Object, Object> map) {
    Objects.requireNonNull(map, this.getClass().getName() + "'s value must not be null.");
    this.map = map;
  }

  /**
   * Returns the underlying map of this MapValue.
   *
   * @return the map of key-value pairs.
   */
  public Map<Object, Object> getMap() {
    return map;
  }

  /**
   * Returns a string representation of the map.
   *
   * @return a string describing the map contents.
   */
  @Override
  public String toString() {
    StringJoiner joiner = new StringJoiner(", ", "MapValue{", "}");
    for (Map.Entry<Object, Object> entry : map.entrySet()) {
      joiner.add(entry.getKey() + "->" + entry.getValue());
    }
    return joiner.toString();
  }

  /**
   * Sets a cell in Aerospike. Not implemented for MapValue yet.
   *
   * @param binName the name of the bin.
   * @param rowBuilder the {@link RowBuilder} to modify
   */
  @Override
  public void setCell(String binName, RowBuilder<?> rowBuilder) {
    rowBuilder.addValue(binName, this.map);
  }

  /** A {@link JsonAdapter} for (de)serializing {@link MapValue} as JSON object. */
  static class MoshiJsonAdapter extends JsonAdapter<MapValue> {
    @Override
    public MapValue fromJson(JsonReader jsonReader) {
      throw new UnsupportedOperationException();
    }

    @Override
    public void toJson(JsonWriter jsonWriter, MapValue value) throws IOException {
      Value.getJsonAdapter(Map.class)
          .toJson(jsonWriter, Optional.ofNullable(value).map(v -> v.map).orElse(null));
    }
  }
}
