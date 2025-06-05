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

import com.datastax.oss.driver.api.core.CqlSession;
import com.datastax.oss.driver.api.core.metadata.Node;
import com.datastax.oss.driver.api.core.metadata.NodeState;
import com.datastax.oss.driver.api.core.metadata.NodeStateListenerBase;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;


/**
 * Internal use only
 */
class BigtableCqlSessionNodeStateListener extends NodeStateListenerBase {

  private final AtomicReference<CqlSession> cqlSessionReference = new AtomicReference<>();
  private final AtomicInteger upNodes = new AtomicInteger(0);

  private static final Logger LOGGER = LoggerFactory.getLogger(
      BigtableCqlSessionNodeStateListener.class);

  protected BigtableCqlSessionNodeStateListener() {}

  void initialize(CqlSession session) {
    cqlSessionReference.set(session);
    int initialUpNodes = 0;
    for (Node node : cqlSessionReference.get().getMetadata().getNodes().values()) {
      if (node.getState() == NodeState.UP) {
        initialUpNodes++;
      }
    }
    upNodes.set(initialUpNodes);
  }

  @Override
  public void onUp(Node node) {
    upNodes.incrementAndGet();
  }

  @Override
  public void onDown(Node node) {
    int currentUp = upNodes.decrementAndGet();
    if (currentUp <= 0) {
      LOGGER.error("All nodes are down. Closing the CqlSession.");
      if (!cqlSessionReference.get().isClosed()) {
        cqlSessionReference.get().forceCloseAsync();
      }
    }
  }

  @Override
  public void onRemove(Node node) {
    if (node.getState() == NodeState.UP) {
      onDown(node);
    }
  }
}
