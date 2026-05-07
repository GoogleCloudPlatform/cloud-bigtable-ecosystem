# Overview

The Bigtable remote Admin MCP server standardizes how large language models (LLMs) and AI agents to  query and manage Bigtable resources ([source](https://docs.cloud.google.com/bigtable/docs/use-bigtable-mcp?content_ref=mcp+servers+let+you+use+their+tools+resources+and+prompts+to+take+actions+and+get+updated+data+from+their+backend+service)). It operates as a remote server on Google Cloud infrastructure, offering an HTTP endpoint for communication ([source](https://docs.cloud.google.com/bigtable/docs/use-bigtable-mcp?content_ref=remote+mcp+servers+run+on+the+service+s+infrastructure+and+offer+an+http+endpoint+to+ai+applications)).

## Prerequisites

*   A Google Cloud project with billing enabled ([source](https://docs.cloud.google.com/bigtable/docs/use-bigtable-mcp?content_ref=verify+that+billing+is+enabled+for+your+google+cloud+project)).
*   The **Bigtable Admin API** must be enabled ([source](https://docs.cloud.google.com/bigtable/docs/use-bigtable-mcp?content_ref=enable+the+bigtable+admin+api)).
*   The following IAM roles are required ([source](https://docs.cloud.google.com/bigtable/docs/use-bigtable-mcp?content_ref=ask+your+administrator+to+grant+you+the+following+iam+roles+on+the+project+where+you+want+to+use+the+bigtable+mcp+server)):
    *   **MCP Tool User** (`roles/mcp.toolUser`) to make tool calls ([source](https://docs.cloud.google.com/bigtable/docs/use-bigtable-mcp?content_ref=make+mcp+tool+calls+mcp+tool+user+roles+mcp+tooluser)).
    *   **Bigtable Administrator** (`roles/bigtable.admin`) for resource access ([source](https://docs.cloud.google.com/bigtable/docs/use-bigtable-mcp?content_ref=full+access+to+bigtable+resources+bigtable+administrator+roles+bigtable+admin)).

## Documentation

For more detailed information, including available tools and security configurations like Model Armor, refer to the official documentation:
[Use the Bigtable remote MCP server](https://docs.cloud.google.com/bigtable/docs/use-bigtable-mcp) ([source](https://docs.cloud.google.com/bigtable/docs/use-bigtable-mcp?content_ref=use+the+bigtable+remote+mcp+server))