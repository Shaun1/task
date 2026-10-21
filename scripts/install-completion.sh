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
	# Use the first writable directory on the user's real $fpath.
	for dir in $(zsh -ic 'print -rl -- $fpath' 2>/dev/null); do
		if [ -d "$dir" ] && [ -w "$dir" ]; then
			install_to zsh "$dir" _task
			echo "(restart your shell, or run: autoload -U compinit && compinit)"
			exit 0
		fi
	done
	echo "no writable directory on your zsh \$fpath; see: task completion --help"
	;;
*)
	echo "skipping completion: unsupported shell '${SHELL:-unknown}' (see: task completion --help)"
	;;
esac
exit 0
