---
name: goback-merge
description: >-
  Merge the current change's PR, pull master, and remove the worktree + branch.
  Use for the tail end of the change workflow, after the user has approved
  merging the PR.
---

# goback merge

Wraps up a change once the user has **approved the merge**. Run from inside the
change's worktree:

```bash
scripts/merge.sh
```

`scripts/merge.sh` squash-merges the branch's PR, checks out and pulls `master`
in the main checkout, then removes this worktree and deletes the branch (local
and remote). It refuses to run on `master`/`main`, if the worktree has
uncommitted changes, or if the PR isn't OPEN.

Only run this after the user has OK'd merging — confirm first if there's any
doubt. The worktree is deleted on success, so `cd` to the main checkout
afterwards.
