//! Command restriction module for blocking dangerous/destructive operations.
//! 
//! This module provides a `Checker` that validates commands against a set of
//! restriction rules, preventing AI agents from executing dangerous commands
//! like `rm -rf`, `sudo`, etc.

use regex::Regex;
use std::sync::atomic::{AtomicBool, Ordering};

/// Category of restricted commands
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Category {
    /// Commands that escalate privileges (sudo, su, doas)
    PrivilegeEscalation,
    /// Commands that delete files (rm, rmdir, shred)
    DestructiveFile,
    /// Commands that modify system configuration (chmod, mount, etc.)
    SystemModification,
}

impl Category {
    /// Get a human-readable description for this category
    pub fn description(&self) -> &'static str {
        match self {
            Category::PrivilegeEscalation => "Privilege escalation",
            Category::DestructiveFile => "Destructive file operation",
            Category::SystemModification => "System modification",
        }
    }
}

/// A restriction rule that matches dangerous commands
pub struct Rule {
    pattern: Regex,
    category: Category,
    command: String,
    #[allow(dead_code)]
    description: String,
}

impl Rule {
    fn new(pattern: &str, category: Category, command: &str, description: &str) -> Self {
        Self {
            pattern: Regex::new(pattern).expect("Invalid regex pattern"),
            category,
            command: command.to_string(),
            description: description.to_string(),
        }
    }
}

/// Result of checking a command against restriction rules
pub struct CheckResult<'a> {
    pub allowed: bool,
    pub rule: Option<&'a Rule>,
}

impl<'a> CheckResult<'a> {
    /// Get the command name that was blocked
    pub fn command(&self) -> Option<&str> {
        self.rule.map(|r| r.command.as_str())
    }

    /// Get the category of the blocked command
    pub fn category(&self) -> Option<Category> {
        self.rule.map(|r| r.category)
    }
}

/// Checker validates commands against restriction rules
pub struct Checker {
    rules: Vec<Rule>,
    enabled: AtomicBool,
}

impl Default for Checker {
    fn default() -> Self {
        Self::new()
    }
}

impl Checker {
    /// Create a new restriction checker with default rules
    pub fn new() -> Self {
        let mut rules = Vec::new();
        
        // Add privilege escalation rules
        rules.extend(build_privilege_escalation_rules());
        
        // Add destructive file operation rules
        rules.extend(build_destructive_file_rules());
        
        // Add system modification rules
        rules.extend(build_system_modification_rules());
        
        Self {
            rules,
            enabled: AtomicBool::new(false),
        }
    }

    /// Enable or disable restriction checking
    pub fn set_enabled(&self, enabled: bool) {
        self.enabled.store(enabled, Ordering::SeqCst);
    }

    /// Check if restriction checking is enabled
    pub fn is_enabled(&self) -> bool {
        self.enabled.load(Ordering::SeqCst)
    }

    /// Check if a command is allowed
    /// 
    /// Returns a `CheckResult` indicating whether the command is allowed
    /// and which rule blocked it (if any).
    pub fn check(&self, cmd: &str) -> CheckResult<'_> {
        if !self.is_enabled() {
            return CheckResult { allowed: true, rule: None };
        }

        let cmd = cmd.trim();
        if cmd.is_empty() {
            return CheckResult { allowed: true, rule: None };
        }

        for rule in &self.rules {
            if rule.pattern.is_match(cmd) {
                return CheckResult {
                    allowed: false,
                    rule: Some(rule),
                };
            }
        }

        CheckResult { allowed: true, rule: None }
    }
}

/// Build privilege escalation rules (sudo, su, doas, pkexec)
fn build_privilege_escalation_rules() -> Vec<Rule> {
    let commands = [
        ("sudo", "execute commands with superuser privileges"),
        ("su", "switch user identity"),
        ("doas", "execute commands as another user"),
        ("pkexec", "execute commands as another user via PolicyKit"),
    ];

    commands
        .iter()
        .map(|(cmd, desc)| {
            // Match command at start of line, or after pipe/semicolon/&&/||
            let pattern = format!(r"(?:^|[|;&])\s*{}\s", regex::escape(cmd));
            Rule::new(&pattern, Category::PrivilegeEscalation, cmd, desc)
        })
        .collect()
}

/// Build destructive file operation rules (rm, rmdir, shred, etc.)
fn build_destructive_file_rules() -> Vec<Rule> {
    let commands = [
        ("rm", "remove files or directories"),
        ("rmdir", "remove empty directories"),
        ("shred", "securely delete files"),
        ("wipe", "securely erase files"),
        ("srm", "secure remove"),
        ("unlink", "remove files"),
        ("dd", "copy and convert files (can overwrite disks)"),
    ];

    let mut rules: Vec<Rule> = commands
        .iter()
        .map(|(cmd, desc)| {
            let pattern = format!(r"(?:^|[|;&])\s*{}\s", regex::escape(cmd));
            Rule::new(&pattern, Category::DestructiveFile, cmd, desc)
        })
        .collect();

    // Special case: truncate with size 0 (destructive)
    rules.push(Rule::new(
        r"(?:^|[|;&])\s*truncate\s+.*-s\s*0",
        Category::DestructiveFile,
        "truncate",
        "truncate files to zero size",
    ));

    // Special case: > file (redirecting nothing to file, truncates it)
    rules.push(Rule::new(
        r"(?:^|[|;&])\s*>\s*\S",
        Category::DestructiveFile,
        "> redirect",
        "truncate file via redirect",
    ));

    rules
}

/// Build system modification rules (chmod, mount, shutdown, etc.)
fn build_system_modification_rules() -> Vec<Rule> {
    let commands = [
        // Permission/ownership changes
        ("chmod", "change file permissions"),
        ("chown", "change file ownership"),
        ("chgrp", "change file group ownership"),
        ("chattr", "change file attributes"),
        // Disk/filesystem operations
        ("fdisk", "partition table manipulator"),
        ("parted", "partition editor"),
        ("mount", "mount filesystems"),
        ("umount", "unmount filesystems"),
        ("fsck", "filesystem check and repair"),
        // System control
        ("shutdown", "shutdown the system"),
        ("reboot", "reboot the system"),
        ("poweroff", "power off the system"),
        ("halt", "halt the system"),
        ("init", "change runlevel"),
        // User/group management
        ("useradd", "create user accounts"),
        ("userdel", "delete user accounts"),
        ("usermod", "modify user accounts"),
        ("groupadd", "create groups"),
        ("groupdel", "delete groups"),
        ("groupmod", "modify groups"),
        ("passwd", "change user password"),
        // Service management
        ("systemctl", "control systemd services"),
        ("service", "control system services"),
        // Kernel/module operations
        ("insmod", "insert kernel module"),
        ("rmmod", "remove kernel module"),
        ("modprobe", "add/remove kernel modules"),
        // SELinux/AppArmor
        ("setenforce", "modify SELinux mode"),
        ("aa-enforce", "set AppArmor profile to enforce"),
        ("aa-complain", "set AppArmor profile to complain"),
    ];

    let mut rules: Vec<Rule> = commands
        .iter()
        .map(|(cmd, desc)| {
            let pattern = format!(r"(?:^|[|;&])\s*{}\s", regex::escape(cmd));
            Rule::new(&pattern, Category::SystemModification, cmd, desc)
        })
        .collect();

    // Special case: mkfs and variants (mkfs.ext4, mkfs.xfs, etc.)
    rules.push(Rule::new(
        r"(?:^|[|;&])\s*mkfs(?:\.\w+)?\s",
        Category::SystemModification,
        "mkfs",
        "create filesystem (formats disk)",
    ));

    rules
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_disabled_by_default() {
        let checker = Checker::new();
        assert!(!checker.is_enabled());
        
        let result = checker.check("rm -rf /");
        assert!(result.allowed);
        assert!(result.rule.is_none());
    }

    #[test]
    fn test_enable_disable() {
        let checker = Checker::new();
        
        checker.set_enabled(true);
        assert!(checker.is_enabled());
        
        checker.set_enabled(false);
        assert!(!checker.is_enabled());
    }

    #[test]
    fn test_privilege_escalation() {
        let checker = Checker::new();
        checker.set_enabled(true);

        // Should be blocked
        assert!(!checker.check("sudo ls").allowed);
        assert!(!checker.check("sudo apt-get update").allowed);
        assert!(!checker.check("echo foo | sudo tee /etc/file").allowed);
        assert!(!checker.check("ls; sudo rm file").allowed);
        assert!(!checker.check("cd /tmp && sudo chmod 777 file").allowed);
        assert!(!checker.check("su -").allowed);
        assert!(!checker.check("su - root").allowed);
        assert!(!checker.check("doas ls").allowed);
        assert!(!checker.check("pkexec apt update").allowed);

        // Should be allowed
        assert!(checker.check("cat /etc/sudoers").allowed);
        assert!(checker.check("echo 'use sudo to...'").allowed);
        assert!(checker.check("result=success").allowed);
        assert!(checker.check("resume").allowed);
    }

    #[test]
    fn test_destructive_file_ops() {
        let checker = Checker::new();
        checker.set_enabled(true);

        // Should be blocked
        assert!(!checker.check("rm file.txt").allowed);
        assert!(!checker.check("rm -rf /tmp/dir").allowed);
        assert!(!checker.check("rm -f important.txt").allowed);
        assert!(!checker.check("rmdir empty_dir").allowed);
        assert!(!checker.check("shred secret.txt").allowed);
        assert!(!checker.check("unlink symlink").allowed);
        assert!(!checker.check("dd if=/dev/zero of=/dev/sda").allowed);
        assert!(!checker.check("wipe -f disk").allowed);
        assert!(!checker.check("ls && rm file").allowed);
        assert!(!checker.check("truncate -s 0 important.log").allowed);

        // Should be allowed
        assert!(checker.check("mkdir new_dir").allowed);
        assert!(checker.check("touch new_file").allowed);
        assert!(checker.check("mv old.txt new.txt").allowed);
        assert!(checker.check("cp source.txt dest.txt").allowed);
        assert!(checker.check("ls -la").allowed);
        assert!(checker.check("cat file.txt").allowed);
        assert!(checker.check("grep 'rm' script.sh").allowed);
        assert!(checker.check("echo 'do not rm this'").allowed);
    }

    #[test]
    fn test_system_modifications() {
        let checker = Checker::new();
        checker.set_enabled(true);

        // Should be blocked
        assert!(!checker.check("chmod 755 script.sh").allowed);
        assert!(!checker.check("chmod 777 /var/www").allowed);
        assert!(!checker.check("chown root:root file").allowed);
        assert!(!checker.check("chgrp admin file").allowed);
        assert!(!checker.check("mkfs /dev/sdb1").allowed);
        assert!(!checker.check("mkfs.ext4 /dev/sdb1").allowed);
        assert!(!checker.check("mkfs.xfs /dev/sdc1").allowed);
        assert!(!checker.check("fdisk /dev/sda").allowed);
        assert!(!checker.check("mount /dev/sdb1 /mnt").allowed);
        assert!(!checker.check("umount /mnt").allowed);
        assert!(!checker.check("shutdown -h now").allowed);
        assert!(!checker.check("reboot now").allowed);
        assert!(!checker.check("poweroff now").allowed);
        assert!(!checker.check("useradd newuser").allowed);
        assert!(!checker.check("userdel olduser").allowed);
        assert!(!checker.check("usermod -aG docker user").allowed);
        assert!(!checker.check("passwd user").allowed);
        assert!(!checker.check("systemctl stop nginx").allowed);
        assert!(!checker.check("systemctl start docker").allowed);
        assert!(!checker.check("service apache2 restart").allowed);
        assert!(!checker.check("insmod module.ko").allowed);
        assert!(!checker.check("rmmod module").allowed);
        assert!(!checker.check("modprobe driver").allowed);

        // Should be allowed
        assert!(checker.check("ls -la").allowed);
        assert!(checker.check("stat file.txt").allowed);
        assert!(checker.check("id").allowed);
        assert!(checker.check("whoami").allowed);
    }

    #[test]
    fn test_empty_and_whitespace() {
        let checker = Checker::new();
        checker.set_enabled(true);

        assert!(checker.check("").allowed);
        assert!(checker.check("   ").allowed);
        assert!(checker.check("\t\t").allowed);
        assert!(checker.check("\n\n").allowed);
    }

    #[test]
    fn test_complex_commands() {
        let checker = Checker::new();
        checker.set_enabled(true);

        // Complex blocked commands
        assert!(!checker.check("cd /tmp && rm -rf *").allowed);
        assert!(!checker.check("rm -rf dir &").allowed);
        assert!(!checker.check("rm file 2>/dev/null").allowed);

        // Complex allowed commands
        assert!(checker.check("cat file | grep pattern | wc -l").allowed);
        assert!(checker.check("echo $(date)").allowed);
        assert!(checker.check("pwd && ls && echo done").allowed);
        assert!(checker.check("sleep 10 &").allowed);
    }

    #[test]
    fn test_category_description() {
        assert_eq!(Category::PrivilegeEscalation.description(), "Privilege escalation");
        assert_eq!(Category::DestructiveFile.description(), "Destructive file operation");
        assert_eq!(Category::SystemModification.description(), "System modification");
    }

    #[test]
    fn test_check_result_accessors() {
        let checker = Checker::new();
        checker.set_enabled(true);

        let result = checker.check("sudo ls");
        assert!(!result.allowed);
        assert_eq!(result.command(), Some("sudo"));
        assert_eq!(result.category(), Some(Category::PrivilegeEscalation));

        let result = checker.check("rm file");
        assert!(!result.allowed);
        assert_eq!(result.command(), Some("rm"));
        assert_eq!(result.category(), Some(Category::DestructiveFile));

        let result = checker.check("ls -la");
        assert!(result.allowed);
        assert!(result.command().is_none());
        assert!(result.category().is_none());
    }
}
