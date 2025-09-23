package com.google.cloud.kafka.connect.bigtable.utils;

import com.google.cloud.kafka.connect.bigtable.exception.BigtableSinkLogicError;
import com.google.common.annotations.VisibleForTesting;
import java.util.Collection;
import java.util.LinkedHashMap;
import java.util.Map;
import org.apache.kafka.connect.sink.SinkRecord;

public class Utils {
  /**
   * Generates a {@link Map} with desired key ordering.
   *
   * @param map A {@link Map} to be sorted.
   * @param order A {@link Collection} defining desired order of the output {@link Map}. Must be
   *     a superset of {@code mutations}'s key set.
   * @return A {@link Map} with the same keys and corresponding values as {@code map} with the same
   *     key ordering as {@code order}.
   */
  @VisibleForTesting
  // It is generic so that we can test it with naturally ordered values easily.
 public static <K, V> Map<K, V> orderMap(Map<K, V> map, Collection<K> order) {
    if (!order.containsAll(map.keySet())) {
      throw new BigtableSinkLogicError(
          "A collection defining order of keys must be a superset of the input map's key set.");
    }
    Map<K, V> sorted = new LinkedHashMap<>();
    for (K key : order) {
      V value = map.get(key);
      if (value != null) {
        sorted.put(key, value);
      }
    }
    return sorted;
  }

  /**
   * Get timestamp the input record's mutation's timestamp.
   *
   * @param record Input record.
   * @return UNIX timestamp in microseconds.
   */
  @VisibleForTesting
  public static long getTimestampMicros(SinkRecord record) {
    // From reading the Java Cloud Bigtable client, it looks that the only usable timestamp
    // granularity is the millisecond one. So we assume it.
    // There's a test that will break when it starts supporting microsecond granularity with a note
    // to modify this function then.
    Long timestampMillis = record.timestamp();
    if (timestampMillis == null) {
      // The timestamp might be null if the kafka cluster is old (<= v0.9):
      // https://github.com/apache/kafka/blob/f9615ed275c3856b73e5b6083049a8def9f59697/clients/src/main/java/org/apache/kafka/clients/consumer/ConsumerRecord.java#L49
      // In such a case, we default to wall clock time as per the design doc.
      timestampMillis = System.currentTimeMillis();
    }
    return 1000 * timestampMillis;
  }

}
