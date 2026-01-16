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
	"github.com/scottgl9/thop/internal/config"
	"github.com/scottgl9/thop/internal/session"
	"golang.org/x/term"
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
		readline.PcItem("/auth",
			readline.PcItemDynamic(a.sessionCompleter()),
		),
		readline.PcItem("/trust",
			readline.PcItemDynamic(a.sessionCompleter()),
		),
		readline.PcItem("/copy"),
		readline.PcItem("/cp"),
		readline.PcItem("/add-session"),
		readline.PcItem("/add"),
		readline.PcItem("/read"),
		readline.PcItem("/cat"),
		readline.PcItem("/write"),
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

	case "/auth":
		if len(args) == 0 {
			return fmt.Errorf("usage: /auth <session>")
		}
		return a.cmdAuth(args[0])

	case "/trust":
		if len(args) == 0 {
			return fmt.Errorf("usage: /trust <session>")
		}
		return a.cmdTrust(args[0])

	case "/copy", "/cp":
		if len(args) < 2 {
			return fmt.Errorf("usage: /copy <source> <destination>\n  Examples:\n    /copy local:/path/to/file remote:/path/to/file\n    /copy remote:/path/to/file local:/path/to/file\n    /copy myserver:/path/to/file local:/path/to/file")
		}
		return a.cmdCopy(args[0], args[1])

	case "/add-session", "/add":
		if len(args) < 2 {
			return fmt.Errorf("usage: /add-session <name> [user@]host[:port]\n  Example: /add-session myserver user@example.com:22")
		}
		return a.cmdAddSession(args[0], args[1])

	case "/read", "/cat":
		if len(args) < 1 {
			return fmt.Errorf("usage: /read <path>")
		}
		return a.cmdRead(args[0])

	case "/write":
		if len(args) < 1 {
			return fmt.Errorf("usage: /write <path> (content from stdin in proxy mode)")
		}
		return a.cmdWrite(args[0], args[1:])

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
  /auth <session>     Set password for SSH session
  /trust <session>    Trust host key for SSH session
  /copy <src> <dst>   Copy file between sessions (session:path format)
  /add-session <name> <host>  Add new SSH session to config
  /read <path>        Read file contents (from current session)
  /write <path> <content>  Write content to file (on current session)
  /env [KEY=VALUE]    Show or set environment variables
  /help               Show this help
  /exit               Exit thop

Shortcuts:
  /c    = /connect
  /sw   = /switch
  /l    = /local
  /s    = /status
  /d    = /close (disconnect)
  /cp   = /copy
  /add  = /add-session
  /cat  = /read
  /q    = /exit

Copy examples:
  /copy local:/path/file remote:/path/file    Upload to active SSH session
  /copy remote:/path/file local:/path/file    Download from active SSH session
  /copy server1:/path/file server2:/path/file Copy between two SSH sessions

Add session examples:
  /add-session myserver user@example.com      Add SSH session (port 22)
  /add-session prod deploy@prod.server.com:2222  Add with custom port

File access (works on remote sessions via SFTP):
  /read /etc/hostname            Read remote file contents
  /write /tmp/test.txt Hello     Write "Hello" to remote file

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

// cmdAuth handles the /auth command for password authentication
func (a *App) cmdAuth(name string) error {
	if !a.sessions.HasSession(name) {
		return &session.Error{
			Code:    session.ErrSessionNotFound,
			Message: fmt.Sprintf("Session '%s' not found", name),
			Session: name,
		}
	}

	sess, _ := a.sessions.GetSession(name)
	if sess.Type() != "ssh" {
		return fmt.Errorf("session '%s' is not an SSH session", name)
	}

	// Get the SSH session
	sshSess, ok := sess.(*session.SSHSession)
	if !ok {
		return fmt.Errorf("session '%s' is not an SSH session", name)
	}

	// Prompt for password securely (no echo)
	fmt.Printf("Password for %s: ", name)
	password, err := readPassword()
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}
	fmt.Println() // Newline after password input

	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	// Set the password on the session
	sshSess.SetPassword(password)
	fmt.Printf("Password set for %s\n", name)

	// If not connected, offer to connect now
	if !sess.IsConnected() {
		fmt.Printf("Connecting to %s...\n", name)
		if err := a.sessions.Connect(name); err != nil {
			return err
		}
		fmt.Printf("Connected to %s\n", name)
	}

	return nil
}

// readPassword reads a password from stdin without echoing
func readPassword() (string, error) {
	// Use x/term for cross-platform password reading
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	return string(password), nil
}

// cmdCopy handles the /copy command for file transfer between sessions
func (a *App) cmdCopy(src, dst string) error {
	// Parse source and destination (format: session:path or just path for active session)
	srcSession, srcPath := parseFileSpec(src)
	dstSession, dstPath := parseFileSpec(dst)

	// Default to active session if not specified
	activeSession := a.sessions.GetActiveSessionName()
	if srcSession == "" {
		srcSession = activeSession
	}
	if dstSession == "" {
		dstSession = activeSession
	}

	// Handle "remote" as alias for active SSH session
	if srcSession == "remote" {
		if activeSession == "local" {
			return fmt.Errorf("no remote session active - use session name instead")
		}
		srcSession = activeSession
	}
	if dstSession == "remote" {
		if activeSession == "local" {
			return fmt.Errorf("no remote session active - use session name instead")
		}
		dstSession = activeSession
	}

	// Validate sessions exist
	if !a.sessions.HasSession(srcSession) {
		return fmt.Errorf("source session '%s' not found", srcSession)
	}
	if !a.sessions.HasSession(dstSession) {
		return fmt.Errorf("destination session '%s' not found", dstSession)
	}

	srcSess, _ := a.sessions.GetSession(srcSession)
	dstSess, _ := a.sessions.GetSession(dstSession)

	// Handle different transfer scenarios
	if srcSess.Type() == "local" && dstSess.Type() == "local" {
		return fmt.Errorf("both source and destination are local - use regular cp command")
	}

	if srcSess.Type() == "local" && dstSess.Type() == "ssh" {
		// Upload: local -> remote
		sshSess, ok := dstSess.(*session.SSHSession)
		if !ok {
			return fmt.Errorf("destination is not an SSH session")
		}
		if !sshSess.IsConnected() {
			fmt.Printf("Connecting to %s...\n", dstSession)
			if err := a.sessions.Connect(dstSession); err != nil {
				return err
			}
		}
		fmt.Printf("Uploading %s to %s:%s...\n", srcPath, dstSession, dstPath)
		if err := sshSess.UploadFile(srcPath, dstPath); err != nil {
			return err
		}
		fmt.Printf("Upload complete\n")
		return nil
	}

	if srcSess.Type() == "ssh" && dstSess.Type() == "local" {
		// Download: remote -> local
		sshSess, ok := srcSess.(*session.SSHSession)
		if !ok {
			return fmt.Errorf("source is not an SSH session")
		}
		if !sshSess.IsConnected() {
			fmt.Printf("Connecting to %s...\n", srcSession)
			if err := a.sessions.Connect(srcSession); err != nil {
				return err
			}
		}
		fmt.Printf("Downloading %s:%s to %s...\n", srcSession, srcPath, dstPath)
		if err := sshSess.DownloadFile(srcPath, dstPath); err != nil {
			return err
		}
		fmt.Printf("Download complete\n")
		return nil
	}

	if srcSess.Type() == "ssh" && dstSess.Type() == "ssh" {
		// Remote to remote: download then upload
		srcSSH, _ := srcSess.(*session.SSHSession)
		dstSSH, _ := dstSess.(*session.SSHSession)

		// Connect both if needed
		if !srcSSH.IsConnected() {
			fmt.Printf("Connecting to %s...\n", srcSession)
			if err := a.sessions.Connect(srcSession); err != nil {
				return err
			}
		}
		if !dstSSH.IsConnected() {
			fmt.Printf("Connecting to %s...\n", dstSession)
			if err := a.sessions.Connect(dstSession); err != nil {
				return err
			}
		}

		// Read from source
		fmt.Printf("Reading %s:%s...\n", srcSession, srcPath)
		data, err := srcSSH.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("failed to read from %s: %w", srcSession, err)
		}

		// Write to destination
		fmt.Printf("Writing to %s:%s...\n", dstSession, dstPath)
		if err := dstSSH.WriteFile(dstPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write to %s: %w", dstSession, err)
		}

		fmt.Printf("Copy complete (%d bytes)\n", len(data))
		return nil
	}

	return fmt.Errorf("unsupported copy operation")
}

// cmdRead handles the /read command to read and output file contents
func (a *App) cmdRead(path string) error {
	sess := a.sessions.GetActiveSession()
	if sess == nil {
		return fmt.Errorf("no active session")
	}

	if sess.Type() == "local" {
		// Read local file
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		fmt.Print(string(data))
		return nil
	}

	// Read remote file via SFTP
	sshSess, ok := sess.(*session.SSHSession)
	if !ok {
		return fmt.Errorf("session is not an SSH session")
	}

	if !sshSess.IsConnected() {
		return fmt.Errorf("session is not connected")
	}

	data, err := sshSess.ReadFile(path)
	if err != nil {
		return err
	}

	fmt.Print(string(data))
	return nil
}

// cmdWrite handles the /write command to write content to a file
func (a *App) cmdWrite(path string, content []string) error {
	sess := a.sessions.GetActiveSession()
	if sess == nil {
		return fmt.Errorf("no active session")
	}

	// If content provided as arguments, join them
	var data string
	if len(content) > 0 {
		data = strings.Join(content, " ")
	} else {
		// In interactive mode, we can't easily read from stdin
		// This is mainly useful in proxy mode where content comes via arguments
		return fmt.Errorf("usage: /write <path> <content>")
	}

	if sess.Type() == "local" {
		// Write local file
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		fmt.Printf("Wrote %d bytes to %s\n", len(data), path)
		return nil
	}

	// Write remote file via SFTP
	sshSess, ok := sess.(*session.SSHSession)
	if !ok {
		return fmt.Errorf("session is not an SSH session")
	}

	if !sshSess.IsConnected() {
		return fmt.Errorf("session is not connected")
	}

	if err := sshSess.WriteFile(path, []byte(data), 0644); err != nil {
		return err
	}

	fmt.Printf("Wrote %d bytes to %s\n", len(data), path)
	return nil
}

// cmdAddSession handles the /add-session command to add a new SSH session
func (a *App) cmdAddSession(name, hostSpec string) error {
	// Check if session already exists
	if a.sessions.HasSession(name) {
		return fmt.Errorf("session '%s' already exists", name)
	}

	// Parse host specification: [user@]host[:port]
	user, host, port := parseHostSpec(hostSpec)

	// Create session config
	sessionCfg := config.Session{
		Type: "ssh",
		Host: host,
		User: user,
		Port: port,
	}

	// Add to session manager
	if err := a.sessions.AddSession(name, sessionCfg); err != nil {
		return err
	}

	// Add to config and save
	cfg := a.sessions.GetConfig()
	if err := cfg.AddSession(name, sessionCfg); err != nil {
		return err
	}

	if err := cfg.Save(""); err != nil {
		return fmt.Errorf("session added but failed to save config: %w", err)
	}

	fmt.Printf("Added SSH session '%s' (%s@%s", name, user, host)
	if port != 0 && port != 22 {
		fmt.Printf(":%d", port)
	}
	fmt.Println(")")
	fmt.Printf("Config saved to %s\n", config.DefaultConfigPath())
	return nil
}

// parseHostSpec parses a host specification in the format [user@]host[:port]
func parseHostSpec(spec string) (user, host string, port int) {
	// Default values
	port = 22

	// Check for user@
	if idx := strings.Index(spec, "@"); idx != -1 {
		user = spec[:idx]
		spec = spec[idx+1:]
	}

	// Check for :port
	if idx := strings.LastIndex(spec, ":"); idx != -1 {
		host = spec[:idx]
		if p, err := fmt.Sscanf(spec[idx+1:], "%d", &port); err != nil || p != 1 {
			port = 22
		}
	} else {
		host = spec
	}

	// Default user if not specified
	if user == "" {
		// Try to get current user
		if currentUser := os.Getenv("USER"); currentUser != "" {
			user = currentUser
		} else {
			user = "root"
		}
	}

	return user, host, port
}

// parseFileSpec parses a file specification in the format "session:path" or just "path"
func parseFileSpec(spec string) (session, path string) {
	// Handle Windows-style paths (C:\...) by checking if it looks like a drive letter
	if len(spec) >= 2 && spec[1] == ':' && (spec[0] >= 'A' && spec[0] <= 'Z' || spec[0] >= 'a' && spec[0] <= 'z') {
		// Windows absolute path
		return "", spec
	}

	// Look for session:path format
	idx := strings.Index(spec, ":")
	if idx > 0 {
		return spec[:idx], spec[idx+1:]
	}

	// Just a path, no session specified
	return "", spec
}

// cmdTrust handles the /trust command for host key verification
func (a *App) cmdTrust(name string) error {
	if !a.sessions.HasSession(name) {
		return &session.Error{
			Code:    session.ErrSessionNotFound,
			Message: fmt.Sprintf("Session '%s' not found", name),
			Session: name,
		}
	}

	sess, _ := a.sessions.GetSession(name)
	if sess.Type() != "ssh" {
		return fmt.Errorf("session '%s' is not an SSH session", name)
	}

	// Get the SSH session
	sshSess, ok := sess.(*session.SSHSession)
	if !ok {
		return fmt.Errorf("session '%s' is not an SSH session", name)
	}

	// Fetch the host key and fingerprint
	fmt.Printf("Fetching host key from %s:%d...\n", sshSess.Host(), sshSess.Port())
	keyType, fingerprint, err := sshSess.FetchHostKey()
	if err != nil {
		return fmt.Errorf("failed to fetch host key: %w", err)
	}

	// Display the fingerprint and ask for confirmation
	fmt.Printf("\nHost key for %s:\n", name)
	fmt.Printf("  Type:        %s\n", keyType)
	fmt.Printf("  Fingerprint: %s\n", fingerprint)
	fmt.Printf("\nAre you sure you want to trust this host? (yes/no): ")

	var answer string
	_, err = fmt.Scanln(&answer)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "yes" && answer != "y" {
		fmt.Println("Host key not trusted.")
		return nil
	}

	// Add the host key to known_hosts
	if err := sshSess.AddHostKey(); err != nil {
		return fmt.Errorf("failed to add host key: %w", err)
	}

	fmt.Printf("Host key added to known_hosts for %s\n", name)
	return nil
}
