package com.google.cloud.kafka.connect.bigtable;

import com.google.cloud.kafka.connect.bigtable.config.BigtableErrorMode;
import com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig;
import com.google.cloud.kafka.connect.bigtable.exception.BatchException;
import com.google.common.annotations.VisibleForTesting;
import java.util.concurrent.Future;
import org.apache.kafka.connect.sink.ErrantRecordReporter;
import org.apache.kafka.connect.sink.SinkRecord;
import org.apache.kafka.connect.sink.SinkTaskContext;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class RecordErrorHandler {

  private final Logger logger = LoggerFactory.getLogger(RecordErrorHandler.class);

  private final SinkTaskContext context;
  private final BigtableErrorMode errorMode;

  public RecordErrorHandler(SinkTaskContext context, BigtableErrorMode errorMode) {
    this.context = context;
    this.errorMode = errorMode;
  }

  public void handleDelayed(SinkRecord record, Future<Void> future) {
    // todo

    if (future.isDone()){
      // todo
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
