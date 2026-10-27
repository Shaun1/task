# task

Run each task in its own isolated Claude Code sandbox.

`task new "fix login redirect bug"` run from inside a git repo will:

1. read the repo's `origin` URL (fails if not in a git repo / no `origin`)
2. `git clone --branch main --single-branch` it into `~/.task/clones/<repo>/<slug>`
   (a fully self-contained clone; no worktree pointers that break in a sandbox)
3. create and check out branch `<slug>` (`fix-login-redirect-bug`)
4. copy your `user.name`/`user.email` into the clone's local git config
5. record state in `~/.task/state/<slug>.json`
6. create sandbox `claude-<slug>` for the clone, allow SSH to github.com for it, and
   replace itself with `sbx run --name claude-<slug>` in the current terminal

## Commands

```
task new <description>      # quotes optional
task do [name]              # another Claude session in the task's sandbox
task do [name] --shell      # your $SHELL on this machine, in the clone
task do [name] --sbx        # a bash shell inside the task's sandbox, in the clone
task do [name] --ide        # your configured IDE, on the clone
task ls
task rm [name] [--force]    # name inferred from cwd if inside a task clone
```

`rm` removes the sandbox and deletes the clone. It refuses if the clone has
uncommitted changes or commits not on any `origin` branch (the clone is the only copy),
unless `--force`. Push your branch first if you want to keep the work.

`TASK_HOME` overrides the `~/.task` location.

### Configuration

`~/.task/config.json` (optional). The first `task do --ide` asks for your IDE, suggesting
launchers it finds installed, and saves your answer here:

```json
{"ide": "goland"}
```

`ide` is a shell command; `task do --ide` runs it with the clone path appended, so
`"open -a 'IntelliJ IDEA'"` or `"code"` also work. JetBrains Toolbox launchers (`goland`,
`pycharm`, …) are found even if Toolbox's scripts folder isn't on your `PATH`.

## Shell completion

Completes subcommands, flags, and task names for `do` and `rm`. Install the script once:

```
# bash (needs the bash-completion package)
task completion bash > ~/.local/share/bash-completion/completions/task

# zsh (Intel and Apple Silicon Macs, Linux)
mkdir -p ~/.zsh/completions
task completion zsh > ~/.zsh/completions/_task

# fish
task completion fish > ~/.config/fish/completions/task.fish
```

For zsh, also add these lines to `~/.zshrc` once (before any existing `compinit`, e.g. above
Oh My Zsh), then restart the shell:

```
fpath=(~/.zsh/completions $fpath)
autoload -U compinit && compinit
```

Don't write to `"${fpath[1]}"`: on Apple Silicon Macs it is usually
`/usr/local/share/zsh/site-functions`, which doesn't exist there (Homebrew uses `/opt/homebrew`),
and the system directories after it aren't writable.

Other shells: `task completion --help`.

## Build

```
make build    # bin/task
make test
make install  # also installs completion for your $SHELL (bash, zsh or fish)
```

`task --version` prints the build: the `git describe` output when built with `make`, or
`dev-<commit>` (with `-dirty` for uncommitted changes) for a plain `go build`.

Publishing a GitHub release runs `.github/workflows/release.yml`, which tests, builds a
macOS Apple Silicon binary and attaches `task_<tag>_darwin_arm64.tar.gz` (plus a
`.sha256`) to the release. To install it:

```
curl -fsSL https://github.com/Shaun1/task/releases/download/<tag>/task_<tag>_darwin_arm64.tar.gz | tar -xz -C /usr/local/bin task
```

A copy downloaded with a browser is quarantined by Gatekeeper; clear that with
`xattr -d com.apple.quarantine task`.

Dependencies: [cobra](https://github.com/spf13/cobra) and
[golang.org/x/term](https://pkg.go.dev/golang.org/x/term).

## Pushing from the sandbox

Git over SSH to GitHub works inside the sandbox with no setup, as long as your key is
loaded in your SSH agent (`ssh-add -l`). sbx forwards the agent, so the sandbox can sign
but never sees the key. `task new` opens port 22 to `github.com` for that sandbox only.

`gh` and other GitHub API calls (e.g. opening a PR) go over HTTPS and need a token. Give
sbx one once; its proxy adds it to requests, so the token never enters the sandbox:

```
sbx secret set github --command 'gh auth token'
```

## Layout

```
cmd/task/            CLI entrypoint (cobra)
internal/config/     TASK_HOME paths and config.json
internal/naming/     slugify, repo name from URL
internal/gitx/       clone, branch, dirty/unpushed checks (shells out to git)
internal/sandbox/    the only sbx-aware package (interface + SBX impl)
internal/host/       shell / IDE on the host for `task do`
internal/proc/       hand the terminal over to another program
internal/state/      JSON task records
internal/workflow/   new / do / ls / rm
internal/testutil/   hermetic git helpers for tests
```
