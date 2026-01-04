# Request Logging in ShellAI

ShellAI automatically logs all API requests to a SQLite database to help you track usage and costs, similar to [Simon Willison's llm tool](https://llm.datasette.io/en/stable/logging.html).

## Log Database

Logs are stored in a SQLite database at: `~/.shell-ai/logs.db`

You can view the database path with:
```bash
q logs --path
```

## What Gets Logged

Each request is stored in the `responses` table with:
- **ID** - Unique request ID from OpenAI
- **Model** - Model used (e.g., gpt-4.1, gpt-4.1-mini)
- **Prompt** - Your query text
- **System** - System message/instructions
- **Response** - AI's complete response
- **Timestamp** - UTC timestamp
- **Token usage**:
  - Input tokens
  - Output tokens
- **Estimated cost** in USD
- **Duration** in milliseconds
- **Conversation ID** - For grouping related requests (future)

## Viewing Logs

### Show recent logs (default: 3)
```bash
q logs
```

### Show more entries
```bash
q logs -n 10
```

### JSON output
```bash
q logs --json
```

### Database statistics
```bash
q logs --status
```

This shows:
- Total number of requests
- Total tokens used
- Total estimated cost
- Breakdown by model

## Example Output

```
Entry 3 - 2025-12-14 14:30:15 [gpt-4.1-mini]

Prompt: list files in current directory

Response: ```bash
ls -la
```

Tokens: 45 input + 12 output = 57 total
Cost: $0.000014
Duration: 1247ms
Request ID: chatcmpl-abc123

────────────────────────────────────────────────────────────────────────────────

Entry 2 - 2025-12-14 14:28:03 [gpt-4.1]

...
```

## Analyzing Logs with SQL

Since logs are stored in SQLite, you can query them directly:

### View all requests
```bash
sqlite3 ~/.shell-ai/logs.db "SELECT * FROM responses ORDER BY datetime_utc DESC LIMIT 10"
```

### Calculate total cost
```bash
sqlite3 ~/.shell-ai/logs.db "SELECT SUM(estimated_cost) as total_cost FROM responses"
```

### Most expensive requests
```bash
sqlite3 ~/.shell-ai/logs.db "SELECT prompt, estimated_cost, model FROM responses ORDER BY estimated_cost DESC LIMIT 5"
```

### Requests by model
```bash
sqlite3 ~/.shell-ai/logs.db "SELECT model, COUNT(*) as count, SUM(estimated_cost) as cost FROM responses GROUP BY model"
```

### Average response time
```bash
sqlite3 ~/.shell-ai/logs.db "SELECT AVG(duration_ms) as avg_ms FROM responses WHERE duration_ms > 0"
```

### Token usage by day
```bash
sqlite3 ~/.shell-ai/logs.db "SELECT DATE(datetime_utc) as date, SUM(input_tokens + output_tokens) as tokens FROM responses GROUP BY DATE(datetime_utc)"
```

## Using with Datasette

You can explore your logs with [Datasette](https://datasette.io/):

```bash
pip install datasette
datasette ~/.shell-ai/logs.db
```

Then open http://localhost:8001 to browse and query your logs with a web UI.

## Database Schema

```sql
CREATE TABLE conversations (
    id TEXT PRIMARY KEY,
    name TEXT,
    model TEXT
);

CREATE TABLE responses (
    id TEXT PRIMARY KEY,
    model TEXT,
    prompt TEXT,
    system TEXT,
    response TEXT,
    conversation_id TEXT REFERENCES conversations(id),
    duration_ms INTEGER,
    datetime_utc TEXT,
    input_tokens INTEGER,
    output_tokens INTEGER,
    estimated_cost REAL
);
```

## Model Pricing (December 2024)

Cost estimates use the following pricing per 1M tokens:

| Model | Input | Output |
|-------|-------|--------|
| gpt-4.1 (gpt-4o) | $2.50 | $10.00 |
| gpt-4.1-mini (gpt-4o-mini) | $0.15 | $0.60 |
| gpt-4-turbo | $10.00 | $30.00 |
| gpt-4 | $30.00 | $60.00 |
| gpt-3.5-turbo | $0.50 | $1.50 |

*Prices are estimates based on OpenAI's current rates. Check [OpenAI's pricing page](https://openai.com/pricing) for current rates.*

## Privacy & Data

- **Local only**: All logs stored locally in `~/.shell-ai/logs.db`
- **No telemetry**: No data sent to third parties
- **Full context**: Logs contain your prompts and AI responses
- **Your control**: You own all your data

## Managing Logs

### Disable logging
Set the environment variable before running:
```bash
export SHELL_AI_DISABLE_LOGGING=1
q "your query"
```

Or add to your shell profile (~/.zshrc, ~/.bashrc):
```bash
export SHELL_AI_DISABLE_LOGGING=1
```

### Clear all logs
```bash
rm ~/.shell-ai/logs.db
```

### Back up logs
```bash
cp ~/.shell-ai/logs.db ~/backups/shell-ai-logs-$(date +%Y%m%d).db
```

### Export to JSON
```bash
q logs -n 1000 --json > my-logs.json
```

### Export to CSV
```bash
sqlite3 -header -csv ~/.shell-ai/logs.db "SELECT * FROM responses" > logs.csv
```

## How Token Tracking Works

ShellAI uses OpenAI's `stream_options` parameter to get accurate token counts even while streaming:

1. Request includes `stream_options: {include_usage: true}`
2. OpenAI sends response chunks as they're generated (streaming)
3. Final chunk includes complete token usage data
4. ShellAI captures and logs this data automatically

This ensures you get:
- ✅ Real-time streaming responses (no delay)
- ✅ Accurate token counts from OpenAI
- ✅ Precise cost estimates

## Future Features

Planned enhancements:
- **Conversation tracking** - Group related queries
- **Cost alerts** - Warn when approaching spending limits
- **Export formats** - CSV, Excel export options
- **Advanced filters** - Search by date, model, cost range
- **Usage reports** - Daily/weekly/monthly summaries

## Troubleshooting

### No logs appearing
- Ensure `SHELL_AI_DISABLE_LOGGING` is not set
- Check database path: `q logs --path`
- Verify you have write permissions to `~/.shell-ai/`

### Token counts are zero
- Older API endpoints might not support `stream_options`
- Azure OpenAI might not support this feature yet
- Check for errors in the response

### Database is locked
- Close any other programs accessing the database
- Make sure you're not running multiple `q` instances simultaneously

## Comparison to Other Tools

ShellAI's logging is inspired by **simonw/llm**, providing:
- ✅ SQLite database (easy to query and analyze)
- ✅ CLI commands for viewing logs
- ✅ Token usage and cost tracking
- ✅ Duration tracking
- ✅ Local-only storage (privacy-first)
- ✅ Compatible with Datasette for web-based exploration

Unlike JSONL logs, SQLite provides:
- Powerful querying with SQL
- Built-in indexing for fast searches
- ACID transactions (data integrity)
- Easy integration with analytics tools
