# Bigtable CLI Data Access

This document provides patterns for reading and writing data in Bigtable using the `cbt` CLI. This is primarily used for debugging and quick data validation.

## Configuring cbt for Data Access

```bash
echo project = ${BIGTABLE_PROJECT} > ~/.cbtrc
echo instance = ${BIGTABLE_INSTANCE} >> ~/.cbtrc
```

## Reading Data

### Read Single Row (Lookup)
Reads all columns and versions for a specific row.
```bash
cbt lookup [TABLE_NAME] [ROW_KEY]
```
*Note: `cbt lookup` is optimized for point reads and is significantly more efficient than using `cbt read` with a count or filter for retrieving a single known row.*


### Read N Rows
Reads the first `N` rows from the table.
```bash
cbt read [TABLE_NAME] count=[N]
```

### Read Range
Reads rows between `START_KEY` (inclusive) and `END_KEY` (exclusive).
```bash
cbt read [TABLE_NAME] start=[START_KEY] end=[END_KEY]
```

### Read using SQL
For complex queries and aggregations use SQL via the `cbt sql` command
```bash
cbt sql "SELECT * FROM my_table WHERE _key = 'user#123'"
```

### Row Count (Estimate)
Provides an estimate of the number of rows in the table.
```bash
gcloud bigtable instances tables describe TABLE_ID  --instance=INSTANCE_ID --view stats
```

**Note**: cbt count [TABLE_NAME] would do a full table scan.

## Writing Data

### Write Cell (Set)
Writes a value to a specific cell (row, family, and column).
```bash
cbt set [TABLE_NAME] [ROW_KEY] [FAMILY]:[COLUMN]=[VALUE]
```
*Example:* `cbt set my-table user123 profile:email=user@example.com`

## Deleting Data

### Delete Row
```bash
cbt deleterow [TABLE_NAME] [ROW_KEY]
```
