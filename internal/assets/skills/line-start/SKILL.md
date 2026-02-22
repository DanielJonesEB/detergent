---
name: line-start
description: Start the line runner to process commits through the station line. Normally auto-triggered by post-commit hooks.
metadata:
  author: line
  version: "3.0"
---

Start the line runner. Normally this happens automatically via the post-commit hook installed by `line init`, but you can run it manually if needed.

---

## How it works

After `line init`, a post-commit hook runs `line run` in the background on every commit. The runner processes all pending commits through the station line and exits.

## Manual start

If the runner isn't starting automatically (e.g., hook not installed), run it manually:

1. **Find the config file**
   Look for `line.yaml` or `line.yml` starting from the repo root:
   ```bash
   git rev-parse --show-toplevel
   ```

2. **Start the runner**
   Run using the Bash tool with `run_in_background: true`:
   ```bash
   line run
   ```
   If the config file is not at the default `line.yaml`, use `--path`:
   ```bash
   line run --path /path/to/config.yaml
   ```
   The runner will process pending commits and exit when done.

3. **Confirm**
   Tell the user:
   ```
   Assembly Line runner started. It will process pending commits and exit when done.
   ```
