# /line-rebase

Safely pick up changes from the terminal station branch onto the watched branch.

## When to use

Use this skill when the line statusline indicates there are commits ready on the terminal station branch that haven't been picked up on the main working branch.

## Procedure

1. **Identify branches**: Read `line.yaml` to determine the watched branch and the terminal (last) station.

2. **Check for changes**: Verify the terminal station branch (`line/stn/<terminal>`) has commits ahead of the watched branch.

3. **Stash current work**: If there are any uncommitted changes on the watched branch, stash them:
   ```sh
   git stash push -m "line-rebase: stashing WIP"
   ```

4. **Rebase from terminal station**: Rebase the watched branch to include the terminal station's changes. All station commits contain `[skip line]` markers, so they will NOT retrigger the assembly line (SKL-2):
   ```sh
   git rebase line/stn/<terminal>
   ```

5. **Restore work in progress**: If changes were stashed in step 3, restore them:
   ```sh
   git stash pop
   ```

6. **Verify**: Confirm the watched branch now includes the station improvements and any WIP is restored.

## Safety guarantees

- **No work is ever lost**: WIP is always stashed before any branch operations
- **No retriggering**: Station commits contain `[skip line]` which prevents `line run` from retriggering (RUN-9, SKL-2)
- **Fast and automatic**: The entire operation is a stash, rebase, unstash sequence

## Important

- NEVER force-push or reset any branches
- ALWAYS stash before rebase, even if the working tree appears clean
- If the rebase has conflicts, abort and inform the user rather than auto-resolving
- Verify the stash was applied successfully after the rebase
