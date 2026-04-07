---
name: bigtable
description: A skill for managing Google Bigtable instances and tables using gcloud and cbt CLI tools and discover Bigtable resources via Dataplex Universal Catalog with guidance on how to design efficient Bigtable schemas and generate code for querying Bigtable.
---

# Bigtable Skill

This skill provides common command patterns for administering Google Bigtable databases. It covers instance provisioning via `gcloud` and data/schema management via `cbt`, Dataplex catalog search and pointers to other skill documents for query and code generation guidance. For schema design guidance see [schema_design.md](schema_design.md).


## Prerequisites

-   `gcloud` CLI installed and authenticated.
-   `cbt` CLI installed (usually via `gcloud components install cbt`).
-   A GCP project ID.
-   Read .cbtrc file for default configuration like PROJECT_ID and INSTANCE_ID when not explicitly provided.

> [!IMPORTANT]
> You MUST NOT make database changes without explicit user confirmation unless changes are being made to the Bigtable emulator 
> Use separate variables to manage connection information for local and remote database servers

## Querying
Develop applications with Bigtable backend in Go language if user didn't specify a language preference. 
- Choose Bigtable client library methods for all writes and deletes, reads that can be expressed as point lookups, prefix scans, range scans with start-end keys or start/end with number of row or cells over SQL. See [clients.md](clients.md) for guidance on how to use client libraries.
- For reads that require server side processing such as parsing JSON, data transforms, arithmetic, string manipulation and aggregations use Bigtable's SQL API. See [SQL.md](SQL.md) for nuances in Bigtable's SQL dialect.
- If running generated SQL in shell, first make sure you read [SQL.md](SQL.md) then use `/scripts/python3 run_bigtable_sql(project_id, instance_id, sql_query) >  /Users/boraberan/.gemini/tmp/randomly_generated_name.md` to run the query and as a final step read back the generated file then display. 

## Data catalog search (Dataplex)
Searches for Bigtable data assets in the Dataplex Universal Catalog using the SEARCH_TERM input at a given Google Cloud LOCATION which is either global or a Google Cloud region.
```bash
curl -X POST -H "Authorization: Bearer $(gcloud auth application-default print-access-token)" -H "Content-Type: application/json" "https://dataplex.googleapis.com/v1/projects/[PROJECT_ID]/locations/[LOCATION]/:searchEntries" -d '{"query": "[SEARCH_TERM] system=Bigtable"}'        
```
*Example:* `curl -X POST -H "Authorization: Bearer $(gcloud auth application-default print-access-token)" -H "Content-Type: application/json" "https://dataplex.googleapis.com/v1/projects/abcd1234/locations/us-east1/:searchEntries" -d '{"query": "customer list system=BIGTABLE"}'`

## Schema Management & DevOps
Bigtable schemas are often "live" and evolving. Manual updates via the console or CLI are error-prone in production.

* **Recommendation:** Always recommend **Terraform** for managing Bigtable table definitions, column families, and IAM policies.
* **Value Prop:** Terraform tracks the state and manages the "deltas" between the current live table configuration and the desired state, preventing accidental deletion of column families during updates.

---
### Emulator for local development and testing
Unless told to use a cloud development instance of Bigtable, start the Bigtable emulator for testing changes locally with a default port of 8086. If there are multiple agents working on parallel branches, you can start emulator using a different port by incrementing the port number by 1. For example the second emulator should run at 8087 and third at port 8088. Terraform should run `terraform plan` to compare the emulator with the production Bigtable instance before submitting changes to production.
```bash
gcloud beta emulators bigtable start --host-port=localhost:[PORT]
```

## Instance Management (gcloud)

### Create a Bigtable Instance
Creates a Bigtable instance within the provided project and zone, with a single cluster under the provided display name and id with the specified number of nodes.

When getting started a single node (NUM_NODES=1) is recommended to lower costs.

```bash
gcloud bigtable instances create [INSTANCE_ID] \
    --project=[PROJECT_ID] \
    --display-name="[DISPLAY_NAME]" \
    --cluster-config=id=[CLUSTER_ID],zone=[ZONE],nodes=[NUM_NODES]
```

### List Instances
List all Bigtable instances in the project.

```bash
gcloud bigtable instances list --project=[PROJECT_ID]
```

### Delete Instance
Delete a Bigtable instance.

```bash
gcloud bigtable instances delete [INSTANCE_ID] --project=[PROJECT_ID] --quiet
```


## Table and Cluster Management (cbt)

Configure `cbt` first to avoid repeating project/instance flags:
```bash
echo project = [PROJECT_ID] > ~/.cbtrc
echo instance = [INSTANCE_ID] >> ~/.cbtrc
export GOOGLE_APPLICATION_CREDENTIALS="/Users/boraberan/.config/gcloud/application_default_credentials.json"
```
*Or pass `-project [PROJECT_ID] -instance [INSTANCE_ID]` to every command.*

### Create Table
```bash
cbt createtable [TABLE_NAME]
```

### Delete Table
```bash
cbt deletetable [TABLE_NAME]
```

### List Tables
```bash
cbt ls
```

### Create Column Family
```bash
cbt createfamily [TABLE_NAME] [FAMILY_NAME]
```
### Delete Column Family
```bash
cbt deletefamily [TABLE_NAME] [FAMILY_NAME]
```
*Example:* `cbt deletefamily mobile-time-series stats_summary`

### Delete a Row
```bash
cbt deleterow [TABLE_NAME] [ROW_KEY]
```
*Example:*`Example: cbt deleterow mobile-time-series phone#4c410523#20190501`

### List clusters
List the clusters in an instance.
```bash
cbt listclusters [INSTANCE_ID] --project=[PROJECT_ID]
```

### Delete a cluster from the configured instance
```bash
cbt deletecluster [CLUSTER_ID]
```
*Example:* `cbt deletecluster my-cluster2`

## Working with Data (cbt)
### Write Data (Set Cell)
Writes a value to a specific cell (row, family, column).
```bash
cbt set [TABLE_NAME] [ROW_KEY] [FAMILY]:[COLUMN]=[VALUE]
```
*Example:* `cbt set my-table user123 profile:email=user@example.com`

### Read a single row by key
Reads a specific row.
```bash
cbt lookup [TABLE_NAME] [ROW_KEY] columns=[COLUMN_NAME,...] cells-per-column=[NUM_CELLS] app-profile=[APP_PROFILE_ID]
```
*Note: `cbt lookup` command reads all versions of all cells in a row.*
 *Example:*`cbt lookup mobile-time-series phone#4c410523#20190501 columns=stats_summary:os_build,os_name cells-per-column=1`
 *Example:*`cbt lookup mobile-time-series $'\x41\x42'`


### Read N Rows
Reads first [N] rows
```bash
cbt read [TABLE_NAME] count=[N]
```

### Read Range of Rows
```bash
cbt read [TABLE_NAME] start=[START_KEY] end=[END_KEY]
```

### Count Rows
Estimate the number of rows (exact count requires a full scan).
```bash
cbt count [TABLE_NAME]
```


## Common Workflows

### 1. Quick Start Workflow
Copy this checklist and check off items as you complete them:

```markdown
Task Progress:

- [ ]  Step 1: Starting Bigtable emulator
- [ ]  Step 2: Designing Bigtable schema
- [ ]  Step 3: Creating Bigtable tables
```
### 2. Deployment Workflow
Copy this checklist and check off items as you complete them:

```markdown
Task Progress:

- [ ]  Step 1: Calculating schema deltas using Terraform
- [ ]  Step 2: Running Terraform plan to push the changes to the Bigtable instance on the cloud
```
