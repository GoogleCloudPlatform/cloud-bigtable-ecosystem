# Bigtable Infrastructure and Administration

This document provides patterns for provisioning and managing Bigtable resources using `gcloud` and `cbt`.

## Instance and Cluster Management (gcloud)

### Create/Delete Instance
```bash
# Create instance with a single cluster
gcloud bigtable instances create [INSTANCE_ID] \
    --project=[PROJECT_ID] \
    --display-name="[DISPLAY_NAME]" \
    --cluster-config=id=[CLUSTER_ID],zone=[ZONE],nodes=[NUM_NODES]

# Delete instance
gcloud bigtable instances delete [INSTANCE_ID] --project=[PROJECT_ID] --quiet
```

### Cluster Operations
```bash
# List clusters in an instance
cbt listclusters [INSTANCE_ID] --project=[PROJECT_ID]

# Delete a cluster
cbt deletecluster [CLUSTER_ID]
```

## Table and Schema Management (cbt)

### Table Operations
```bash
# Create/Delete table
cbt createtable [TABLE_NAME]
cbt deletetable [TABLE_NAME]

# List tables
cbt ls
```

### Column Family Operations
```bash
# Create/Delete column family
cbt createfamily [TABLE_NAME] [FAMILY_NAME]
cbt deletefamily [TABLE_NAME] [FAMILY_NAME]
```

## Local Development (Emulator)

Start the Bigtable emulator for testing changes locally:
```bash
gcloud beta emulators bigtable start --host-port=localhost:[PORT]
```
*Default port: 8086.*
