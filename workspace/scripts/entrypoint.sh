#!/bin/bash
set -e

# ClaraTeach Workspace Entrypoint
# Manages tmux session for terminal persistence

SESSION_NAME="workspace"

# If running interactively (with TTY), use tmux
if [ -t 0 ] && [ -t 1 ]; then
    # Check if tmux session already exists
    if tmux has-session -t "$SESSION_NAME" 2>/dev/null; then
        echo "Attaching to existing tmux session: $SESSION_NAME"
        exec tmux attach-session -t "$SESSION_NAME"
    else
        echo "Creating new tmux session: $SESSION_NAME"
        # Create new session and run the provided command (or bash)
        exec tmux new-session -s "$SESSION_NAME" "$@"
    fi
else
    # Non-interactive: just run the command directly
    exec "$@"
fi
