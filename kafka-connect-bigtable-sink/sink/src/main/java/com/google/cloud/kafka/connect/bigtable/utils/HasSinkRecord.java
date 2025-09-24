package com.google.cloud.kafka.connect.bigtable.utils;

import org.apache.kafka.connect.sink.SinkRecord;

public interface HasSinkRecord {

  SinkRecord getRecord();
}
