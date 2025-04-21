#!/bin/bash

# --- Configuration ---
# The generic commit message to use for the daily squashed commit.
GENERIC_MESSAGE="prep for release"
# Set the editor git will use for the 'reword' operation.
# This simple editor script just replaces the commit message file content.
GIT_EDITOR_SCRIPT=$(mktemp /tmp/git-editor-script.XXXXXX) # Use /tmp explicitly
chmod +x "$GIT_EDITOR_SCRIPT"
cat << EOF > "$GIT_EDITOR_SCRIPT"
#!/bin/sh
echo "$GENERIC_MESSAGE" > "\$1"
EOF
# --- End Configuration ---

# --- Script Logic ---
set -e # Exit immediately if a command exits with a non-zero status.

# Function to cleanup temporary files
cleanup() {
  rm -f "$GIT_EDITOR_SCRIPT"
  # Remove rebase todo file only if it was created
  [[ -n "${REBASE_TODO_FILE:-}" && -f "$REBASE_TODO_FILE" ]] && rm -f "$REBASE_TODO_FILE"
  # echo "Temporary files cleaned up." # Optional: uncomment for verbose cleanup
}
trap cleanup EXIT # Ensure cleanup runs on script exit (normal or error)

# Function to display help
usage() {
  echo "Usage: $0 [-m <message>] [-s <start_commit_ref>] [--dry-run] [-h]"
  echo "  -m <message>:           Override the default generic commit message ('$GENERIC_MESSAGE')."
  echo "  -s <start_commit_ref>:  The commit hash or ref *before* the range you want to process."
  echo "                          If omitted, processes from the repository root (--root)."
  echo "  --dry-run:              Print the generated rebase 'todo' list instead of executing the rebase."
  echo "  -h:                     Show this help message."
  echo
  echo "WARNING: This script rewrites Git history. Use with caution and preferably on a backup branch."
  exit 1
}

# --- Argument Parsing ---
START_COMMIT_ARG=""
DRY_RUN=0

# Pre-process arguments to handle --dry-run before getopts
ARGS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN=1
      shift # Consume --dry-run
      ;;
    # Add cases for other long options here if needed in the future
    *)
      ARGS+=("$1") # Keep the argument
      shift # Consume the argument
      ;;
  esac
done
# Restore the arguments positional parameters ($1, $2, ...) with non-long-options
set -- "${ARGS[@]}"

# Now parse short options with getopts
while getopts ":hm:s:" opt; do
  case ${opt} in
    h )
      usage
      ;;
    m )
      GENERIC_MESSAGE=$OPTARG
      # Update the editor script with the new message
      cat << EOF > "$GIT_EDITOR_SCRIPT"
#!/bin/sh
echo "$GENERIC_MESSAGE" > "\$1"
EOF
      ;;
    s )
      START_COMMIT_ARG=$OPTARG
      ;;
    \? )
      echo "Invalid option: -$OPTARG" 1>&2 # Corrected error message format
      usage
      ;;
    : )
      echo "Invalid option: -$OPTARG requires an argument" 1>&2
      usage
      ;;
  esac
done
shift $((OPTIND -1)) # Shift away processed short options

# Check for unexpected remaining arguments
if [[ $# -gt 0 ]]; then
    echo "Unexpected arguments: $*"
    usage
fi
# --- End Argument Parsing ---


# Safety Check: Ensure working directory is clean
if ! git diff-index --quiet HEAD --; then
    echo "Error: Working directory is not clean. Please commit or stash changes."
    # cleanup function will run via trap
    exit 1
fi

# Determine the rebase starting point
REBASE_TARGET=""
if [[ -z "$START_COMMIT_ARG" ]]; then
    # Find the root commit
    ROOT_COMMIT=$(git rev-list --max-parents=0 HEAD)
    if [[ -z "$ROOT_COMMIT" ]]; then
        # Check if HEAD exists, maybe it's an empty repo?
        if ! git rev-parse --verify HEAD > /dev/null 2>&1; then
             echo "Repository appears to be empty. No commits to process."
             exit 0 # Exit successfully, nothing to do
        else
             echo "Error: Could not determine the root commit, but HEAD exists. Strange state?"
             exit 1
        fi
    fi
    # Check if there's more than one commit, otherwise rebase --root doesn't make sense
    if [[ $(git rev-list --count HEAD) -le 1 ]]; then
         echo "Only one commit or less found. No rebase needed."
         exit 0
    fi
    echo "Processing from the root commit ($ROOT_COMMIT)."
    REBASE_TARGET="--root"
    # Get commits from root, oldest first
    COMMIT_LOG=$(git log --reverse --pretty=format:'%H %cs' --no-patch $REBASE_TARGET)
else
    # Check if the start commit exists
    if ! git cat-file -e "$START_COMMIT_ARG^{commit}" 2>/dev/null; then
        echo "Error: Start commit '$START_COMMIT_ARG' not found."
        exit 1
    fi
    echo "Processing commits after $START_COMMIT_ARG."
    REBASE_TARGET="$START_COMMIT_ARG"
    # Get commits after the specified one, oldest first
    COMMIT_LOG=$(git log --reverse --pretty=format:'%H %cs' --no-patch ${REBASE_TARGET}..HEAD)

    # Check if there are any commits to process
    if [[ -z "$COMMIT_LOG" ]]; then
        echo "No commits found after $START_COMMIT_ARG. Nothing to do."
        exit 0
    fi
fi


# Generate the rebase todo list
REBASE_TODO_FILE=$(mktemp /tmp/rebase-todo.XXXXXX) # Use /tmp explicitly
CURRENT_DAY=""
FIRST_COMMIT_IN_LOG=1 # Flag to track if it's the very first commit being processed

echo "Generating rebase instructions..."

while IFS= read -r line; do
    [[ -z "$line" ]] && continue # Skip empty lines if any

    COMMIT_HASH=$(echo "$line" | cut -d' ' -f1)
    COMMIT_DATE=$(echo "$line" | cut -d' ' -f2)

    if [[ "$COMMIT_DATE" != "$CURRENT_DAY" ]]; then
        # First commit of a new day (or the very first commit overall)
        # If it's the VERY first commit in the log (oldest), it MUST be 'pick' or 'reword'.
        # Subsequent first-commits-of-the-day can also be 'reword'.
        # Using 'reword' ensures we can apply the generic message.
        echo "reword $COMMIT_HASH # Date: $COMMIT_DATE" >> "$REBASE_TODO_FILE"
        CURRENT_DAY="$COMMIT_DATE"
        FIRST_COMMIT_IN_LOG=0 # Mark that we've processed at least one commit
    else
        # Subsequent commit on the same day
        # Need to check if it's the *very first* commit in the log.
        # If it is, and there are somehow multiple root commits (unlikely but possible),
        # or if processing from --root and the first day had multiple commits,
        # the *first* one must still be pick/reword. We already handled this above.
        # So, any commit here is guaranteed NOT to be the absolute first commit.
        echo "fixup $COMMIT_HASH # Date: $COMMIT_DATE (squashing)" >> "$REBASE_TODO_FILE"
    fi
done <<< "$COMMIT_LOG"

# Check if any instructions were generated
if [[ ! -s "$REBASE_TODO_FILE" ]]; then
    echo "No rebase instructions generated (COMMIT_LOG was likely empty or processing failed)."
    # This case should ideally be caught earlier by checks on COMMIT_LOG, but added as safety
    exit 0
fi

echo "Rebase instructions generated in $REBASE_TODO_FILE"

# --- Execution ---
if [[ $DRY_RUN -eq 1 ]]; then
    echo "--- DRY RUN ---"
    echo "Rebase command that would run: GIT_SEQUENCE_EDITOR=\"cat '$REBASE_TODO_FILE'\" GIT_EDITOR=\"'$GIT_EDITOR_SCRIPT'\" git rebase -i $REBASE_TARGET"
    echo "--- Generated Todo List ($REBASE_TODO_FILE): ---"
    cat "$REBASE_TODO_FILE"
    echo "--- End Dry Run ---"
else
    echo "--- Starting Rebase ---"
    echo "This will rewrite history. Make sure you have a backup!"
    echo "Running: GIT_SEQUENCE_EDITOR=\"cat '$REBASE_TODO_FILE'\" GIT_EDITOR=\"'$GIT_EDITOR_SCRIPT'\" git rebase -i $REBASE_TARGET"

    # Perform the rebase
    # We use GIT_SEQUENCE_EDITOR to provide our generated todo list non-interactively.
    # We use GIT_EDITOR to provide our generic message when 'reword' is encountered.
    if GIT_SEQUENCE_EDITOR="cat '$REBASE_TODO_FILE'" GIT_EDITOR="'$GIT_EDITOR_SCRIPT'" git rebase -i $REBASE_TARGET; then
        echo "Rebase completed successfully."
    else
        REBASE_EXIT_CODE=$?
        echo "Error during rebase (exit code: $REBASE_EXIT_CODE). Git might be in a rebase state."
        echo "You may need to resolve conflicts or run 'git rebase --abort'."
        # Don't delete temp files immediately in case of error, user might need them
        echo "Temporary files were NOT automatically cleaned up due to error:"
        echo "  Todo List: $REBASE_TODO_FILE"
        echo "  Editor Script: $GIT_EDITOR_SCRIPT"
        # Prevent the EXIT trap from deleting the files by clearing the variable
        REBASE_TODO_FILE=""
        exit $REBASE_EXIT_CODE # Signal error
    fi
    echo "--- Rebase Finished ---"
fi

# Cleanup is handled by the trap EXIT unless there was a rebase error

exit 0