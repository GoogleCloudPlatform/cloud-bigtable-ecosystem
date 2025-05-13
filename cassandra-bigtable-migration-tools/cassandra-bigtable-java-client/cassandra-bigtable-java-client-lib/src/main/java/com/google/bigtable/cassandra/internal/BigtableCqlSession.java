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
import com.datastax.oss.driver.api.core.context.DriverContext;
import com.datastax.oss.driver.api.core.cql.AsyncResultSet;
import com.datastax.oss.driver.api.core.cql.PreparedStatement;
import com.datastax.oss.driver.api.core.cql.SimpleStatement;
import com.datastax.oss.driver.api.core.cql.Statement;
import com.datastax.oss.driver.api.core.metadata.Metadata;
import com.datastax.oss.driver.api.core.metrics.Metrics;
import com.datastax.oss.driver.api.core.session.Request;
import com.datastax.oss.driver.api.core.type.reflect.GenericType;
import com.datastax.oss.driver.api.core.CqlIdentifier;

import java.util.Optional;
import java.util.concurrent.CompletionStage;

class BigtableCqlSession implements CqlSession {

  private final CqlSession delegate;
  private final Proxy proxy;

  protected BigtableCqlSession(CqlSession delegate, Proxy proxy) {
    this.delegate = delegate;
    this.proxy = proxy;
  }

  @Override
  public String getName() {
    return delegate.getName();
  }

  @Override
  public Metadata getMetadata() {
    return delegate.getMetadata();
  }

  @Override
  public boolean isSchemaMetadataEnabled() {
    return delegate.isSchemaMetadataEnabled();
  }

  @Override
  public CompletionStage<Metadata> setSchemaMetadataEnabled(Boolean aBoolean) {
    return delegate.setSchemaMetadataEnabled(aBoolean);
  }

  @Override
  public CompletionStage<Metadata> refreshSchemaAsync() {
    return delegate.refreshSchemaAsync();
  }

  @Override
  public CompletionStage<Boolean> checkSchemaAgreementAsync() {
    return delegate.checkSchemaAgreementAsync();
  }

  @Override
  public DriverContext getContext() {
    return delegate.getContext();
  }

  @Override
  public Optional<CqlIdentifier> getKeyspace() {
    return delegate.getKeyspace();
  }

  @Override
  public Optional<Metrics> getMetrics() {
    return delegate.getMetrics();
  }

  @Override
  public <RequestT extends Request, ResultT> ResultT execute(RequestT request,
      GenericType<ResultT> genericType) {
    return delegate.execute(request, genericType);
  }

  @Override
  public PreparedStatement prepare(String query) {
    return delegate.prepare(query);
  }

  @Override
  public PreparedStatement prepare(SimpleStatement statement) {
    return delegate.prepare(statement);
  }

  @Override
  public CompletionStage<PreparedStatement> prepareAsync(String query) {
    return delegate.prepareAsync(query);
  }

  @Override
  public CompletionStage<PreparedStatement> prepareAsync(SimpleStatement statement) {
    return delegate.prepareAsync(statement);
  }

  @Override
  public CompletionStage<AsyncResultSet> executeAsync(String query) {
    return delegate.executeAsync(SimpleStatement.newInstance(query));
  }

  @Override
  public CompletionStage<AsyncResultSet> executeAsync(Statement<?> statement) {
    return delegate.executeAsync(statement);
  }

  @Override
  public CompletionStage<Void> closeFuture() {
    return delegate.closeFuture();
  }

  @Override
  public CompletionStage<Void> closeAsync() {
    return delegate.closeAsync();
  }

  @Override
  public CompletionStage<Void> forceCloseAsync() {
    return delegate.forceCloseAsync();
  }

  @Override
  public void close() {
    try {
      delegate.close();
    } finally {
      proxy.stop();
    }
  }

}
