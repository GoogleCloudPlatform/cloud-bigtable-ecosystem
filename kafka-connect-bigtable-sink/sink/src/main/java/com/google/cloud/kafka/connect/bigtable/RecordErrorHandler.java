package com.google.cloud.kafka.connect.bigtable;

import com.google.cloud.kafka.connect.bigtable.config.BigtableErrorMode;
import com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig;
import com.google.cloud.kafka.connect.bigtable.exception.BatchException;
import com.google.cloud.kafka.connect.bigtable.mapping.MutationData;
import com.google.cloud.kafka.connect.bigtable.utils.HasSinkRecord;
import com.google.cloud.kafka.connect.bigtable.utils.SinkResult;
import com.google.common.annotations.VisibleForTesting;
import java.util.Collection;
import java.util.Optional;
import java.util.concurrent.CompletableFuture;
import org.apache.kafka.connect.sink.ErrantRecordReporter;
import org.apache.kafka.connect.sink.SinkRecord;
import org.apache.kafka.connect.sink.SinkTaskContext;
import org.slf4j.Logger;

public class RecordErrorHandler {

  private final Logger logger;
  private final SinkTaskContext context;
  private final BigtableErrorMode errorMode;

  public RecordErrorHandler(Logger logger, SinkTaskContext context, BigtableErrorMode errorMode) {
    this.logger = logger;
    this.context = context;
    this.errorMode = errorMode;
  }

  public void handleDelayed(SinkRecord sinkRecord, CompletableFuture<?> future) {
    future.whenCompleteAsync((result, e) -> {
      // todo confirm this will interrupt the task thread
      if (e != null) {
        reportError(sinkRecord, e);
      }
    });
  }

  public <V> Optional<V> processTry(SinkResult<V> value) {
    if (value.isSuccess()) {
      return Optional.of(value.getValue());
    } else {
      // may throw an exception if the error mode is set to FAIL
      reportError(value.getRecord(), value.getThrowable());
      return Optional.empty();
    }
  }

  public <V> void reportErrors(Collection<SinkResult<V>> records) {
    for (SinkResult<?> record : records) {
      if (!record.isSuccess()) {
        reportError(record.getRecord(), record.getThrowable());
      }
    }
  }

  /**
   * Report error as described in {@link BigtableSinkConfig#getDefinition()}.
   *
   * @param record Input record whose processing caused an error.
   * @param throwable The error.
   */
  @VisibleForTesting
  public void reportError(SinkRecord record, Throwable throwable) {
    ErrantRecordReporter reporter;
    /// We get a reference to `reporter` using a procedure described in javadoc of
    /// {@link SinkTaskContext#errantRecordReporter()} that guards against old Kafka versions.
    try {
      reporter = context.errantRecordReporter();
    } catch (NoSuchMethodError | NoClassDefFoundError ignored) {
      reporter = null;
    }
    if (reporter != null) {
      reporter.report(record, throwable);
      logger.warn(
          "Used DLQ for reporting a problem with a record (throwableClass={}).",
          throwable.getClass().getName());
    } else {
      switch (errorMode) {
        case IGNORE:
          break;
        case WARN:
          logger.warn("Processing of a record with key {} failed", record.key(), throwable);
          break;
        case FAIL:
          throw new BatchException(throwable);
      }
    }
  }
}
