
import sys
import asyncio
from google.cloud.bigtable.data import BigtableDataClientAsync
import json


async def run_bigtable_sql(project_id, instance_id, sql_query):
  try:
    rows = []
    headers = []
    
    async with BigtableDataClientAsync(project=project_id) as client:
      async for row in await client.execute_query(sql_query, instance_id):
        current_cols = []
        row_data = {}
        for column_name, value in row.fields:
          val_str = str(value)
          row_data[column_name] = val_str
          current_cols.append(column_name)
        
        if not rows:
            headers.extend(current_cols)
            
        rows.append(row_data)

    if not rows:
        return "No results found."

    # Build Markdown table
    md_lines = []
    md_lines.append("| " + " | ".join(headers) + " |")
    md_lines.append("| " + " | ".join([":---"] * len(headers)) + " |")
    
    for r in rows:
        row_values = [r.get(h, "") for h in headers]
        md_lines.append("| " + " | ".join(row_values) + " |")
        
    return "\n".join(md_lines)

  except Exception as e:
     return f"An error occurred: {e}"

if __name__ == "__main__":
    if len(sys.argv) != 4:
        print("Usage: python run_bigtable_sql.py <project_id> <instance_id> <sql_query>")
        sys.exit(1)
    project_id = sys.argv[1]
    instance_id = sys.argv[2]
    sql_query = sys.argv[3]
    print(asyncio.run(run_bigtable_sql(project_id, instance_id, sql_query)))