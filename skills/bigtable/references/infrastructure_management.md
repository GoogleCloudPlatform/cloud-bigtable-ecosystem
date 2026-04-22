# Bigtable Infrastructure and Administration

This document provides patterns for provisioning and managing Bigtable resources.

## Tooling Split

- **`gcloud` (Control Plane):** Use for instances, clusters, app profiles, backups, and IAM.
- **`cbt` (Data Plane):** Use for tables, column families, and data manipulation.

## Control Plane (gcloud)

### Instance and Cluster Management
```bash
# Create instance with a single cluster
gcloud bigtable instances create ${BIGTABLE_INSTANCE} \
    --project=${BIGTABLE_PROJECT} \
    --display-name="[DISPLAY_NAME]" \
    --cluster-config=id=${BIGTABLE_CLUSTER},zone=[ZONE],nodes=[NUM_NODES]

# Add a cluster to an existing instance
gcloud bigtable clusters create ${BIGTABLE_CLUSTER} \
    --instance=${BIGTABLE_INSTANCE} \
    --zone=[ZONE] \
    --nodes=[NUM_NODES]

# Delete instance
gcloud bigtable instances delete ${BIGTABLE_INSTANCE} --project=${BIGTABLE_PROJECT} --quiet
```

### Backup and Restore
```bash
# Create a backup
gcloud bigtable backups create [BACKUP_ID] \
    --instance=${BIGTABLE_INSTANCE} \
    --cluster=${BIGTABLE_CLUSTER} \
    --table=[TABLE_ID] \
    --retention-period=7d

# Restore a table from backup
gcloud bigtable instances tables restore \
    --source=projects/[PROJECT_ID_SOURCE]/instances/[INSTANCE_ID_SOURCE]/clusters/${BIGTABLE_CLUSTER}/backups/[BACKUP_ID] \
    --destination=[NEW_TABLE_ID] \
    --destination-instance=[INSTANCE_ID_DESTINATION] \
    --project=[PROJECT_ID_DESTINATION] \
    --async
```

## Data Plane (cbt)

### Table and Schema Operations
```bash
# Create/Delete table
cbt createtable [TABLE_NAME]
cbt deletetable [TABLE_NAME]

# List tables and families
cbt ls
cbt ls [TABLE_NAME]

# Create/Delete column family
cbt createfamily [TABLE_NAME] [FAMILY_NAME]
cbt setgcpolicy [TABLE_NAME] [FAMILY_NAME] "maxversions=1"
cbt deletefamily [TABLE_NAME] [FAMILY_NAME]
```

## Observability and Performance

### Hotspotting Diagnosis
When performance degrades or a "hotspot" is suspected:
1. **Key Visualizer:** Direct the user to the Google Cloud Console. Key Visualizer provides a heatmap of access patterns across row keys.
2. **List Hot Tablets (gcloud):** Identify specific tablets with high CPU usage.
   ```bash
   gcloud bigtable hot-tablets list ${BIGTABLE_CLUSTER} --instance=${BIGTABLE_INSTANCE}
   ```

## Local Development (Emulator)

Start the Bigtable emulator for testing:
```bash
gcloud beta emulators bigtable start --host-port=localhost:8086
```
To point `cbt` or client libraries to the emulator:
```bash
export BIGTABLE_EMULATOR_HOST=localhost:8086
```

** Note**: Bigtable emulator doesn't support Bigtable GoogleSQL yet.
