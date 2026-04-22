# Copyright 2025 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

ARG BEAM_VERSION="MUST_BE_SET_FROM_COMMAND_LINE"
# ----------- Stage 1: builder -----------
FROM ubuntu:22.04 AS builder

ENV DEBIAN_FRONTEND=noninteractive

# Update and install dependencies
RUN apt-get update -y && apt-get install -y --no-install-recommends \
    build-essential \
    libssl-dev \
    libzstd-dev \
    libpsl-dev \
    cmake \
    pkg-config \
    zlib1g-dev \
    autotools-dev \
    automake \
    libtool \
    git \
    openjdk-17-jdk \
    maven \
    curl \
    clang-format \
    clang-tidy \
    libuv1-dev \
    libcurl4-openssl-dev \
    libjansson-dev

WORKDIR /build

# install just
RUN mkdir just
RUN curl -sSL https://just.systems/install.sh | bash -s -- --to /build/just
ENV PATH="/build/just:$PATH"

# Build AWS SDK for C++
RUN git clone https://github.com/aws/aws-sdk-cpp.git --branch 1.11.210 --depth 1 --recurse-submodules -j"$(nproc)" --shallow-submodules
RUN cd aws-sdk-cpp && \
    mkdir build && \
    cmake -S . -B build \
        -DCMAKE_BUILD_TYPE=Release \
        -DBUILD_ONLY="s3" \
        -DBUILD_SHARED_LIBS=ON \
        -DENABLE_TESTING=OFF \
        -DCMAKE_INSTALL_PREFIX=/usr/local \
        -DCMAKE_INSTALL_LIBDIR=lib \
        -DENABLE_UNITY_BUILD=ON && \
    make -j$(nproc) -C build && make install -C build

# Clone and build Aerospike backup tools
RUN git clone https://github.com/aerospike/aerospike-tools-backup.git /app --branch 4.5.1 --depth 1 --recurse-submodules -j"$(nproc)" --shallow-submodules

WORKDIR /app

# Here, we need to work around Aerospike backup and restore utilities utilizing a logging function named `err`.
# Contrary to the err() from the standard C library, Aerospike's version doesn't exit the process.
# It works fine when running the tools on their own, but when this library is loaded into the JVM process,
# the symbol is already resolved to the C standard library's implementation. This effectively causes the
# Aerospike client to exit the process as soon as it encounters any error (it would have continued
# execution if Aerospike's version were used).
#
# We do it using the C preprocessor by defining macro that replaces every `err` with `aerospike_backup_err_hack__`.
# Unfortunately, the project's Makefile disallows overriding C++ compilation flags, so we need to include
# it in a Makefile we control and override the variables here.
RUN CPPFLAGS="-g -Derr=aerospike_backup_err_func__" \
    EVENT_LIB=libuv \
    CC="gcc" \
    CXX="g++" \
    make -f - shared all -j$(nproc) <<EOF
include Makefile
CFLAGS += \$(CPPFLAGS)
CXXFLAGS += \$(CPPFLAGS)
EOF

# For safety, let's check that the symbol substitution works.
RUN bash <<EOF
set -e
readelf --demangle --wide --symbols bin/asrestore.so  | tr --squeeze-repeats '[:blank:]' ' ' | cut --delimiter=' ' --fields=9- | grep --silent '^aerospike_backup_err_func__$'
! readelf --demangle --wide --symbols bin/asrestore.so  | tr --squeeze-repeats '[:blank:]' ' ' | cut --delimiter=' ' --fields=9- | grep --silent '^err$'
EOF

ENV LD_LIBRARY_PATH=/usr/local/lib

# ----------- Stage 2: compiled -----------
FROM builder AS compiled

# copy the source and build the library
COPY adapter /src/adapter
COPY Justfile /src/Justfile
COPY backup-loader /src/backup-loader
COPY replicator /src/replicator
COPY pom.xml /src/pom.xml
RUN cd /src && \
    just build

# ----------- Stage 3: dataflow-worker -----------
FROM gcr.io/cloud-dataflow/v1beta3/beam_java17_sdk:${BEAM_VERSION} AS dataflow-worker

# Copy required libraries from builder
COPY --from=builder /usr/local/lib/libaws-cpp-sdk-s3.so /usr/local/lib/
COPY --from=builder /usr/local/lib/libaws-cpp-sdk-core.so /usr/local/lib/
COPY --from=builder /app/bin/asrestore.so /usr/local/lib/libasrestore.so
COPY --from=compiled /usr/lib/libbackupreader.so /usr/local/lib/

# Add Google Cloud SDK repository
RUN echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | \
    tee -a /etc/apt/sources.list.d/google-cloud-sdk.list

# Import the Google Cloud public key
RUN curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | \
    apt-key --keyring /usr/share/keyrings/cloud.google.gpg add -

# Install Google Cloud SDK and asrestore dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      google-cloud-cli \
      libssl3 \
      libuv1 \
      libcurl4 \
      libzstd1 \
      libjansson4 && \
    rm -rf /var/lib/apt/lists/*

ENV LD_LIBRARY_PATH="/usr/local/lib:${LD_LIBRARY_PATH}"
