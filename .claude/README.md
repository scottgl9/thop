# Claude Code Plugin Directory

This directory contains the Claude Code plugin for thop - a terminal session manager for AI agents.

## Files

### `thop-plugin.md`

The main plugin file containing comprehensive instructions for using thop with Claude Code. This is a **completely self-contained** plugin - all documentation is embedded directly in the file.

## How Claude Code Uses This

While Claude Code primarily uses `.clinerules` for project-level instructions, this plugin file provides:

1. **Extended documentation** for complex thop workflows
2. **Reference material** that can be included in `.clinerules` via reference
3. **Standalone guide** that can be opened directly by Claude or users
4. **Portable documentation** that can be copied to any project

## Usage

### Option 1: Direct Reference (Recommended)

The `.clinerules` file in the project root includes all essential thop instructions inline. This ensures Claude has immediate access to the information.

### Option 2: Supplementary Documentation

This plugin file serves as extended documentation that provides:
- More detailed examples
- Comprehensive error handling scenarios
- Advanced workflow patterns
- Complete command reference

### Option 3: Copy to Other Projects

This file is completely self-contained and can be copied to any project:

```bash
# Copy to another project
cp .claude/thop-plugin.md ~/your-project/.claude/

# Or reference it from .clinerules
# "See .claude/thop-plugin.md for complete thop documentation"
```

## Design Philosophy

**Self-Contained**: The plugin has no external dependencies. All documentation, examples, and instructions are embedded directly in the file.

**Portable**: Can be copied to any project and work immediately without modification.

**Comprehensive**: Includes everything needed to use thop effectively:
- Detection and setup
- All commands with examples
- Workflow patterns
- Error handling
- Best practices
- Quick reference

## For Other Projects

To use this plugin in other projects that use thop:

```bash
# Create .claude directory
mkdir -p ~/your-project/.claude

# Copy the plugin
cp .claude/thop-plugin.md ~/your-project/.claude/

# Optionally update .clinerules to reference it
echo "See .claude/thop-plugin.md for complete thop documentation" >> ~/your-project/.clinerules
```

The plugin will work immediately with no additional setup required.
