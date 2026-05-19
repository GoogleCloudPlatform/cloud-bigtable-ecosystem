import os
import csv
import sys
import datetime
from google.cloud import bigtable
from dotenv import load_dotenv

def setup_demo_bigtable():
    # Load environment variables
    env_path = os.path.join(os.path.dirname(__file__), '../.env')
    load_dotenv(env_path)

    PROJECT_ID = os.getenv("GOOGLE_CLOUD_PROJECT")
    INSTANCE_ID = os.getenv("BIGTABLE_INSTANCE_ID")
    TABLE_ID = "patients"
    CSV_FILE = os.path.join(os.path.dirname(__file__), "demo_dataset.csv")

    if not PROJECT_ID or not INSTANCE_ID:
        print("ERROR: GOOGLE_CLOUD_PROJECT and BIGTABLE_INSTANCE_ID must be set in .env")
        sys.exit(1)

    if not os.path.exists(CSV_FILE):
        print(f"ERROR: Dataset file '{CSV_FILE}' does not exist.")
        sys.exit(1)

    print(f"Connecting to Bigtable project '{PROJECT_ID}', instance '{INSTANCE_ID}'...")
    client = bigtable.Client(project=PROJECT_ID, admin=True)
    instance = client.instance(INSTANCE_ID)
    table = instance.table(TABLE_ID)

    # 1. Create Table and Column Families
    if not table.exists():
        print(f"Creating table {TABLE_ID}...")
        table.create()
    else:
        print(f"Table {TABLE_ID} already exists.")

    families = table.list_column_families()
    for cf in ['tests', 'prescriptions', 'visits', 'profile']:
        if cf not in families:
            print(f"Creating column family: {cf}")
            table.column_family(cf).create()
        else:
            print(f"Column family '{cf}' already exists.")

    # 2. Seed Demo Data from CSV
    print(f"\nLoading standardized demo dataset from '{CSV_FILE}'...")
    
    # Group rows by row_key
    rows_data = {}
    count = 0
    with open(CSV_FILE, mode="r", encoding="utf-8") as file:
        reader = csv.DictReader(file)
        for entry in reader:
            rkey = entry["row_key"]
            if rkey not in rows_data:
                rows_data[rkey] = []
            rows_data[rkey].append(entry)
            count += 1

    print(f"Found {count} records across {len(rows_data)} patient keys.")

    for rkey, cells in rows_data.items():
        print(f"-> Seeding data for patient: {rkey}...")
        row = table.direct_row(rkey)
        for cell in cells:
            cf = cell["column_family"]
            cq = cell["column_qualifier"]
            value = cell["value"]
            ts_str = cell["timestamp"]
            
            try:
                dt = datetime.datetime.fromisoformat(ts_str)
            except Exception:
                dt = datetime.datetime.now(datetime.timezone.utc)

            row.set_cell(cf, cq.encode("utf-8"), value.encode("utf-8"), timestamp=dt)
        
        row.commit()
        print(f"   Successfully committed {len(cells)} cells for '{rkey}'.")

    print(f"\nSUCCESS: Bigtable instance '{INSTANCE_ID}' is fully initialized with uniform demo data!")

if __name__ == "__main__":
    setup_demo_bigtable()
