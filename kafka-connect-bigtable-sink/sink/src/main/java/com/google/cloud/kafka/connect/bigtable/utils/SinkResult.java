package com.google.cloud.kafka.connect.bigtable.utils;

import javax.validation.constraints.NotNull;
import org.apache.kafka.connect.sink.SinkRecord;

public interface SinkResult<V> extends HasSinkRecord {

  public static <T extends HasSinkRecord> SuccessfulSinkResult<T> success(@NotNull T value) {
    return success(value.getRecord(), value);
  }

  public static <T> SuccessfulSinkResult<T> success(SinkRecord sinkRecord, @NotNull T value) {
    return new SuccessfulSinkResult<>(sinkRecord, value);
  }

  public static <T> FailureSinkResult<T> failure(SinkRecord sinkRecord, Throwable throwable) {
    return new FailureSinkResult<>(sinkRecord, throwable);
  }

  SinkRecord getRecord();

  V getValue();

  Throwable getThrowable();

  boolean isSuccess();

  public class SuccessfulSinkResult<V> implements SinkResult<V> {

    private final SinkRecord sinkRecord;
    private final V value;

    SuccessfulSinkResult(SinkRecord sinkRecord, V value) {
      this.sinkRecord = sinkRecord;
      this.value = value;
    }

    public SinkRecord getRecord() {
      return sinkRecord;
    }

    public V getValue() {
      return value;
    }

    public Throwable getThrowable() {
      return null;
    }

    public boolean isSuccess() {
      return true;
    }
  }

  public class FailureSinkResult<V> implements SinkResult<V> {

    private final SinkRecord sinkRecord;
    private final Throwable throwable;

    FailureSinkResult(SinkRecord sinkRecord, Throwable throwable) {
      this.sinkRecord = sinkRecord;
      this.throwable = throwable;
    }

    public SinkRecord getRecord() {
      return sinkRecord;
    }

    public V getValue() {
      return null;
    }

    public Throwable getThrowable() {
      return throwable;
    }

    public boolean isSuccess() {
      return false;
    }
  }
}


