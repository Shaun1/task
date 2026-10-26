# task — project context for Claude Code

`task` is a Go CLI that runs each task in its own isolated Claude Code sandbox: fresh clone of the repo's `origin/main` → new branch → opened in an `sbx` sandbox, with easy cleanup. See README.md for usage.

## Status
- Unit-tested (`make vet test build`); module name `task`; dependencies: cobra, golang.org/x/term v0.47.0; `go.mod` targets `go 1.26.1`.
- Verified against real `sbx` v0.49.1: `new` (then `start`) launched a sandbox (before `--name` was added; re-check it), `ls` and `rm` (then named `clean`) worked end to end.

## CLI (v1, deliberately small)
```
task new <description>      # quotes optional
task open [name] [--shell | --sbx | --ide]   # new Claude session (default), host shell, sandbox shell, or IDE
task ls
task rm [name] [--force]      # name inferred from cwd if inside a task clone
```
No `--repo`, `--branch`, `--base`, `--prefix`, `--keep-branch` or `--delete-branch` flags. Do not add them without asking: branch = slug of the description, base = `main`, repo = git root of cwd (fail if not a git repo or no `origin`). User settings live in `$TASK_HOME/config.json` (currently only `ide`); a branch prefix is planned for a later version. `--ide` deliberately takes no value: the IDE is configured globally (user's choice), and an optional-value flag would make `--ide pycharm` parse `pycharm` as the task name.

## Behavior
`new`: find git root → read `origin` URL → slugify description (lowercase, non-alphanumerics → `-`, collapsed, max 40 chars) → print warnings (never fatal; GitHub origins only) if the host SSH agent has no keys or isn't reachable (SSH origins) or there's no global `github` sbx secret → `git clone --branch main --single-branch <origin> ~/.task/clones/<repo>/<slug>` → widen `remote.origin.fetch` to all branches + `push.autoSetupRemote=true` (single-branch refspec meant pushed branches got no `origin/<branch>` ref, which broke `gh pr create`) → `git checkout -b <slug>` → copy `user.name`/`user.email` (as resolved in the source repo) into the clone's local config → save `~/.task/state/<slug>.json` → `sbx create --name claude-<slug> -e GIT_SSH_COMMAND=… claude <clone>` → `sbx policy allow network --sandbox claude-<slug> github.com:22` → `syscall.Exec` into `sbx run --name claude-<slug>` in the current terminal. Rolls back clone + state if launch fails. Fails early if the task, clone dir or sandbox name already exists. `TASK_HOME` overrides `~/.task`.

`open`: resolve task (name or cwd) → require the clone dir → default: check the sandbox still exists (`sbx run --name` on a missing one would create a new sandbox on cwd), then `syscall.Exec` into `sbx run --name <sandbox>`; each such run starts a separate agent process in the same sandbox (verified) → `--shell`: chdir to the clone and exec `$SHELL` → `--sbx`: same sandbox-exists check, then exec `sbx exec -it -e GIT_SSH_COMMAND=… -w <clone> <sandbox> bash` (the clone is mounted at the same path inside; `create -e` env doesn't reach `exec`, so it's passed again) → `--ide`: run `sh -c '<ide> "$1"' sh <clone>` with JetBrains Toolbox's scripts dir appended to PATH; if `ide` isn't configured: when stdin is a terminal (`x/term`; a char-device check is wrong because `/dev/null` is one), prompt with the first installed known launcher as default, save to `config.json`, then open; otherwise error.

`rm`: resolve task (name or cwd) → refuse unless `--force` if the clone has uncommitted/untracked changes or commits not on any `origin` branch (uses `git ls-remote`, because single-branch clones don't track branches pushed later) → `sbx rm --force` (stops too; ignore not-found) → delete clone (only if under `~/.task/clones`) → delete state file. The clone is the only copy of unpushed work, so the safety check matters.

## Key decisions (don't relitigate without reason)
- Target the standalone **`sbx`** CLI. Docker Desktop's `docker sandbox` is deprecated.
- **Full independent clones, not git worktrees.** Spike result: inside the sandbox `git status` fails on a worktree because its `.git` pointer references a parent repo that isn't mounted. Never use `git clone --shared` or `--reference` — they recreate the same dangling-pointer problem. Rejected alternatives: mounting the main repo too (breaks isolation), `sbx run --clone` (unverified; changes would live only in the sandbox).
- No `--keep-branch`/`--delete-branch`: the branch exists only inside the clone, so it's deleted with it. Push first to keep work. A `--delete-remote` flag was considered and left out of v1.
- Sandbox name is `claude-<slug>` (sbx's own default form), passed explicitly with `sbx run --name`.

## Layout
```
cmd/task/main.go        CLI commands (cobra; `completion` and task-name completion for `open`/`rm` come with it)
cmd/task/version.go     `--version`: ldflags `main.version` (set by the Makefile), else VCS info from build metadata
internal/naming/        slugify, repo name from URL
internal/gitx/          clone, branch, dirty/unpushed checks (shells out to git)
internal/sandbox/       the ONLY package that knows about sbx: interface + SBX impl
internal/host/          host-side processes for `open`: shell, IDE
internal/proc/          Exec: hand the terminal to another program (syscall.Exec; child process on Windows)
internal/config/        TASK_HOME paths + config.json settings
internal/state/         JSON task records
internal/workflow/      new / open / ls / rm (tested with a fake sandbox)
internal/testutil/      hermetic git helpers (isolated from global git config)
```
Keep sbx-specific details inside `internal/sandbox`. Sandbox launches `claude --dangerously-skip-permissions` itself. Tests use real git against throwaway local repos and a fake `Sandbox`.

## sbx facts (verified, v0.49.1)
- `sbx ls` table output gets an update banner appended, so parse `sbx ls -q` (one name per line) instead. `--json` is also available.
- `sbx rm` asks for confirmation unless `--force` is passed. It also stops the sandbox.
- Not-found error: `error: sandbox '<name>' not found` (exit 1).
- `sbx run` supports `--name`, `-e KEY=VAL` (env for the agent session) and `-- <agent args>`.
- Git over SSH works in the sandbox: sbx forwards the host SSH agent by default (`ssh.agentForwardingEnabled`; the sandbox sees `SSH_AUTH_SOCK=/run/ssh-agent.sock` and can sign, but the key never enters it). Port 22 is blocked by default, so `new` adds a sandbox-scoped `github.com:22` rule. sbx only accepts that rule for an existing sandbox, hence `create` then `run --name`. The rule is deleted with the sandbox. The sandbox has no `known_hosts`, so `GIT_SSH_COMMAND` (baked in via `create -e`) uses `StrictHostKeyChecking=accept-new`.
- `gh`/GitHub API calls need the `github` service secret: `sbx secret set github --command 'gh auth token'` (one-time, global, user-run; resolved on the host). The sandbox gets a placeholder `GH_TOKEN` that the proxy swaps for the real token. Verified: `gh auth status` and `gh api user` work inside the sandbox, and a running sandbox picks up a newly set secret without a restart. Without the secret, `gh` reports the token as invalid.
- Rejected alternative for git push: HTTPS through the sbx proxy with `sbx secret set github` plus an SSH→HTTPS `insteadOf` rewrite. It avoids opening port 22 and forwarding the SSH agent, but needs a URL rewrite. The user chose SSH. Now that the `github` secret is needed anyway for `gh`, it is the simpler option if SSH ever causes trouble.

## Still unverified
- Whether `sbx run claude <dir> -- "<prompt>"` pre-seeds the task description.
- A full `task new` → push from inside the sandbox (each step verified separately with `sbx create`/`exec`).

## Next steps
1. Verify pushing from inside a sandbox started by `task new`.
2. Optional: non-GitHub SSH remotes aren't handled (only `github.com:22` is opened).
3. Optional: `--recurse-submodules` if repos use submodules, initial prompt seeding, `--all-merged` cleanup, branch prefix.
