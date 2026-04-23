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

import java.io.IOException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class TestForLeaks {
  private static final Logger LOG = LoggerFactory.getLogger(TestForLeaks.class);

  public static void main(String[] args) throws IOException, InterruptedException {
    if (args.length != 1) {
      throw new IllegalArgumentException("USAGE: <java> FILE_PATH");
    }

    try (BackupReader br = new BackupReader(args[0], BackupReader.CompressionAlgorithm.NONE)) {
      for (long i = 0; true; i += 1) {
        if (i % 10000 == 0) {
          LOG.info("Progress: {}", i);
        }

        try {
          ReadRecordResult result = br.readRecord("test");
          if (result == null) {
            LOG.info("Successful end of reading!");
            break;
          }
        } catch (Throwable t) {
          LOG.debug("Error while reading.", t);
        }
      }
    } catch (Throwable t) {
      LOG.error("Unexpected exception caught.", t);
    }
  }
}
