# git-wtkit

Initialize a worktree's submodules without downloading Git history already present
in another worktree. Runs independently of OpenCode. Requires Go 1.23+ to build
and Git on `PATH` at runtime.

```sh
go install .   # put $(go env GOPATH)/bin on PATH
git wtkit     # show help
git wtkit install   # optional: register git wtsetup in global Git config
cd /path/to/new/worktree
git wtkit setup     # or: git wtsetup
```

`install` sets `alias.wtsetup = wtkit setup`. To remove the alias, run
`git config --global --unset alias.wtsetup`.

### Agent skill

Install the bundled skill for OpenCode across projects, from any directory:

```sh
git wtkit install skill
```

Agents can discover the skill when preparing a worktree that needs submodules.
To request it explicitly, say: `Use @git-wtkit to prepare this worktree`.

### Setup behavior

Run `git wtkit install hook` once inside the parent repository to enable
automatic setup across its worktrees. A `post-checkout` hook handles
`git worktree add` (including OpenCode) and branch switches; `--no-checkout`
and file-only checkouts do not trigger setup. It follows the parent index,
not the latest upstream branch. If setup fails, Git has already checked out
the parent; resolve the reported module and rerun `git wtkit setup`.

Existing hooks and custom `core.hooksPath` are never replaced. To integrate
with an existing post-checkout hook, run this in the target worktree after
its existing logic:

```sh
if [ "$3" = 1 ]; then
  (unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR GIT_PREFIX; git wtkit setup)
fi
```

`setup` reads the current **index** and recursively checks out its pinned
commits. Output has one line per module:

- `local`: cloned from a populated module at the same relative path in a sibling
  worktree. The configured upstream remains `origin`. Git can hardlink local
  objects, but the checkout does not depend on the donor staying around.
- `upstream`: used `git submodule update`, borrowing objects from a local
  reference when available, then dissociating. Git uses your normal credentials.
- `ready`: already at the pinned commit. Continue into nested modules.

Clean detached checkouts follow changed pins, using their own objects first,
then sibling worktrees, then upstream. Dirty modules and local branches that
would have to move are reported and left alone. Independent
modules continue; any failure gives a nonzero exit status. Rerun after resolving
conflicts. Only complete, non-shallow repositories are used as local donors.
Local hits cover Git objects; external checkout filters such as Git LFS can still
require their own downloads. Build directories are outside the tool's scope.

Run `sh integration_test.sh` for an isolated end-to-end test. To verify all direct
pins of an actual worktree using local donors and file-only Git transports:

```sh
WTKIT_TEST_PROJECT=/absolute/path/to/worktree go test -v -count=1 ./...
```
