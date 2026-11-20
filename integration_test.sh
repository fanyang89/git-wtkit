#!/bin/sh
set -eu
tmp=$(mktemp -d "${TMPDIR:-/tmp}/wtkit.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
export GIT_CONFIG_GLOBAL="$tmp/gitconfig" GIT_CONFIG_NOSYSTEM=1 GIT_ALLOW_PROTOCOL=file
export GIT_AUTHOR_NAME=test GIT_COMMITTER_NAME=test
export GIT_AUTHOR_EMAIL=test@example.org GIT_COMMITTER_EMAIL=test@example.org
go build -o "$tmp/git-wtkit" .
expected_skill="$PWD/skills/git-wtkit/SKILL.md"
export PATH="$tmp:$PATH"
cd "$tmp"
export XDG_CONFIG_HOME="$tmp/config"
skill="$XDG_CONFIG_HOME/opencode/skills/git-wtkit/SKILL.md"
git wtkit install skill
git wtkit install skill
cmp "$expected_skill" "$skill"
test ! -e "$GIT_CONFIG_GLOBAL"
echo customized >"$skill"
if git wtkit install skill; then exit 1; fi
grep -Fx customized "$skill"
git wtkit >help
git wtkit -h >expected
cmp expected help
git wtkit install
git wtkit install
test "$(git config --global --get-all alias.wtsetup)" = 'wtkit setup'
git config --global alias.wtsetup status
if git wtkit install; then exit 1; fi
test "$(git config --global alias.wtsetup)" = status
git config --global --unset alias.wtsetup
git wtkit install
for repo in leaf module project; do
	git init -q "$repo"
	git -C "$repo" commit -qm initial --allow-empty
done
git -C module submodule add -q ../leaf nested
git -C module commit -qam nested
git -C project submodule add -q --name custom ../module 'deps/mod A'
git -C project commit -qam old
git -C project worktree add -q --detach ../old
git -C module commit -qm second --allow-empty
git -C 'project/deps/mod A' pull -q --ff-only
git -C project commit -qam new
git -C project worktree add -q --detach ../new
# First nested checkout has no local source.
(cd project && git wtsetup) >first
grep -Fx 'upstream deps/mod A/nested' first
# The donor lacks the next pin. Reuse its objects and fetch the delta upstream.
git -C module commit -qm third --allow-empty
git -C project worktree add -q --detach ../miss
git -C miss update-index --cacheinfo "160000,$(git -C module rev-parse HEAD),deps/mod A"
(cd miss && git wtkit setup) >log
grep -Fx 'upstream deps/mod A' log
test "$(git -C 'miss/deps/mod A' rev-parse HEAD)" = "$(git -C module rev-parse HEAD)"
mv leaf unavailable-leaf
mv module unavailable-module
for tree in old new; do
	(cd "$tree" && git wtkit setup) >log
	grep -Fx 'local    deps/mod A' log
	grep -Fx 'local    deps/mod A/nested' log
	test "$(git -C "$tree/deps/mod A" rev-parse HEAD)" = "$(git -C "$tree" rev-parse 'HEAD:deps/mod A')"
	test "$(git -C "$tree/deps/mod A" remote get-url origin)" = "$tmp/module"
done
echo dirty >'old/deps/mod A/untracked'
if (cd old && git wtkit setup); then exit 1; fi
test -f 'old/deps/mod A/untracked'
test ! -e 'new/deps/mod A/untracked'
git -C 'new/deps/mod A' switch -qc development HEAD^
before=$(git -C 'new/deps/mod A' rev-parse HEAD)
if (cd new && git wtkit setup); then exit 1; fi
test "$before" = "$(git -C 'new/deps/mod A' rev-parse HEAD)"
git -C 'new/deps/mod A' checkout -q --detach "$(git -C new rev-parse 'HEAD:deps/mod A')"
# Enable once for this repository, then rely on native checkout hooks offline.
hook=project/.git/hooks/post-checkout
echo custom >"$hook"
if (cd project && git wtkit install hook); then exit 1; fi
grep -Fx custom "$hook"
rm "$hook"
(cd project && git wtkit install hook && git wtkit install hook)
git -C project worktree add -q --detach ../automatic HEAD^
sub='automatic/deps/mod A'
test "$(git -C "$sub" rev-parse HEAD)" = "$(git -C old rev-parse 'HEAD:deps/mod A')"
test -e "$sub/nested/.git"
git -C automatic switch -qc automatic-new "$(git -C project rev-parse HEAD)"
test "$(git -C "$sub" rev-parse HEAD)" = "$(git -C new rev-parse 'HEAD:deps/mod A')"
echo keep >"$sub/untracked"
if git -C automatic switch -q --detach HEAD^; then exit 1; fi
test -f "$sub/untracked"
test "$(git -C "$sub" rev-parse HEAD)" = "$(git -C new rev-parse 'HEAD:deps/mod A')"
mv project/.git/modules unavailable-modules
(cd new && git wtkit setup) >log
grep -Fx 'ready    deps/mod A/nested' log
git -C 'new/deps/mod A' fsck --full
