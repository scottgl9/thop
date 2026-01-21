package cli

import (
	"strings"
	"testing"
)

func TestGenerateBashCompletion(t *testing.T) {
	script := GenerateBashCompletion()

	// Check for required elements
	if !strings.Contains(script, "_thop()") {
		t.Error("bash completion should contain _thop function")
	}

	if !strings.Contains(script, "complete -F _thop thop") {
		t.Error("bash completion should contain complete command")
	}

	if !strings.Contains(script, "--proxy") {
		t.Error("bash completion should list --proxy option")
	}

	if !strings.Contains(script, "--config") {
		t.Error("bash completion should list --config option")
	}
}

func TestGenerateZshCompletion(t *testing.T) {
	script := GenerateZshCompletion()

	// Check for required elements
	if !strings.Contains(script, "#compdef thop") {
		t.Error("zsh completion should start with #compdef")
	}

	if !strings.Contains(script, "_thop()") {
		t.Error("zsh completion should contain _thop function")
	}

	if !strings.Contains(script, "--proxy") {
		t.Error("zsh completion should list --proxy option")
	}

	if !strings.Contains(script, "_arguments") {
		t.Error("zsh completion should use _arguments")
	}
}

func TestGenerateFishCompletion(t *testing.T) {
	script := GenerateFishCompletion()

	// Check for required elements
	if !strings.Contains(script, "complete -c thop") {
		t.Error("fish completion should contain complete commands")
	}

	if !strings.Contains(script, "-l proxy") {
		t.Error("fish completion should list --proxy option")
	}

	if !strings.Contains(script, "-l config") {
		t.Error("fish completion should list --config option")
	}
}

func TestPrintCompletions(t *testing.T) {
	app := NewApp("1.0.0", "test", "test")

	tests := []struct {
		shell       string
		shouldError bool
	}{
		{"bash", false},
		{"zsh", false},
		{"fish", false},
		{"unsupported", true},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			err := app.printCompletions(tt.shell)
			if tt.shouldError && err == nil {
				t.Errorf("expected error for shell %s", tt.shell)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error for shell %s: %v", tt.shell, err)
			}
		})
	}
}
