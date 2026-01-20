package restriction

import (
	"testing"
)

func TestChecker_DisabledByDefault(t *testing.T) {
	c := NewChecker()
	if c.IsEnabled() {
		t.Error("expected checker to be disabled by default")
	}

	// Should allow any command when disabled
	allowed, rule := c.Check("rm -rf /")
	if !allowed {
		t.Error("expected command to be allowed when checker is disabled")
	}
	if rule != nil {
		t.Error("expected no rule match when checker is disabled")
	}
}

func TestChecker_EnableDisable(t *testing.T) {
	c := NewChecker()

	c.SetEnabled(true)
	if !c.IsEnabled() {
		t.Error("expected checker to be enabled")
	}

	c.SetEnabled(false)
	if c.IsEnabled() {
		t.Error("expected checker to be disabled")
	}
}

func TestChecker_PrivilegeEscalation(t *testing.T) {
	c := NewChecker()
	c.SetEnabled(true)

	tests := []struct {
		name    string
		cmd     string
		blocked bool
	}{
		// Should be blocked
		{"sudo simple", "sudo ls", true},
		{"sudo with args", "sudo apt-get update", true},
		{"sudo in pipeline", "echo foo | sudo tee /etc/file", true},
		{"sudo after semicolon", "ls; sudo rm file", true},
		{"sudo after &&", "cd /tmp && sudo chmod 777 file", true},
		{"su command", "su -", true},
		{"su with user", "su - root", true},
		{"doas command", "doas ls", true},
		{"pkexec command", "pkexec apt update", true},

		// Should be allowed
		{"sudoers file read", "cat /etc/sudoers", false},
		{"sudo in string", "echo 'use sudo to...'", false},
		{"su in word", "result=success", false},
		{"resume command", "resume", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, rule := c.Check(tt.cmd)
			if tt.blocked && allowed {
				t.Errorf("expected %q to be blocked", tt.cmd)
			}
			if !tt.blocked && !allowed {
				t.Errorf("expected %q to be allowed, but blocked by rule: %s", tt.cmd, rule.Command)
			}
			if tt.blocked && rule == nil {
				t.Error("expected rule to be returned when blocked")
			}
			if tt.blocked && rule != nil && rule.Category != CategoryPrivilegeEscalation {
				t.Errorf("expected category %s, got %s", CategoryPrivilegeEscalation, rule.Category)
			}
		})
	}
}

func TestChecker_DestructiveFileOps(t *testing.T) {
	c := NewChecker()
	c.SetEnabled(true)

	tests := []struct {
		name    string
		cmd     string
		blocked bool
	}{
		// Should be blocked
		{"rm file", "rm file.txt", true},
		{"rm recursive", "rm -rf /tmp/dir", true},
		{"rm with force", "rm -f important.txt", true},
		{"rmdir", "rmdir empty_dir", true},
		{"shred", "shred secret.txt", true},
		{"unlink", "unlink symlink", true},
		{"dd to disk", "dd if=/dev/zero of=/dev/sda", true},
		{"wipe", "wipe -f disk", true},
		{"rm after &&", "ls && rm file", true},
		{"truncate to zero", "truncate -s 0 important.log", true},

		// Edge cases - these require shell parsing beyond simple regex
		// xargs rm is tricky because rm appears after xargs, not at command start
		// We accept this limitation for now
		{"rm in xargs pipeline", "find . -name '*.tmp' | xargs rm", false}, // Not caught - acceptable limitation

		// Should be allowed
		{"mkdir", "mkdir new_dir", false},
		{"touch", "touch new_file", false},
		{"mv file", "mv old.txt new.txt", false},
		{"cp file", "cp source.txt dest.txt", false},
		{"ls", "ls -la", false},
		{"cat", "cat file.txt", false},
		{"grep rm", "grep 'rm' script.sh", false},
		{"rm in string", "echo 'do not rm this'", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, rule := c.Check(tt.cmd)
			if tt.blocked && allowed {
				t.Errorf("expected %q to be blocked", tt.cmd)
			}
			if !tt.blocked && !allowed {
				t.Errorf("expected %q to be allowed, but blocked by rule: %s", tt.cmd, rule.Command)
			}
			if tt.blocked && rule != nil && rule.Category != CategoryDestructiveFile {
				t.Errorf("expected category %s, got %s", CategoryDestructiveFile, rule.Category)
			}
		})
	}
}

func TestChecker_SystemModifications(t *testing.T) {
	c := NewChecker()
	c.SetEnabled(true)

	tests := []struct {
		name    string
		cmd     string
		blocked bool
	}{
		// Should be blocked
		{"chmod", "chmod 755 script.sh", true},
		{"chmod 777", "chmod 777 /var/www", true},
		{"chown", "chown root:root file", true},
		{"chgrp", "chgrp admin file", true},
		{"mkfs", "mkfs /dev/sdb1", true},
		{"mkfs.ext4", "mkfs.ext4 /dev/sdb1", true},
		{"mkfs.xfs", "mkfs.xfs /dev/sdc1", true},
		{"fdisk", "fdisk /dev/sda", true},
		{"mount", "mount /dev/sdb1 /mnt", true},
		{"umount", "umount /mnt", true},
		{"shutdown", "shutdown -h now", true},
		{"reboot", "reboot", true},
		{"poweroff", "poweroff", true},
		{"useradd", "useradd newuser", true},
		{"userdel", "userdel olduser", true},
		{"usermod", "usermod -aG docker user", true},
		{"passwd", "passwd user", true},
		{"systemctl stop", "systemctl stop nginx", true},
		{"systemctl start", "systemctl start docker", true},
		{"service restart", "service apache2 restart", true},
		{"insmod", "insmod module.ko", true},
		{"rmmod", "rmmod module", true},
		{"modprobe", "modprobe driver", true},

		// Should be allowed
		{"ls permissions", "ls -la", false},
		{"stat file", "stat file.txt", false},
		{"id command", "id", false},
		{"whoami", "whoami", false},
		{"systemctl status", "echo 'use systemctl status'", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, rule := c.Check(tt.cmd)
			if tt.blocked && allowed {
				t.Errorf("expected %q to be blocked", tt.cmd)
			}
			if !tt.blocked && !allowed {
				t.Errorf("expected %q to be allowed, but blocked by rule: %s", tt.cmd, rule.Command)
			}
			if tt.blocked && rule != nil && rule.Category != CategorySystemModification {
				t.Errorf("expected category %s, got %s", CategorySystemModification, rule.Category)
			}
		})
	}
}

func TestChecker_EmptyAndWhitespace(t *testing.T) {
	c := NewChecker()
	c.SetEnabled(true)

	tests := []struct {
		name string
		cmd  string
	}{
		{"empty", ""},
		{"whitespace", "   "},
		{"tabs", "\t\t"},
		{"newlines", "\n\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, rule := c.Check(tt.cmd)
			if !allowed {
				t.Errorf("expected empty/whitespace command to be allowed")
			}
			if rule != nil {
				t.Errorf("expected no rule for empty/whitespace")
			}
		})
	}
}

func TestChecker_ComplexCommands(t *testing.T) {
	c := NewChecker()
	c.SetEnabled(true)

	tests := []struct {
		name    string
		cmd     string
		blocked bool
	}{
		// Complex blocked commands
		{"chained with rm", "cd /tmp && rm -rf *", true},
		{"background rm", "rm -rf dir &", true},
		{"redirect with rm", "rm file 2>/dev/null", true},

		// Edge cases - commands inside quotes/subshells require shell parsing
		// We accept this limitation for simple regex-based detection
		{"subshell with sudo", "bash -c 'sudo apt update'", false}, // Not caught - acceptable limitation

		// Complex allowed commands
		{"safe pipeline", "cat file | grep pattern | wc -l", false},
		{"command substitution", "echo $(date)", false},
		{"multiple safe cmds", "pwd && ls && echo done", false},
		{"safe background", "sleep 10 &", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, _ := c.Check(tt.cmd)
			if tt.blocked && allowed {
				t.Errorf("expected %q to be blocked", tt.cmd)
			}
			if !tt.blocked && !allowed {
				t.Errorf("expected %q to be allowed", tt.cmd)
			}
		})
	}
}

func TestCategoryDescription(t *testing.T) {
	tests := []struct {
		cat      Category
		expected string
	}{
		{CategoryDestructiveFile, "Destructive file operation"},
		{CategorySystemModification, "System modification"},
		{CategoryPrivilegeEscalation, "Privilege escalation"},
		{Category("unknown"), "Restricted operation"},
	}

	for _, tt := range tests {
		t.Run(string(tt.cat), func(t *testing.T) {
			desc := CategoryDescription(tt.cat)
			if desc != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, desc)
			}
		})
	}
}
