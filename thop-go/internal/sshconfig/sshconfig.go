package sshconfig

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// HostConfig represents SSH configuration for a host
type HostConfig struct {
	Host         string
	HostName     string
	User         string
	Port         string
	IdentityFile string
	ProxyJump    string
}

// Config holds parsed SSH configuration
type Config struct {
	Hosts map[string]*HostConfig
}

// Load loads SSH configuration from the default location
func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return &Config{Hosts: make(map[string]*HostConfig)}, nil
	}

	configPath := filepath.Join(home, ".ssh", "config")
	return LoadFromFile(configPath)
}

// LoadFromFile loads SSH configuration from a specific file
func LoadFromFile(path string) (*Config, error) {
	config := &Config{
		Hosts: make(map[string]*HostConfig),
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentHost *HostConfig
	var currentPatterns []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key-value pair
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			// Try with = separator
			parts = strings.SplitN(line, "=", 2)
			if len(parts) < 2 {
				continue
			}
		}

		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		value = strings.Trim(value, "\"'")

		if key == "host" {
			// Save previous host config
			if currentHost != nil {
				for _, pattern := range currentPatterns {
					// Skip wildcard patterns
					if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "?") {
						config.Hosts[pattern] = currentHost
					}
				}
			}

			// Start new host block
			currentPatterns = strings.Fields(value)
			currentHost = &HostConfig{
				Host: currentPatterns[0],
			}
		} else if currentHost != nil {
			switch key {
			case "hostname":
				currentHost.HostName = value
			case "user":
				currentHost.User = value
			case "port":
				currentHost.Port = value
			case "identityfile":
				// Expand ~ in path
				if strings.HasPrefix(value, "~") {
					home, _ := os.UserHomeDir()
					value = filepath.Join(home, value[1:])
				}
				currentHost.IdentityFile = value
			case "proxyjump":
				currentHost.ProxyJump = value
			}
		}
	}

	// Save last host config
	if currentHost != nil {
		for _, pattern := range currentPatterns {
			if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "?") {
				config.Hosts[pattern] = currentHost
			}
		}
	}

	return config, scanner.Err()
}

// GetHost returns the configuration for a host alias
func (c *Config) GetHost(alias string) *HostConfig {
	return c.Hosts[alias]
}

// ResolveHost returns the actual hostname for an alias
func (c *Config) ResolveHost(alias string) string {
	if host := c.Hosts[alias]; host != nil && host.HostName != "" {
		return host.HostName
	}
	return alias
}

// ResolveUser returns the user for an alias
func (c *Config) ResolveUser(alias string) string {
	if host := c.Hosts[alias]; host != nil && host.User != "" {
		return host.User
	}
	return ""
}

// ResolvePort returns the port for an alias
func (c *Config) ResolvePort(alias string) string {
	if host := c.Hosts[alias]; host != nil && host.Port != "" {
		return host.Port
	}
	return "22"
}

// ResolveIdentityFile returns the identity file for an alias
func (c *Config) ResolveIdentityFile(alias string) string {
	if host := c.Hosts[alias]; host != nil && host.IdentityFile != "" {
		return host.IdentityFile
	}
	return ""
}

// ListHosts returns all configured host aliases
func (c *Config) ListHosts() []string {
	hosts := make([]string, 0, len(c.Hosts))
	for name := range c.Hosts {
		hosts = append(hosts, name)
	}
	return hosts
}
