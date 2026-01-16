package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/chzyer/readline"
	"github.com/scottgl9/thop/internal/session"
)

// runInteractive runs the interactive shell mode
func (a *App) runInteractive() error {
	// Set up history file
	historyFile := ""
	if dataDir := os.Getenv("XDG_DATA_HOME"); dataDir != "" {
		historyFile = filepath.Join(dataDir, "thop", "history")
	} else if home, err := os.UserHomeDir(); err == nil {
		historyFile = filepath.Join(home, ".local", "share", "thop", "history")
	}

	// Ensure history directory exists
	if historyFile != "" {
		os.MkdirAll(filepath.Dir(historyFile), 0700)
	}

	// Create completer for slash commands
	completer := readline.NewPrefixCompleter(
		readline.PcItem("/connect",
			readline.PcItemDynamic(a.sessionCompleter()),
		),
		readline.PcItem("/switch",
			readline.PcItemDynamic(a.sessionCompleter()),
		),
		readline.PcItem("/close",
			readline.PcItemDynamic(a.sessionCompleter()),
		),
		readline.PcItem("/local"),
		readline.PcItem("/status"),
		readline.PcItem("/help"),
		readline.PcItem("/exit"),
		readline.PcItem("/c",
			readline.PcItemDynamic(a.sessionCompleter()),
		),
		readline.PcItem("/sw",
			readline.PcItemDynamic(a.sessionCompleter()),
		),
		readline.PcItem("/d",
			readline.PcItemDynamic(a.sessionCompleter()),
		),
		readline.PcItem("/l"),
		readline.PcItem("/s"),
		readline.PcItem("/h"),
		readline.PcItem("/q"),
	)

	// Create readline instance
	rl, err := readline.NewEx(&readline.Config{
		Prompt:            a.getPrompt(),
		HistoryFile:       historyFile,
		AutoComplete:      completer,
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	})
	if err != nil {
		// Fall back to simple mode if readline fails
		return a.runInteractiveSimple()
	}
	defer rl.Close()

	if !a.quiet {
		fmt.Println("thop - Terminal Hopper for Agents")
		fmt.Println("Type /help for available commands")
		fmt.Println()
	}

	for {
		// Update prompt with current session
		rl.SetPrompt(a.getPrompt())

		// Read input
		input, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				// Ctrl+C - clear line and continue
				continue
			}
			if err == io.EOF {
				// Ctrl+D - exit
				fmt.Println()
				return nil
			}
			return err
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

		// Execute command with signal forwarding
		result, err := a.executeWithSignalForwarding(input)
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

// executeWithSignalForwarding executes a command with Ctrl+C forwarding
func (a *App) executeWithSignalForwarding(cmd string) (*session.ExecuteResult, error) {
	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for SIGINT
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)
	defer signal.Stop(sigChan)

	// Run signal handler in goroutine
	go func() {
		select {
		case <-sigChan:
			// Ctrl+C received - cancel the context
			cancel()
		case <-ctx.Done():
			// Context already cancelled
		}
	}()

	return a.sessions.ExecuteWithContext(ctx, cmd)
}

// runInteractiveSimple runs interactive mode without readline (fallback)
func (a *App) runInteractiveSimple() error {
	reader := readline.NewCancelableStdin(os.Stdin)
	defer reader.Close()

	if !a.quiet {
		fmt.Println("thop - Terminal Hopper for Agents")
		fmt.Println("Type /help for available commands")
		fmt.Println()
	}

	buf := make([]byte, 4096)
	for {
		fmt.Print(a.getPrompt())

		n, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF {
				fmt.Println()
				return nil
			}
			return err
		}

		input := strings.TrimSpace(string(buf[:n]))
		if input == "" {
			continue
		}

		if strings.HasPrefix(input, "/") {
			if err := a.handleSlashCommand(input); err != nil {
				a.outputError(err)
			}
			continue
		}

		result, err := a.executeWithSignalForwarding(input)
		if err != nil {
			a.outputError(err)
			continue
		}

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

// getPrompt returns the current prompt string
func (a *App) getPrompt() string {
	sessionName := a.sessions.GetActiveSessionName()
	return session.FormatPrompt(sessionName)
}

// sessionCompleter returns a function that provides session name completions
func (a *App) sessionCompleter() func(string) []string {
	return func(line string) []string {
		return a.sessions.SessionNames()
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

	case "/env":
		return a.cmdEnv(args)

	default:
		return fmt.Errorf("unknown command: %s (use /help for available commands)", cmd)
	}
}

// cmdEnv handles the /env command for setting environment variables
func (a *App) cmdEnv(args []string) error {
	sess := a.sessions.GetActiveSession()
	if sess == nil {
		return fmt.Errorf("no active session")
	}

	// No args - show current environment
	if len(args) == 0 {
		env := sess.GetEnv()
		if len(env) == 0 {
			fmt.Println("No environment variables set")
		} else {
			fmt.Println("Environment variables:")
			for k, v := range env {
				fmt.Printf("  %s=%s\n", k, v)
			}
		}
		return nil
	}

	// Parse KEY=VALUE format
	arg := args[0]
	if idx := strings.Index(arg, "="); idx > 0 {
		key := arg[:idx]
		value := arg[idx+1:]
		// Use session manager to set and persist
		if err := a.sessions.SetSessionEnv(key, value); err != nil {
			return err
		}
		fmt.Printf("Set %s=%s\n", key, value)
		return nil
	}

	return fmt.Errorf("usage: /env [KEY=VALUE]")
}

// printSlashHelp prints help for slash commands
func (a *App) printSlashHelp() {
	fmt.Println(`Available commands:
  /connect <session>  Connect to an SSH session
  /switch <session>   Switch to a session
  /local              Switch to local shell (alias for /switch local)
  /status             Show all sessions
  /close <session>    Close an SSH connection
  /env [KEY=VALUE]    Show or set environment variables
  /help               Show this help
  /exit               Exit thop

Shortcuts:
  /c   = /connect
  /sw  = /switch
  /l   = /local
  /s   = /status
  /d   = /close (disconnect)
  /q   = /exit

Keyboard shortcuts:
  Ctrl+D  Exit
  Ctrl+C  Interrupt running command
  Up/Down History navigation
  Tab     Auto-complete commands`)
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
