# Bigtable Schema Design Guide for Agents

This document provides guidelines for designing performant schemas.

## Key concepts
* **Row key:** Bigtable stores data lexicographically (sorted alphabetically) by row key. For best performance queries should be design to filter by rowkey in its entirety or prefix. Point lookups by row key or reading ranges starting with a key will be the most performant. Rowkeys can have multiple parts combined using a delimiter, typically following a hierarchical format such as `category#subcategory#productID` as in `apparel#shoes#0123`. Bigtable doesn't support multi-row transactions but changes within a row are transactional. When designing schemas put data that needs to be updated transactionally within the same row.
* **Column Families:** Group data that is accessed together within a row. Defined as part of the schema. Contents of a family can easily be deleted in bulk with a single command for a given row key.
* **Column Qualifiers:** Defined at write time. Each row can have as many unique qualifiers within the row size limits (256 MB) with no limit on number of qualifiers per table. Qualifiers can be used in two ways: 1. as attributes in a JSON document e.g. `zipcode`, `city`, `state`, `street address` or 2. to store data like affinity scores e.g. `0.9`, `0.7` for different products or web pages they visited e.g. `home`, `search`, `cart`.   
* **Timestamps:** Are used for versioning. They are not system timestamps. They are user-defined and often used for event times like a sensor reading, address change timestamp or date a social media post was written. They can be used to expire items using TTL or move them to cold storage for cost savings as well as time-travel queries to find the "as of" state of a record.

## Row Key Design & Hotspotting
 If row keys are autoincrement or are prefixed by date or timestamp, all writes will hit a single node, creating a "hotspot" and killing performance. Bigtable's in-memory tier addresses hotspotting for reads (e.g. trending content on social media) but keys should be designed by keeping writes in mind. 

### Distribution Strategy
To ensure high performance, agents must validate that row keys are designed for **high cardinality**.
* **Avoid:** Sequential timestamps at the start of the key.
* **Prefer:** Prefixes to divide up the key space or reversed timestamps (e.g., `tenantID#reversedTimestamp#objectID`).

#### Field Salting Example
If a user must use a low-cardinality prefix, recommend "salting" the key:
`salt = hash(original_key) % number_of_nodes`
`new_row_key = salt + "#" + original_key`

## Performance Checklist
When reviewing user code or generating code, apply the following checks:
| Constraint | Requirement | Agent Tip |
| :--- | :--- | :--- |
| **Row Key Size** | < 4KB (Ideal: 10–100 bytes) | Large keys increase memory pressure and disk usage. |
| **Uniqueness** | Must be globally unique | Duplicate keys will overwrite existing data cells. |
| **Regex Check** | `^[a-zA-Z0-0\-_#]+$` | Recommend sticking to alphanumeric, underscores, and hashes for compatibility for row keys, column families and column qualifiers. Zero pad all numbers since they will be sorted like strings.|
| **Column Qualifier Size:** | < 16 KB | Longer column qualifiers will increase your storage footprint | 
| **Column Family Limits:** | < 100 | Large number of families degrade performance. While there is no explicit limit, keeping family names short is recommended. | 
| **Field Size:** | < 10 MB | This is the recommended value. 100 MB is the hard limit. Larger the cell, slower the retrieval | 
| **Row Size:** | < 100 MB | Bigtable doesn't check row size at write time but enforces a hard limit of 256 MB at read time.  |
