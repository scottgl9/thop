package cli

import (
	"fmt"
)

// GenerateBashCompletion generates bash completion script
func GenerateBashCompletion() string {
	return `# Bash completion for thop

_thop() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    # Main options
    opts="--proxy --status --config --json -v --verbose -q --quiet -h --help -V --version -c"

    # Handle specific options
    case "${prev}" in
        --config)
            COMPREPLY=( $(compgen -f -- "${cur}") )
            return 0
            ;;
        -c)
            # No completion for command argument
            return 0
            ;;
    esac

    # Complete options
    if [[ ${cur} == -* ]]; then
        COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
        return 0
    fi
}

complete -F _thop thop
`
}

// GenerateZshCompletion generates zsh completion script
func GenerateZshCompletion() string {
	return `#compdef thop

# Zsh completion for thop

_thop() {
    local -a opts
    local -a sessions

    opts=(
        '--proxy[Run in proxy mode for AI agents]'
        '-c[Execute command and exit]:command:'
        '--status[Show status and exit]'
        '--config[Use alternate config file]:config file:_files'
        '--json[Output in JSON format]'
        '-v[Verbose output]'
        '--verbose[Verbose output]'
        '-q[Quiet output]'
        '--quiet[Quiet output]'
        '-h[Show help]'
        '--help[Show help]'
        '-V[Show version]'
        '--version[Show version]'
    )

    _arguments -s $opts
}

_thop "$@"
`
}

// GenerateFishCompletion generates fish completion script
func GenerateFishCompletion() string {
	return `# Fish completion for thop

# Main options
complete -c thop -l proxy -d 'Run in proxy mode for AI agents'
complete -c thop -s c -r -d 'Execute command and exit'
complete -c thop -l status -d 'Show status and exit'
complete -c thop -l config -r -F -d 'Use alternate config file'
complete -c thop -l json -d 'Output in JSON format'
complete -c thop -s v -l verbose -d 'Verbose output'
complete -c thop -s q -l quiet -d 'Quiet output'
complete -c thop -s h -l help -d 'Show help'
complete -c thop -s V -l version -d 'Show version'
`
}

// printCompletions prints completions for the specified shell
func (a *App) printCompletions(shell string) error {
	switch shell {
	case "bash":
		fmt.Print(GenerateBashCompletion())
	case "zsh":
		fmt.Print(GenerateZshCompletion())
	case "fish":
		fmt.Print(GenerateFishCompletion())
	default:
		return fmt.Errorf("unsupported shell: %s (use bash, zsh, or fish)", shell)
	}
	return nil
}
