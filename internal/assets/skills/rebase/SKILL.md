---
name: rebase
description: Safely rebase the current branch onto another branch with automatic conflict resolution, backup, and stash management.
metadata:
  author: detergent
  version: "1.0"
---

Perform a safe, fully-automated rebase of the current branch onto a target branch.

**This skill handles the entire rebase lifecycle**: backup, stash, fetch, rebase, conflict resolution, stash pop, and verification.

---

## Inputs

The user may provide a **target branch** as an argument (e.g., `/rebase main`). If not provided, auto-detect the default branch.

---

## Phase 1: Preflight

1. **Identify current branch**
   ```bash
   git branch --show-current
   ```
   If empty (detached HEAD), STOP and tell the user: "Cannot rebase in detached HEAD state."

2. **Identify target branch**
   Use the argument if provided. Otherwise auto-detect:
   ```bash
   git symbolic-ref refs/remotes/origin/HEAD 2>/dev/null | sed 's@^refs/remotes/origin/@@'
   ```
   If that fails, check for `main` then `master`:
   ```bash
   git rev-parse --verify origin/main 2>/dev/null && echo main
   git rev-parse --verify origin/master 2>/dev/null && echo master
   ```
   If nothing works, ask the user with `AskUserQuestion`.

3. **Verify not rebasing onto self**
   If current branch equals target branch, STOP: "Already on the target branch."

4. **Check for uncommitted changes**
   ```bash
   git status --porcelain
   ```
   Store whether there are changes as `HAD_CHANGES`.

---

## Phase 2: Safety Net

5. **Create backup branch**
   ```bash
   git branch -f pre-rebase-backup
   ```
   Report: "Created backup at `pre-rebase-backup` (SHORTSHA)"

6. **Stash if needed**
   If `HAD_CHANGES` is true:
   ```bash
   git stash push -m "detergent-rebase-autostash"
   ```
   Store `DID_STASH=true`. If the stash fails, STOP — do not proceed with dirty state.

---

## Phase 3: Fetch & Rebase

7. **Fetch target**
   ```bash
   git fetch origin <target-branch>
   ```

8. **Attempt rebase**
   ```bash
   git rebase origin/<target-branch>
   ```
   - If clean (exit 0): skip to Phase 5
   - If conflicts: proceed to Phase 4

---

## Phase 4: Conflict Resolution (loop)

Repeat until the rebase completes or is aborted. Track `CONFLICT_ROUND` starting at 0.

9. **List conflicted files**
   ```bash
   git diff --name-only --diff-filter=U
   ```

10. **For each conflicted file:**
    a. **Read** the full file (it contains conflict markers)
    b. **Understand both sides:**
       - `<<<<<<<` to `=======` is the current branch's version (ours)
       - `=======` to `>>>>>>>` is the incoming version (theirs)
    c. **Resolve intelligently:**
       - Preserve intent of both sides where possible
       - When changes are incompatible, prefer the current branch's logic but incorporate non-conflicting updates from the target
       - **Remove ALL conflict markers** — no `<<<<<<<`, `=======`, or `>>>>>>>` may remain
    d. **Verify** the resolved file has no remaining conflict markers:
       ```bash
       grep -c '<<<<<<<\|=======\|>>>>>>>' <file> || true
       ```
       If any remain, re-resolve.
    e. **Stage** the resolved file:
       ```bash
       git add <file>
       ```

11. **Continue rebase**
    ```bash
    GIT_EDITOR=true git rebase --continue
    ```
    Set `GIT_EDITOR=true` to prevent interactive editor prompts.
    - If more conflicts appear: increment `CONFLICT_ROUND`, return to step 9
    - If clean: proceed to Phase 5
    - If "nothing to commit" error: `git rebase --skip` and continue

**ABORT CONDITION**: If `CONFLICT_ROUND` exceeds 10, something is seriously wrong. Abort:
```bash
git rebase --abort
```
Tell the user: "Rebase aborted after too many conflict rounds. Your branch is restored to its pre-rebase state. Backup branch `pre-rebase-backup` is available."

---

## Phase 5: Restore Stash

12. **Pop stash if we stashed**
    If `DID_STASH` is true:
    ```bash
    git stash pop
    ```
    - If conflicts during pop: resolve using the same approach as Phase 4, then stage resolved files
    - If pop fails entirely: do NOT drop the stash. Tell the user: "Stash could not be cleanly applied. Your changes are safe in `git stash list`. Apply manually with `git stash pop`."

---

## Phase 6: Report

13. **Show summary**
    ```bash
    git log --oneline pre-rebase-backup..HEAD
    ```

    Report to the user:

    ```
    Rebase complete.

    - Branch: <current> rebased onto origin/<target>
    - Commits rebased: <N>
    - Conflicts resolved: <count> (or "none")
    - Stash: restored (or "nothing to restore")
    - Backup: `pre-rebase-backup` at <shortsha>
    - To undo: git reset --hard pre-rebase-backup
    ```

---

## Guardrails

- **NEVER** force-push. The rebase is local only. If the user wants to push, they must do it themselves (and they should use `--force-with-lease`, not `--force`).
- **NEVER** delete the `pre-rebase-backup` branch. The user decides when to clean it up.
- **ALWAYS** create the backup branch before any destructive operation.
- **ALWAYS** set `GIT_EDITOR=true` when running `git rebase --continue` to prevent editor prompts.
- If the rebase is aborted, verify the branch is back to its pre-rebase state.
- If anything unexpected happens (command failures, unknown git state), prefer aborting and restoring over continuing blindly.
- Do **not** run tests or builds automatically. If the user wants verification, they can ask.
- This skill only performs local git operations — no remote mutations.
