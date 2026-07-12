#!/usr/bin/env bash
#
# merge.sh — merge the current worktree's PR, update master, and remove the
# worktree + branch.
#
# Run this from inside the change's worktree AFTER the user has approved merging
# the PR. It squash-merges the PR, pulls master in the main checkout, then
# removes this worktree and deletes the branch (local + remote).
#
# The worktree this script runs from is deleted on success, so cd back to the
# main checkout afterwards.
set -euo pipefail

WORKTREE="$(git rev-parse --show-toplevel)"
BRANCH="$(git -C "$WORKTREE" rev-parse --abbrev-ref HEAD)"

if [[ "$BRANCH" == "master" || "$BRANCH" == "main" ]]; then
  echo "error: refusing to run on '$BRANCH' — run this from a change worktree" >&2
  exit 1
fi

# Refuse if the worktree has uncommitted changes.
if [[ -n "$(git -C "$WORKTREE" status --porcelain)" ]]; then
  echo "error: worktree has uncommitted changes — commit or discard them first" >&2
  git -C "$WORKTREE" status --short >&2
  exit 1
fi

# The first entry of `git worktree list` is the main working tree.
MAIN="$(git worktree list --porcelain | awk '/^worktree /{print $2; exit}')"

PR_NUMBER="$(gh pr view "$BRANCH" --json number -q .number)"
PR_STATE="$(gh pr view "$BRANCH" --json state -q .state)"
if [[ "$PR_STATE" != "OPEN" ]]; then
  echo "error: PR #$PR_NUMBER for '$BRANCH' is $PR_STATE, not OPEN" >&2
  exit 1
fi

echo "Merging PR #$PR_NUMBER ($BRANCH)..."
gh pr merge "$PR_NUMBER" --squash

echo "Updating master in $MAIN ..."
git -C "$MAIN" checkout master
git -C "$MAIN" pull

echo "Removing worktree $WORKTREE ..."
# Run from the main checkout so our cwd isn't inside the dir being removed.
cd "$MAIN"
git -C "$MAIN" worktree remove "$WORKTREE"

echo "Deleting branch $BRANCH ..."
git -C "$MAIN" branch -D "$BRANCH" 2>/dev/null || true
git -C "$MAIN" push origin --delete "$BRANCH" 2>/dev/null || true

echo "Done: PR #$PR_NUMBER merged, master updated, worktree and branch removed."
echo "Note: the old worktree dir is gone — cd $MAIN"
