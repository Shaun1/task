#!/bin/sh
# Install shell completion for the task binary given as $1, for the user's
# login shell ($SHELL). Best effort: prints a note and exits 0 if unsupported.
task=${1:-bin/task}

install_to() { # shell dir file
	mkdir -p "$2" && "$task" completion "$1" > "$2/$3" &&
		echo "installed $1 completion: $2/$3"
}

case "$(basename "${SHELL:-}")" in
bash)
	install_to bash "${XDG_DATA_HOME:-$HOME/.local/share}/bash-completion/completions" task
	echo "(bash completion needs the bash-completion package)"
	;;
fish)
	install_to fish "${XDG_CONFIG_HOME:-$HOME/.config}/fish/completions" task.fish
	;;
zsh)
	# Use the first writable directory on the user's real $fpath. Its first
	# entry often doesn't exist (e.g. /usr/local/share/zsh/site-functions on
	# Apple Silicon, where Homebrew lives in /opt/homebrew), hence the checks.
	for dir in $(zsh -ic 'print -rl -- $fpath' 2>/dev/null); do
		if [ -d "$dir" ] && [ -w "$dir" ]; then
			install_to zsh "$dir" _task
			echo "(restart your shell, or run: autoload -U compinit && compinit)"
			exit 0
		fi
	done
	# None: fall back to a user directory, which ~/.zshrc must add to $fpath.
	install_to zsh "$HOME/.zsh/completions" _task
	echo "add these lines to ~/.zshrc (before any existing compinit), then restart your shell:"
	echo '  fpath=(~/.zsh/completions $fpath)'
	echo '  autoload -U compinit && compinit'
	;;
*)
	echo "skipping completion: unsupported shell '${SHELL:-unknown}' (see: task completion --help)"
	;;
esac
exit 0
