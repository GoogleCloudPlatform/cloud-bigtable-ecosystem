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
package com.google.cloud.aerospike;

import com.google.cloud.bigtable.data.v2.models.Mutation;
import com.google.cloud.bigtable.data.v2.models.RowMutation;
import com.google.cloud.bigtable.data.v2.models.TableId;
import com.google.common.annotations.VisibleForTesting;
import com.google.protobuf.ByteString;
import java.util.Optional;
import java.util.concurrent.TimeUnit;

/**
 * This class lets the user create a Bigtable {@link Mutation} that can be used to write Bigtable
 * cells equivalent to input Aerospike values.
 */
public class BigtableMutationBuilder extends RowBuilder<BigtableMutationBuilder.CellSetter> {
  private final TableId tableId;
  private final ByteString rowKey;

  public Mutation getMutation() {
    return cellSetter.mutation;
  }

  public RowMutation getRowMutation() {
    return RowMutation.create(tableId, rowKey, cellSetter.mutation);
  }

  /**
   * Constructs a {@link RowBuilder} containing an empty {@link Mutation}.
   *
   * @param tableId the ID of the table.
   * @param rowKey the key of the row.
   * @param timestamp the Bigtable timestamp to use. If `null`, the current system time is used.
   */
  public BigtableMutationBuilder(String tableId, ByteString rowKey, Long timestamp) {
    this(tableId, rowKey, Mutation.create(), timestamp);
  }

  /**
   * Constructs a {@link RowBuilder} by wrapping the provided {@link Mutation}.
   *
   * @param tableId the ID of the table.
   * @param rowKey the key of the row.
   * @param mutation the builder's initial value. All values will be added on top of it.
   * @param timestamp the Bigtable timestamp to use. If `null`, the current system time is used.
   */
  @VisibleForTesting
  BigtableMutationBuilder(String tableId, ByteString rowKey, Mutation mutation, Long timestamp) {
    super(
        new CellSetter(
            mutation,
            Optional.ofNullable(timestamp)
                .orElse(TimeUnit.MILLISECONDS.toMicros(System.currentTimeMillis()))));
    this.tableId = TableId.of(tableId);
    this.rowKey = rowKey;
  }

  /** A {@link RowBuilder.CellSetter} implementation that creates a Bigtable {@link Mutation}. */
  public static class CellSetter implements RowBuilder.CellSetter {
    private final Mutation mutation;
    private final long timestamp;

    public CellSetter(Mutation m, long ts) {
      mutation = m;
      this.timestamp = ts;
    }

    @Override
    public void setCell(String family, String qualifier, ByteString value) {
      mutation.setCell(family, ByteString.copyFromUtf8(qualifier), timestamp, value);
    }
  }
}
