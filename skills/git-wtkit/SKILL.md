---
name: git-wtkit
description: Use when preparing a new Git or OpenCode worktree that needs submodules, or when a build in a worktree is blocked by uninitialized submodules. Reuse local Git objects before downloading.
---

Run `git wtkit setup` from the **target worktree**, not the checkout that already has the dependencies. Use the full command: `git wtkit` alone prints help, and the `wtsetup` alias is optional.

Setup recursively checks out the commits pinned in the index. It uses sibling worktrees first and may contact the configured upstream on a cache miss. If the task requires strictly offline operation, do not assume setup enforces it.

For automatic setup on future worktree creation and branch switches, run `git wtkit install hook` in the parent repository when requested. Clean detached submodules follow pinned commits; preserve dirty checkouts and development branches. If a hook reports failure, the parent checkout has already happened. Report the module path and retry setup after the conflict is resolved.

If Git cannot find `wtkit`, report that `git-wtkit` must be installed on `PATH` rather than silently starting a full recursive download.
