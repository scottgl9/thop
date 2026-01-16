package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// runProxy runs the proxy mode for AI agent integration
func (a *App) runProxy() error {
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
			// In proxy mode, continue even on error
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

		// Exit with command's exit code for non-zero exits
		// Note: In proxy mode, we continue processing but could track last exit code
		if result.ExitCode != 0 && a.verbose {
			fmt.Fprintf(os.Stderr, "[exit code: %d]\n", result.ExitCode)
		}
	}
}
