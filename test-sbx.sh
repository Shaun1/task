set -euo pipefail
SPIKE=$(mktemp -d)/spike && mkdir -p "$SPIKE" && cd "$SPIKE"
echo "$SPIKE"

# 1. throwaway repo with an "origin" so origin/main exists
git init -q --bare ../origin.git
git init -q -b main repo && cd repo
git commit -q --allow-empty -m "init"
git remote add origin ../../origin.git && git push -q origin main

# 2. worktree off origin/main, named like the task slug
git fetch -q origin main
git worktree add -b spike-task "$SPIKE/repo-spike-task" origin/main

# 3. check sbx basics
sbx --help | head -30
sbx run --help
sbx ls
