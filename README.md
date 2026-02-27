# tautulli-yamtrack-exporter

A Go script that exports Plex watch history from [Tautulli](https://github.com/Tautulli/Tautulli) into a [Yamtrack](https://github.com/FuzzyGrim/Yamtrack) compatible CSV.

## Purpose

Yamtrack is a self-hosted media tracker that [supports importing data via CSV in a specific format](https://github.com/FuzzyGrim/Yamtrack/wiki/Media-Import-Configuration#yamtrack-csv-format). Tautulli tracks every play on your Plex server. This tool bridges the gap by:

1.  **Fetching** your movie and episode history via the Tautulli API.
2.  **Deduplicating** entries (ensuring multiple watches or episode plays don't create redundant show-level records).
3.  **Formatting** timestamps to ISO-8601 strings compatible with Yamtrack's database schema.
4.  **Categorizing** exports into specific movie, episode, and global history files.

Note that Yamtrack already has a native Plex integration that will import ongoing watch history. This tool is intended for a one-time export/import of historical data that was recorded prior to a Yamtrack instance being installed.

## Prerequisites

* **Go**: Version 1.18 or higher (if compiling/running from source).
* **Tautulli API Key**: Found in Tautulli > Settings > Web Interface > API Key (`settings#tabs_tabs-web_interface`).

Note that all data is exported via a [single Tautulli API resource](https://docs.tautulli.com/extending-tautulli/api-reference?q=get_history#get_history), using the `get_history` API method.

## Usage

The exporter requires your Tautulli endpoint, API key, and the specific Plex username you wish to export history for.

```bash
git clone git@github.com:dbeg/tautulli-yamtrack-exporter.git
cd tautulli-yamtrack-exporter
go run main.go --endpoint http://127.0.0.1:8181 --api-key YOUR_TAUTULLI_API_KEY --username YOUR_PLEX_USERNAME
```

### Flags

| Flag | Description | Default |
| :--- | :--- | :--- |
| `--endpoint` | URL of your Tautulli server | `http://127.0.0.1:8181` |
| `--api-key` | Your Tautulli API Key (**Required**) | N/A |
| `--username` | Plex/Tautulli username to export (**Required**) | N/A |
| `--start` | Filter records after date (YYYY-MM-DD) | All history |
| `--end` | Filter records before date (YYYY-MM-DD) | All history |
| `--dry-run` | Count records without downloading or writing files | `false` |
| `--verbose` | Enable detailed debug logging and API URL output | `false` |

### How to import to Yamtrack

1. Run the exporter to generate CSV export/import files in the `./out/` directory.
1. Within Yamtrack, navigate to Settings > Import Data (`/settings/import`)
1. Upload the generated CSV file(s).
1. That's it!
