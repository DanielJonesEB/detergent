# /line-preview

Read-only preview of what the assembly line would change. Shows what's different on station branches compared to the watched branch — the actual content, not the commit history. No branches are modified.

## When to use

Use this skill to see what the assembly line has changed before picking up the changes with `/line-rebase`.

## Procedure

All commands below are **read-only** — no branches are checked out, modified, or created.

1. **Read config**: Read `line.yaml` to determine the watched branch and the ordered list of stations.

2. **Check for stations**: If no stations are configured, report "No stations configured" and stop.

3. **Identify the terminal station**: The terminal station is the last in the list. Its branch is `line/stn/<terminal-name>`.

4. **Check terminal branch exists**: Run:
   ```sh
   git rev-parse --verify line/stn/<terminal-name>
   ```
   If this fails, report "The assembly line hasn't run yet — no station branches exist" and stop.

5. **Count unpicked commits**: Run:
   ```sh
   git rev-list --count <watched>..line/stn/<terminal-name>
   ```
   If the count is 0, report "No unpicked changes — the watched branch is up to date with the terminal station" and stop.

6. **Show what changed overall**: Show the diff from the watched branch to the terminal station — this is what `/line-rebase` would introduce:
   ```sh
   git diff <watched>...line/stn/<terminal-name>
   ```

7. **Show per-station breakdown**: For each station in order, show what it changed. The predecessor of the first station is the watched branch; for subsequent stations, the predecessor is the previous station's branch.

   For each station:
   ```sh
   git rev-list --count <predecessor>..line/stn/<station-name>
   ```
   - If the branch doesn't exist, note "branch not found (station may not have run yet)" and skip.
   - If the count is 0, skip (station ran but produced no changes).
   - Otherwise show the diff:
     ```sh
     git diff <predecessor>...line/stn/<station-name>
     ```

8. **Summarize**: Describe *what* is different — the specific content changes each station introduced. Don't describe commit counts, passes, or process. Suggest running `/line-rebase` to pick up the changes.

## Important

- This is a **read-only** inspection — do NOT checkout, merge, rebase, or modify any branches
- Use triple-dot (`...`) for `diff` (symmetric difference)
- Use double-dot (`..`) for `rev-list` (commit range)
- Focus on *what* changed (added lines, removed code, new sections), not *how many commits* or *how many passes*
- If a mid-chain station branch is missing, skip it in the breakdown but continue with subsequent stations (using the last valid predecessor)
