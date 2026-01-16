package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/scottgl9/thop/internal/session"
)

// Exit codes for proxy mode
const (
	ExitSuccess      = 0
	ExitGeneralError = 1
	ExitAuthError    = 2
	ExitHostKeyError = 3
)

// ProxyResult represents the result of proxy mode execution
type ProxyResult struct {
	ExitCode int
}

// runProxy runs the proxy mode for AI agent integration
func (a *App) runProxy() error {
	// If a command was provided via -c flag, execute it and exit
	if a.proxyCommand != "" {
		result := a.executeProxyCommand(a.proxyCommand)
		os.Exit(result.ExitCode)
	}

	// Otherwise, read commands from stdin
	return a.runProxyLoop()
}

// executeProxyCommand executes a single command and returns the result
func (a *App) executeProxyCommand(cmd string) *ProxyResult {
	result, err := a.sessions.Execute(cmd)
	if err != nil {
		a.outputError(err)
		return &ProxyResult{ExitCode: a.errorToExitCode(err)}
	}

	// Output results
	if result.Stdout != "" {
		fmt.Print(result.Stdout)
		if !strings.HasSuffix(result.Stdout, "\n") {
			fmt.Println()
		}
	}

	if result.Stderr != "" {
		fmt.Fprint(os.Stderr, result.Stderr)
		if !strings.HasSuffix(result.Stderr, "\n") {
			fmt.Fprintln(os.Stderr)
		}
	}

	return &ProxyResult{ExitCode: result.ExitCode}
}

// runProxyLoop reads commands from stdin in a loop
func (a *App) runProxyLoop() error {
	reader := bufio.NewReader(os.Stdin)

	for {
		// Read command from stdin
		input, err := reader.ReadString('\n')
		if err != nil {
			// EOF - exit cleanly
			return nil
		}

		input = strings.TrimSuffix(input, "\n")
		input = strings.TrimSuffix(input, "\r")

		if input == "" {
			continue
		}

		// Execute command on active session
		result, err := a.sessions.Execute(input)
		if err != nil {
			a.outputError(err)
			// In loop proxy mode, continue even on error
			continue
		}

		// Output results
		if result.Stdout != "" {
			fmt.Print(result.Stdout)
			// Ensure newline at end
			if !strings.HasSuffix(result.Stdout, "\n") {
				fmt.Println()
			}
		}

		if result.Stderr != "" {
			fmt.Fprint(os.Stderr, result.Stderr)
			if !strings.HasSuffix(result.Stderr, "\n") {
				fmt.Fprintln(os.Stderr)
			}
		}

		// Show exit code in verbose mode
		if result.ExitCode != 0 && a.verbose {
			fmt.Fprintf(os.Stderr, "[exit code: %d]\n", result.ExitCode)
		}
	}
}

// errorToExitCode converts an error to an appropriate exit code
func (a *App) errorToExitCode(err error) int {
	if sessionErr, ok := err.(*session.Error); ok {
		switch sessionErr.Code {
		case session.ErrAuthPasswordRequired, session.ErrAuthFailed:
			return ExitAuthError
		case session.ErrHostKeyVerification, session.ErrHostKeyChanged:
			return ExitHostKeyError
		default:
			return ExitGeneralError
		}
	}
	return ExitGeneralError
}
