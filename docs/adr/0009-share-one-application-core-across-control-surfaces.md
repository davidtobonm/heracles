# Share one application core across control surfaces

The CLI, stdio MCP server, and future desktop application will expose the same Heracles capabilities through one shared application core. The MCP server will ship in the Go binary as `heracles mcp serve`, expose high-level workflow operations without arbitrary shell execution, and contain no duplicate orchestration logic.
