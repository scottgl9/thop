package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/chzyer/readline"
	"github.com/scottgl9/thop/internal/config"
	"github.com/scottgl9/thop/internal/logger"
	"github.com/scottgl9/thop/internal/session"
	"github.com/scottgl9/thop/internal/state"
)

// BackgroundJob represents a command running in the background
type BackgroundJob struct {
	ID        int
	Command   string
	Session   string
	StartTime time.Time
	EndTime   time.Time
	Status    string // "running", "completed", "failed"
	ExitCode  int
	Stdout    string
	Stderr    string
}

// App represents the thop application
type App struct {
	Version   string
	GitCommit string
	BuildTime string

	config       *config.Config
	state        *state.Manager
	sessions     *session.Manager
	configPath   string
	proxyMode    bool
	proxyCommand string // Command to execute in proxy mode (-c flag)
	jsonOutput   bool
	showStatus   bool
	completions  string // Shell name for completions
	verbose      bool
	quiet        bool

	// readline instance for interactive mode (nil when not in interactive mode)
	rl *readline.Instance

	// Background job tracking
	bgJobs   map[int]*BackgroundJob
	bgJobsMu sync.RWMutex
	nextJobID int
}

// NewApp creates a new App instance
func NewApp(version, commit, buildTime string) *App {
	return &App{
		Version:   version,
		GitCommit: commit,
		BuildTime: buildTime,
		bgJobs:    make(map[int]*BackgroundJob),
		nextJobID: 1,
	}
}

// Run runs the application with the given arguments
func (a *App) Run(args []string) error {
	// Parse flags
	if err := a.parseFlags(args); err != nil {
		return err
	}

	// Handle completions before loading config (doesn't need config)
	if a.completions != "" {
		return a.printCompletions(a.completions)
	}

	// Load configuration
	cfg, err := config.Load(a.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	a.config = cfg

	// Initialize logger
	logLevel := cfg.Settings.LogLevel
	if a.verbose {
		logLevel = "debug"
	} else if a.quiet {
		logLevel = "error"
	}

	if err := logger.Init(logger.Config{
		Level:    logLevel,
		FilePath: logger.DefaultLogPath(),
		Enabled:  logLevel != "off" && logLevel != "none",
	}); err != nil {
		// Non-fatal, continue without file logging
		if a.verbose {
			fmt.Fprintf(os.Stderr, "Warning: failed to initialize logger: %v\n", err)
		}
	}

	logger.Info("thop starting, version=%s", a.Version)
	logger.Debug("config loaded from %s", a.configPath)

	// Initialize state manager
	a.state = state.NewManager(cfg.Settings.StateFile)
	if err := a.state.Load(); err != nil {
		// Non-fatal, continue with defaults
		logger.Warn("failed to load state: %v", err)
		if a.verbose {
			fmt.Fprintf(os.Stderr, "Warning: failed to load state: %v\n", err)
		}
	}

	// Initialize session manager
	a.sessions = session.NewManager(cfg, a.state)
	logger.Debug("session manager initialized with %d sessions", len(cfg.Sessions))

	// Handle special flags
	if a.showStatus {
		return a.printStatus()
	}

	// Run in appropriate mode
	if a.proxyMode {
		return a.runProxy()
	}

	return a.runInteractive()
}

// parseFlags parses command line flags
func (a *App) parseFlags(args []string) error {
	flags := flag.NewFlagSet("thop", flag.ContinueOnError)

	var showVersion bool
	var showHelp bool

	flags.BoolVar(&a.proxyMode, "proxy", false, "Run in proxy mode (for AI agents)")
	flags.StringVar(&a.proxyCommand, "c", "", "Execute command (for shell compatibility)")
	flags.BoolVar(&a.showStatus, "status", false, "Show status and exit")
	flags.StringVar(&a.configPath, "config", "", "Path to config file")
	flags.BoolVar(&a.jsonOutput, "json", false, "Output in JSON format")
	flags.StringVar(&a.completions, "completions", "", "Generate shell completions (bash, zsh, fish)")
	flags.BoolVar(&a.verbose, "v", false, "Verbose output")
	flags.BoolVar(&a.verbose, "verbose", false, "Verbose output")
	flags.BoolVar(&a.quiet, "q", false, "Quiet output")
	flags.BoolVar(&a.quiet, "quiet", false, "Quiet output")
	flags.BoolVar(&showVersion, "V", false, "Show version")
	flags.BoolVar(&showVersion, "version", false, "Show version")
	flags.BoolVar(&showHelp, "h", false, "Show help")
	flags.BoolVar(&showHelp, "help", false, "Show help")

	flags.Usage = func() {
		a.printHelp()
	}

	if err := flags.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if showVersion {
		a.printVersion()
		os.Exit(0)
	}

	if showHelp {
		a.printHelp()
		os.Exit(0)
	}

	// If -c is provided, enable proxy mode automatically
	if a.proxyCommand != "" {
		a.proxyMode = true
	}

	return nil
}

// printVersion prints version information
func (a *App) printVersion() {
	fmt.Printf("thop version %s\n", a.Version)
	fmt.Printf("  commit: %s\n", a.GitCommit)
	fmt.Printf("  built:  %s\n", a.BuildTime)
}

// printHelp prints help information
func (a *App) printHelp() {
	fmt.Println(`thop - Terminal Hopper for Agents

USAGE:
    thop [OPTIONS]              Start interactive mode
    thop --proxy                Start proxy mode (for AI agents)
    thop -c "command"           Execute command and exit
    thop --status               Show status and exit

OPTIONS:
    --proxy           Run in proxy mode (SHELL compatible)
    -c <command>      Execute command and exit with its exit code
    --status          Show all sessions and exit
    --config <path>   Use alternate config file
    --json            Output in JSON format
    --completions <s> Generate shell completions (bash, zsh, fish)
    -v, --verbose     Increase logging verbosity
    -q, --quiet       Suppress non-error output
    -h, --help        Print help information
    -V, --version     Print version

INTERACTIVE MODE COMMANDS:
    /connect <session>  Establish SSH connection
    /switch <session>   Change active context
    /local              Switch to local shell
    /status             Show all sessions
    /close <session>    Close SSH connection
    /help               Show commands

EXIT CODES:
    0  Success
    1  General error
    2  Authentication failed
    3  Host key verification failed

EXAMPLES:
    # Start interactive mode
    thop

    # Execute single command
    thop -c "ls -la"

    # Use as shell for AI agent
    SHELL="thop --proxy" claude

    # Check status
    thop --status

SHELL COMPLETIONS:
    # Bash (add to ~/.bashrc)
    eval "$(thop --completions bash)"

    # Zsh (add to ~/.zshrc)
    eval "$(thop --completions zsh)"

    # Fish (save to ~/.config/fish/completions/thop.fish)
    thop --completions fish > ~/.config/fish/completions/thop.fish`)
}

// printStatus prints the status of all sessions
func (a *App) printStatus() error {
	sessions := a.sessions.ListSessions()

	if a.jsonOutput {
		data, err := json.MarshalIndent(sessions, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println("Sessions:")
	for _, s := range sessions {
		status := "disconnected"
		if s.Connected {
			status = "connected"
		}

		active := ""
		if s.Active {
			active = " [active]"
		}

		if s.Type == "ssh" {
			fmt.Printf("  %-12s %s@%s (%s)%s %s\n", s.Name, s.User, s.Host, status, active, s.CWD)
		} else {
			fmt.Printf("  %-12s local (%s)%s %s\n", s.Name, status, active, s.CWD)
		}
	}

	return nil
}

// outputError outputs an error in the appropriate format
func (a *App) outputError(err error) {
	if a.jsonOutput {
		if sessionErr, ok := err.(*session.Error); ok {
			data, _ := json.Marshal(map[string]interface{}{
				"error":      true,
				"code":       sessionErr.Code,
				"message":    sessionErr.Message,
				"session":    sessionErr.Session,
				"host":       sessionErr.Host,
				"retryable":  sessionErr.Retryable,
				"suggestion": sessionErr.Suggestion,
			})
			fmt.Fprintln(os.Stderr, string(data))
		} else {
			data, _ := json.Marshal(map[string]interface{}{
				"error":   true,
				"message": err.Error(),
			})
			fmt.Fprintln(os.Stderr, string(data))
		}
	} else {
		if sessionErr, ok := err.(*session.Error); ok {
			fmt.Fprintf(os.Stderr, "Error: %s\n", sessionErr.Message)
			if sessionErr.Suggestion != "" {
				fmt.Fprintf(os.Stderr, "Suggestion: %s\n", sessionErr.Suggestion)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
}
