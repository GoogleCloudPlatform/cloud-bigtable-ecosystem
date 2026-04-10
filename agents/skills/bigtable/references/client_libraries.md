# Bigtable Client Library User Guide

This document outlines critical technical details about Bigtable data model and client libraries.

## Timestamp Precision & Granularity

Bigtable stores timestamps as **64-bit integers** representing **microseconds** since the Unix epoch. However, Bigtable’s internal garbage collection and versioning operate at **millisecond granularity**.

> [!IMPORTANT]
> **Implementation Rule:** When generating code to store data, calculate the timestamp in milliseconds and multiply by 1,000.
>
> * **Correct:** `timestamp_micros = time_ms() * 1000`
> * **Incorrect:** Using raw microsecond precision (e.g., `time_micros()`), as this can lead to unexpected behavior with cell versioning and TTL.

## Replication & Atomic Operations

Bigtable’s replication model impacts the availability of certain "atomicity" features.

* **The Conflict:** **ReadModifyWrite** (increments/appends) and **CheckAndMutateRow** (conditional updates) require a single-point-of-truth to maintain consistency.
* **The Constraint:** These operations **will not work** with multi-cluster routing (App Profiles set to Multi-cluster).
* **Agent Action:** If a user’s code contains these methods, proactively warn them that they must use a **Single-cluster routing** App Profile or accept that these operations will fail in a multi-cluster configuration.
