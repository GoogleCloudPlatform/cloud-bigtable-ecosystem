// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package com.google.bigtable.cassandra.internal;

import com.sun.jna.Native;
import com.sun.jna.Library;

final class OsUtils {

  private OsUtils() {}

  static boolean isWindows() {
    return getOsName().contains("win");
  }

  static boolean isMac() {
    String osName = getOsName();
    return osName.contains("mac") || osName.contains("darwin") || osName.contains("osx");
  }

  static String getOsName() {
    return System.getProperty("os.name").trim().toLowerCase();
  }

  interface CLibrary extends Library {
    CLibrary INSTANCE = Native.load("c", CLibrary.class);

    int getpid();
    int getuid();
  }

  static String getPid() {
    if (isWindows()) {
      throw new UnsupportedOperationException("Windows is currently not supported");
    }
    return String.valueOf(CLibrary.INSTANCE.getpid());
  }

  static String getUid() {
    if (isWindows()) {
      throw new UnsupportedOperationException("Windows is currently not supported");
    }
    return String.valueOf(CLibrary.INSTANCE.getuid());
  }

}
