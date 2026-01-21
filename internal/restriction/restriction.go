// Package restriction provides command filtering for restricted mode
// to prevent AI agents from executing dangerous/destructive operations.
package restriction

import (
	"regexp"
	"strings"
)

// Category represents a category of restricted commands
type Category string

const (
	CategoryDestructiveFile     Category = "destructive_file"
	CategorySystemModification  Category = "system_modification"
	CategoryPrivilegeEscalation Category = "privilege_escalation"
)

// Rule defines a restriction rule
type Rule struct {
	Pattern     *regexp.Regexp
	Category    Category
	Description string
	Command     string // Original command name for error messages
}

// Checker validates commands against restriction rules
type Checker struct {
	rules   []Rule
	enabled bool
}

// NewChecker creates a new restriction checker
func NewChecker() *Checker {
	return &Checker{
		rules:   buildDefaultRules(),
		enabled: false,
	}
}

// SetEnabled enables or disables restriction checking
func (c *Checker) SetEnabled(enabled bool) {
	c.enabled = enabled
}

// IsEnabled returns whether restriction checking is enabled
func (c *Checker) IsEnabled() bool {
	return c.enabled
}

// Check validates a command against restriction rules.
// Returns (allowed bool, rule *Rule) - if not allowed, rule contains the matched rule.
func (c *Checker) Check(cmd string) (bool, *Rule) {
	if !c.enabled {
		return true, nil
	}

	// Normalize the command (trim whitespace)
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return true, nil
	}

	// Check against all rules
	for i := range c.rules {
		if c.rules[i].Pattern.MatchString(cmd) {
			return false, &c.rules[i]
		}
	}

	return true, nil
}

// buildDefaultRules creates the default set of restriction rules
func buildDefaultRules() []Rule {
	rules := []Rule{}

	// Privilege escalation commands (sudo, su, doas)
	rules = append(rules, buildPrivilegeEscalationRules()...)

	// Destructive file operations
	rules = append(rules, buildDestructiveFileRules()...)

	// System modification commands
	rules = append(rules, buildSystemModificationRules()...)

	return rules
}

// buildPrivilegeEscalationRules creates rules for privilege escalation commands
func buildPrivilegeEscalationRules() []Rule {
	commands := []struct {
		name string
		desc string
	}{
		{"sudo", "execute commands with superuser privileges"},
		{"su", "switch user identity"},
		{"doas", "execute commands as another user"},
		{"pkexec", "execute commands as another user via PolicyKit"},
	}

	rules := make([]Rule, 0, len(commands))
	for _, cmd := range commands {
		// Match command at start of line, or after pipe/semicolon/&&/||
		// Handles: "sudo ...", "echo foo | sudo ...", "cmd && sudo ...", etc.
		pattern := regexp.MustCompile(`(?:^|[|;&])\s*` + regexp.QuoteMeta(cmd.name) + `(?:\s|$)`)
		rules = append(rules, Rule{
			Pattern:     pattern,
			Category:    CategoryPrivilegeEscalation,
			Description: cmd.desc,
			Command:     cmd.name,
		})
	}

	return rules
}

// buildDestructiveFileRules creates rules for destructive file operations
func buildDestructiveFileRules() []Rule {
	commands := []struct {
		name string
		desc string
	}{
		{"rm", "remove files or directories"},
		{"rmdir", "remove empty directories"},
		{"shred", "securely delete files"},
		{"wipe", "securely erase files"},
		{"srm", "secure remove"},
		{"unlink", "remove files"},
		// Dangerous dd operations (can overwrite disks)
		{"dd", "copy and convert files (can overwrite disks)"},
	}

	rules := make([]Rule, 0, len(commands))
	for _, cmd := range commands {
		pattern := regexp.MustCompile(`(?:^|[|;&])\s*` + regexp.QuoteMeta(cmd.name) + `(?:\s|$)`)
		rules = append(rules, Rule{
			Pattern:     pattern,
			Category:    CategoryDestructiveFile,
			Description: cmd.desc,
			Command:     cmd.name,
		})
	}

	// Special case: truncate with size 0 (destructive)
	rules = append(rules, Rule{
		Pattern:     regexp.MustCompile(`(?:^|[|;&])\s*truncate\s+.*-s\s*0`),
		Category:    CategoryDestructiveFile,
		Description: "truncate files to zero size",
		Command:     "truncate",
	})

	// Special case: > file (redirecting nothing to file, truncates it)
	rules = append(rules, Rule{
		Pattern:     regexp.MustCompile(`(?:^|[|;&])\s*>\s*\S`),
		Category:    CategoryDestructiveFile,
		Description: "truncate file via redirect",
		Command:     "> redirect",
	})

	return rules
}

// buildSystemModificationRules creates rules for system modification commands
func buildSystemModificationRules() []Rule {
	commands := []struct {
		name string
		desc string
	}{
		// Permission/ownership changes
		{"chmod", "change file permissions"},
		{"chown", "change file ownership"},
		{"chgrp", "change file group ownership"},
		{"chattr", "change file attributes"},

		// Disk/filesystem operations
		{"fdisk", "partition table manipulator"},
		{"parted", "partition editor"},
		{"mount", "mount filesystems"},
		{"umount", "unmount filesystems"},
		{"fsck", "filesystem check and repair"},

		// System control
		{"shutdown", "shutdown the system"},
		{"reboot", "reboot the system"},
		{"poweroff", "power off the system"},
		{"halt", "halt the system"},
		{"init", "change runlevel"},

		// User/group management
		{"useradd", "create user accounts"},
		{"userdel", "delete user accounts"},
		{"usermod", "modify user accounts"},
		{"groupadd", "create groups"},
		{"groupdel", "delete groups"},
		{"groupmod", "modify groups"},
		{"passwd", "change user password"},

		// Service management (could disrupt services)
		{"systemctl", "control systemd services"},
		{"service", "control system services"},

		// Kernel/module operations
		{"insmod", "insert kernel module"},
		{"rmmod", "remove kernel module"},
		{"modprobe", "add/remove kernel modules"},

		// SELinux/AppArmor
		{"setenforce", "modify SELinux mode"},
		{"aa-enforce", "set AppArmor profile to enforce"},
		{"aa-complain", "set AppArmor profile to complain"},
	}

	rules := make([]Rule, 0, len(commands)+1)
	for _, cmd := range commands {
		pattern := regexp.MustCompile(`(?:^|[|;&])\s*` + regexp.QuoteMeta(cmd.name) + `(?:\s|$)`)
		rules = append(rules, Rule{
			Pattern:     pattern,
			Category:    CategorySystemModification,
			Description: cmd.desc,
			Command:     cmd.name,
		})
	}

	// Special case: mkfs and variants (mkfs.ext4, mkfs.xfs, etc.)
	rules = append(rules, Rule{
		Pattern:     regexp.MustCompile(`(?:^|[|;&])\s*mkfs(?:\.\w+)?(?:\s|$)`),
		Category:    CategorySystemModification,
		Description: "create filesystem (formats disk)",
		Command:     "mkfs",
	})

	return rules
}

// CategoryDescription returns a human-readable description for a category
func CategoryDescription(cat Category) string {
	switch cat {
	case CategoryDestructiveFile:
		return "Destructive file operation"
	case CategorySystemModification:
		return "System modification"
	case CategoryPrivilegeEscalation:
		return "Privilege escalation"
	default:
		return "Restricted operation"
	}
}
