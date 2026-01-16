package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/scottgl9/thop/internal/session"
)

// runInteractive runs the interactive shell mode
func (a *App) runInteractive() error {
	reader := bufio.NewReader(os.Stdin)

	if !a.quiet {
		fmt.Println("thop - Terminal Hopper for Agents")
		fmt.Println("Type /help for available commands")
		fmt.Println()
	}

	for {
		// Print prompt
		sessionName := a.sessions.GetActiveSessionName()
		prompt := session.FormatPrompt(sessionName)
		fmt.Print(prompt)

		// Read input
		input, err := reader.ReadString('\n')
		if err != nil {
			// EOF (Ctrl+D)
			fmt.Println()
			return nil
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Check for slash commands
		if strings.HasPrefix(input, "/") {
			if err := a.handleSlashCommand(input); err != nil {
				a.outputError(err)
			}
			continue
		}

		// Execute command
		result, err := a.sessions.Execute(input)
		if err != nil {
			a.outputError(err)
			continue
		}

		// Print output
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
	}
}

// handleSlashCommand handles slash commands
func (a *App) handleSlashCommand(input string) error {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "/help", "/h", "/?":
		a.printSlashHelp()
		return nil

	case "/status", "/s":
		return a.printStatus()

	case "/connect", "/c":
		if len(args) == 0 {
			return fmt.Errorf("usage: /connect <session>")
		}
		return a.cmdConnect(args[0])

	case "/switch", "/sw":
		if len(args) == 0 {
			return fmt.Errorf("usage: /switch <session>")
		}
		return a.cmdSwitch(args[0])

	case "/local", "/l":
		return a.cmdSwitch("local")

	case "/close", "/disconnect", "/d":
		if len(args) == 0 {
			return fmt.Errorf("usage: /close <session>")
		}
		return a.cmdClose(args[0])

	case "/sessions", "/list":
		return a.printStatus()

	case "/exit", "/quit", "/q":
		fmt.Println("Goodbye!")
		os.Exit(0)
		return nil

	default:
		return fmt.Errorf("unknown command: %s (use /help for available commands)", cmd)
	}
}

// printSlashHelp prints help for slash commands
func (a *App) printSlashHelp() {
	fmt.Println(`Available commands:
  /connect <session>  Connect to an SSH session
  /switch <session>   Switch to a session
  /local              Switch to local shell (alias for /switch local)
  /status             Show all sessions
  /close <session>    Close an SSH connection
  /help               Show this help
  /exit               Exit thop

Shortcuts:
  /c   = /connect
  /sw  = /switch
  /l   = /local
  /s   = /status
  /d   = /close (disconnect)
  /q   = /exit`)
}

// cmdConnect handles the /connect command
func (a *App) cmdConnect(name string) error {
	if !a.sessions.HasSession(name) {
		return &session.Error{
			Code:    session.ErrSessionNotFound,
			Message: fmt.Sprintf("Session '%s' not found", name),
			Session: name,
		}
	}

	sess, _ := a.sessions.GetSession(name)
	if sess.Type() == "local" {
		fmt.Printf("Session '%s' is local, no connection needed\n", name)
		return nil
	}

	if sess.IsConnected() {
		fmt.Printf("Session '%s' is already connected\n", name)
		return nil
	}

	fmt.Printf("Connecting to %s...\n", name)
	if err := a.sessions.Connect(name); err != nil {
		return err
	}

	fmt.Printf("Connected to %s\n", name)
	return nil
}

// cmdSwitch handles the /switch command
func (a *App) cmdSwitch(name string) error {
	if !a.sessions.HasSession(name) {
		return &session.Error{
			Code:    session.ErrSessionNotFound,
			Message: fmt.Sprintf("Session '%s' not found", name),
			Session: name,
		}
	}

	sess, _ := a.sessions.GetSession(name)

	// For SSH sessions, connect if not connected
	if sess.Type() == "ssh" && !sess.IsConnected() {
		fmt.Printf("Connecting to %s...\n", name)
		if err := a.sessions.Connect(name); err != nil {
			return err
		}
		fmt.Printf("Connected to %s\n", name)
	}

	if err := a.sessions.SetActiveSession(name); err != nil {
		return err
	}

	if !a.quiet {
		fmt.Printf("Switched to %s\n", name)
	}

	return nil
}

// cmdClose handles the /close command
func (a *App) cmdClose(name string) error {
	if !a.sessions.HasSession(name) {
		return &session.Error{
			Code:    session.ErrSessionNotFound,
			Message: fmt.Sprintf("Session '%s' not found", name),
			Session: name,
		}
	}

	sess, _ := a.sessions.GetSession(name)
	if sess.Type() == "local" {
		fmt.Printf("Cannot close local session\n")
		return nil
	}

	if !sess.IsConnected() {
		fmt.Printf("Session '%s' is not connected\n", name)
		return nil
	}

	if err := a.sessions.Disconnect(name); err != nil {
		return err
	}

	fmt.Printf("Disconnected from %s\n", name)

	// Switch to local if we closed the active session
	if a.sessions.GetActiveSessionName() == name {
		a.sessions.SetActiveSession("local")
		fmt.Println("Switched to local")
	}

	return nil
}
