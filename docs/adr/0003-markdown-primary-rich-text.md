# Markdown as Primary Rich Text Interchange Format

## Context & Decision
Jira Cloud REST v3 strictly expects Atlassian Document Format (ADF JSON AST) for issue descriptions and comments, and Confluence relies on Storage Format (XHTML). LLMs produce and consume Markdown natively, whereas generating complex ADF JSON leads to severe token bloat and malformed AST structures. We decided to use Markdown as the default rich-text format across all MCP tool interfaces, performing transparent bidirectional translation to ADF and XHTML, with an optional pass-through for raw formatting.

## Considered Options
- **Direct ADF/XHTML JSON**: Requiring the LLM to provide native ADF. Rejected due to excessive token cost and high LLM error rates when constructing nested JSON ASTs.
- **Strict Markdown Only**: Stripping advanced markup entirely. Rejected because users need Confluence macros and complex table structures that pure Markdown cannot represent.
