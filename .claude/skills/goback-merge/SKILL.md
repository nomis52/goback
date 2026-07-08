---
name: goback-merge
description: >-
  Merge the current change's PR, pull master, and remove the worktree + branch.
  Use for the tail end of the change workflow, after the user has approved
  merging the PR.
---

# goback merge

Wraps up a change once the user has **approved the merge**: squash-merges the
PR, updates `master` in the main checkout, and removes the worktree and its
branch. Every step assumes the user already OK'd merging — confirm before
running if there's any doubt.

## Steps

1. **Identify the change.** Discover the current worktree path, branch, and PR
   rather than hard-coding:

   ```bash
   git worktree list                 # find this worktree's path
   git rev-parse --abbrev-ref HEAD   # current branch, e.g. worktree-<topic>
   gh pr view --json number,title,state,url   # confirm the PR to merge
   ```

2. **Merge the PR** (squash, and delete the remote branch):

   ```bash
   gh pr merge --squash --delete-branch
   ```

3. **Update master in the main checkout.** Worktree removal must run from the
   main checkout, and `master` is checked out there. Substitute `<main-checkout>`
   (the top-level repo dir, e.g. `/Users/simon/homelab/goback`):

   ```bash
   git -C <main-checkout> checkout master
   git -C <main-checkout> pull
   ```

4. **Remove the worktree** (run from the main checkout):

   ```bash
   git -C <main-checkout> worktree remove <worktree-path>
   ```

5. **Delete the local branch** if `gh pr merge --delete-branch` didn't already
   remove it locally:

   ```bash
   git -C <main-checkout> branch -D worktree-<topic>
   ```

6. Confirm to the user: PR merged, `master` up to date, worktree removed.
