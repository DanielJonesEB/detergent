#!/usr/bin/env bash
# detergent-statusline.sh — Claude Code statusline renderer
# Reads Claude Code JSON from stdin (for cwd), finds detergent.yaml,
# calls `detergent statusline-data`, and renders a horizontal concern graph.
#
# Usage in .claude/settings.local.json:
#   { "statusLine": { "type": "command", "command": "./statusline/detergent-statusline.sh" } }

set -euo pipefail

# --- Configuration ---
CACHE_TTL=3  # seconds
CACHE_DIR="${TMPDIR:-/tmp}/detergent-statusline"
DETERGENT_BIN="${DETERGENT_BIN:-detergent}"

# --- Find detergent.yaml by walking up from a directory ---
find_config() {
    local dir="$1"
    while [ "$dir" != "/" ]; do
        if [ -f "$dir/detergent.yaml" ]; then
            echo "$dir/detergent.yaml"
            return 0
        fi
        if [ -f "$dir/detergent.yml" ]; then
            echo "$dir/detergent.yml"
            return 0
        fi
        dir="$(dirname "$dir")"
    done
    return 1
}

# --- Cache management ---
get_cached_data() {
    local config_path="$1"
    local cache_key
    cache_key=$(echo "$config_path" | md5sum 2>/dev/null | cut -d' ' -f1 || echo "$config_path" | md5 2>/dev/null || echo "default")
    local cache_file="$CACHE_DIR/$cache_key"

    mkdir -p "$CACHE_DIR"

    # Check cache freshness
    if [ -f "$cache_file" ]; then
        local now mtime age
        now=$(date +%s)
        if stat -f %m "$cache_file" >/dev/null 2>&1; then
            mtime=$(stat -f %m "$cache_file")
        else
            mtime=$(stat -c %Y "$cache_file")
        fi
        age=$((now - mtime))
        if [ "$age" -lt "$CACHE_TTL" ]; then
            cat "$cache_file"
            return 0
        fi
    fi

    # Fetch fresh data
    local data
    if data=$("$DETERGENT_BIN" statusline-data "$config_path" 2>/dev/null); then
        echo "$data" > "$cache_file"
        echo "$data"
        return 0
    fi

    # If command fails but cache exists, return stale data
    if [ -f "$cache_file" ]; then
        cat "$cache_file"
        echo "__stale__" >&2
        return 0
    fi

    return 1
}

# --- Render graph using Python for reliable JSON + ANSI handling ---
render_graph() {
    local json_data="$1"
    local is_stale="${2:-}"

    echo "$json_data" | python3 -c "
import json, sys

data = json.load(sys.stdin)
concerns = {c['name']: c for c in data.get('concerns', [])}
roots = data.get('roots', [])
graph = data.get('graph') or []
is_stale = $([ -n "$is_stale" ] && echo 'True' || echo 'False')

if not concerns:
    print('detergent: no concerns')
    sys.exit(0)

# ANSI colors
GREEN = '\033[32m'
CYAN = '\033[36m'
YELLOW = '\033[33m'
RED = '\033[31m'
DIM = '\033[2m'
RESET = '\033[0m'

def status_symbol(state, last_result=''):
    if state == 'running':  return '⟳'
    if state == 'failed':   return '✗'
    if state == 'skipped':  return '⊘'
    if state == 'idle':
        if last_result == 'modified': return '*'
        if last_result == 'noop':     return '✓'
        return '·'
    if state == 'unknown':  return '·'
    return '◯'

def status_color(state, last_result=''):
    if state == 'running':  return YELLOW
    if state == 'failed':   return RED
    if state == 'skipped':  return DIM
    if state == 'idle':
        if last_result == 'modified': return CYAN
        if last_result == 'noop':     return GREEN
        return DIM
    if state == 'unknown':  return DIM
    return RESET

def render_concern(name):
    c = concerns[name]
    state = c.get('state', 'unknown')
    lr = c.get('last_result', '')
    sym = status_symbol(state, lr)
    clr = status_color(state, lr)
    return f'{clr}{name} {sym}{RESET}'

# Build downstream map
downstream = {}
for edge in graph:
    downstream.setdefault(edge['from'], []).append(edge['to'])

# Group roots by watched branch
branch_roots = {}
for c in data['concerns']:
    if c['name'] in roots:
        branch_roots.setdefault(c['watches'], []).append(c['name'])

def build_chain(name):
    \"\"\"Follow single-child path from name.\"\"\"
    chain = [name]
    while name in downstream and len(downstream[name]) == 1:
        name = downstream[name][0]
        chain.append(name)
    return chain

def render_chain(chain):
    return ' ── '.join(render_concern(n) for n in chain)

def collect_branches(name):
    \"\"\"Collect all branches (chains) rooted at name, DFS.\"\"\"
    chain = build_chain(name)
    last = chain[-1]
    result = [chain]
    if last in downstream and len(downstream[last]) > 1:
        for child in downstream[last]:
            result.extend(collect_branches(child))
    return result

stale_marker = f' {DIM}(stale){RESET}' if is_stale else ''

for branch, root_names in branch_roots.items():
    # Collect all fork arms
    arms = []
    for rn in root_names:
        arms.extend(collect_branches(rn))

    if len(arms) == 1:
        # Simple chain, no forks
        print(f'{branch} ─── {render_chain(arms[0])}{stale_marker}', end='')
    else:
        # Multiple arms: use tree connectors
        print(f'{branch} ─┬─ {render_chain(arms[0])}', end='')
        padding = ' ' * (len(branch) + 2)
        for i, arm in enumerate(arms[1:], 1):
            connector = '└' if i == len(arms) - 1 else '├'
            print(f'\n{padding}{connector}─ {render_chain(arm)}', end='')
        print(f'{stale_marker}', end='')
" 2>/dev/null
}

# --- Main ---
main() {
    local input
    if ! input=$(cat); then
        exit 1
    fi

    local cwd
    cwd=$(echo "$input" | python3 -c "
import json, sys
d = json.load(sys.stdin)
# Prefer project_dir (stable), fall back to cwd
w = d.get('workspace') or {}
print(w.get('project_dir') or d.get('cwd') or '')
" 2>/dev/null || echo "")

    if [ -z "$cwd" ]; then
        cwd="$(pwd)"
    fi

    local config_path
    if ! config_path=$(find_config "$cwd"); then
        exit 0
    fi

    local is_stale=""
    local json_data
    mkdir -p "$CACHE_DIR"
    if ! json_data=$(get_cached_data "$config_path" 2>"$CACHE_DIR/.stale_check"); then
        exit 0
    fi

    if grep -q "__stale__" "$CACHE_DIR/.stale_check" 2>/dev/null; then
        is_stale="1"
    fi

    render_graph "$json_data" "$is_stale"
}

main
