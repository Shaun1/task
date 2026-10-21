# task

Run each task in its own isolated Claude Code sandbox.

`task start "fix login redirect bug"` run from inside a git repo will:

1. read the repo's `origin` URL (fails if not in a git repo / no `origin`)
2. `git clone --branch main --single-branch` it into `~/.task/clones/<repo>/<slug>`
   (a fully self-contained clone; no worktree pointers that break in a sandbox)
3. create and check out branch `<slug>` (`fix-login-redirect-bug`)
4. record state in `~/.task/state/<slug>.json`
5. replace itself with `sbx run claude <clone>` in the current terminal

The sandbox is named `claude-<slug>` (sbx names it `<agent>-<dir name>`).

## Commands

```
task start <description>      # quotes optional
task ls
task clean [name] [--force]   # name inferred from cwd if inside a task clone
```

`clean` stops/removes the sandbox and deletes the clone. It refuses if the clone has
uncommitted changes or commits not on any `origin` branch (the clone is the only copy),
unless `--force`. Push your branch first if you want to keep the work.

`TASK_HOME` overrides the `~/.task` location.

## Build

```
make build    # bin/task
make test
make install
```

No third-party dependencies.

## Known assumptions to verify

- `sbx ls` output format: `internal/sandbox.ParseList` assumes a whitespace table with the
  sandbox name in the first column (optional header row).
- `sbx stop` / `sbx rm` "not found" detection is by error-message substring.
- Sandbox name is predicted as `claude-<clone dir name>` (observed in the spike) rather than read back.
- Git identity / push credentials inside the sandbox are not configured by this tool.

## Layout

```
cmd/task/            CLI entrypoint (stdlib flag, no deps)
internal/naming/     slugify, repo name from URL
internal/gitx/       clone, branch, dirty/unpushed checks (shells out to git)
internal/sandbox/    the only sbx-aware package (interface + SBX impl)
internal/state/      JSON task records
internal/workflow/   start / ls / clean
internal/testutil/   hermetic git helpers for tests
```
