# How to Run

1. Create a Bigtable Instance called `bigtabledevinstance`.
2. Start the Proxy with a listener on port `9042` with a `bigtabledevinstance`
   keyspace configured.
3. Before running the tests, create the required tables in your Bigtable instance using `cqlsh`:

```sql
CREATE TABLE IF NOT EXISTS bigtabledevinstance.fuzztestkeys (
   id varchar,
   str_key varchar,
   int_key int,
   bigint_key bigint,
   name varchar,
   PRIMARY KEY (id, str_key, int_key, bigint_key)
);

CREATE TABLE IF NOT EXISTS bigtabledevinstance.fuzztestcolumns (
   id text PRIMARY KEY,
   name text,
   code int,
   credited double,
   balance float,
   is_active boolean,
   birth_date timestamp,
   zip_code bigint,
   extra_info map<text, text>,
   map_text_int map<text, int>,
   map_text_bigint map<text, bigint>,
   map_text_boolean map<text, boolean>,
   map_text_ts map<text, timestamp>,
   map_text_float map<text, float>,
   map_text_double map<text, double>,
   ts_text_map map<timestamp, text>,
   ts_boolean_map map<timestamp, boolean>,
   ts_float_map map<timestamp, float>,
   ts_double_map map<timestamp, double>,
   ts_bigint_map map<timestamp, bigint>,
   ts_ts_map map<timestamp, timestamp>,
   ts_int_map map<timestamp, int>,
   tags set<text>,
   set_boolean set<boolean>,
   set_int set<int>,
   set_bigint set<bigint>,
   set_float set<float>,
   set_double set<double>,
   set_timestamp set<timestamp>,
   list_text list<text>,
   list_int list<int>,
   list_bigint list<bigint>,
   list_float list<float>,
   list_double list<double>,
   list_boolean list<boolean>,
   list_timestamp list<timestamp>
);
```

4. Run the tests using typical **fuzz** testing commands like `go test -test.fuzz FuzzRowKeys -test.run ^$`.

Note: If schema changes are made to any test tables they will need to be
manually deleted and recreated.
