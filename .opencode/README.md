# thop Plugins

This directory contains AI agent plugins that help OpenCode and Claude Code understand how to use thop. **All plugins are completely self-contained** - they don't reference external files and can be copied to any project.

## OpenCode Plugin

**File**: `plugins/thop.ts`

A completely self-contained TypeScript plugin that automatically:
- Detects when OpenCode is running inside thop (checks `$THOP_SESSION`)
- Injects comprehensive thop usage instructions into the session context
- Logs the current thop session on startup
- Warns before executing destructive commands on remote sessions
- Tracks session switches and logs them

### Features

1. **Auto-detection**: Checks `$THOP_SESSION` environment variable
2. **Embedded documentation**: All thop instructions are built into the plugin (no external file dependencies)
3. **Safety warnings**: Alerts on destructive commands in remote sessions (rm -rf, dd, mkfs, etc.)
4. **Session tracking**: Logs when you switch between sessions

### Usage

The plugin is automatically loaded when OpenCode starts in this directory. No configuration needed.

**To use in other projects:**
```bash
# Copy the plugin to any project
mkdir -p ~/your-project/.opencode/plugins
cp .opencode/plugins/thop.ts ~/your-project/.opencode/plugins/

# That's it! The plugin is completely self-contained.
```

## Claude Code Plugin

**Files**: 
- `../.clinerules` - Essential runtime instructions (automatically loaded)
- `../.claude/thop-plugin.md` - Extended documentation and reference guide

### .clinerules

A completely self-contained project-level configuration file that teaches Claude Code how to:
- Detect when it's running inside thop
- Use all thop slash commands
- Follow best practices for multi-server workflows
- Handle errors gracefully
- Avoid common pitfalls

### .claude/thop-plugin.md

Extended plugin documentation providing:
- Comprehensive command reference
- Detailed workflow examples
- Advanced error handling scenarios
- Performance and security notes
- Quick reference table

### Usage

When you run Claude Code in a directory with `.clinerules`, it automatically reads the file and understands how to use thop. The `.claude/thop-plugin.md` file provides supplementary documentation.

**To use in other projects:**
```bash
# Copy both files to any project
cp .clinerules ~/your-project/
cp -r .claude ~/your-project/

# Or just .clinerules for basic usage
cp .clinerules ~/your-project/

# Both are completely self-contained with no external dependencies
```

## Key Design Principle

**Both plugins are completely self-contained:**
- ✅ All documentation is embedded directly in the files
- ✅ No references to project-specific paths
- ✅ No external file dependencies
- ✅ Can be copied to any project and work immediately

This ensures the plugins are portable and work anywhere thop is being used.

## What's Included in the Plugins

All plugins contain complete thop documentation including:

### OpenCode Plugin (`plugins/thop.ts`)
- Session management commands with TypeScript integration
- Auto-detection and context injection
- Safety warnings for destructive operations
- Session switch tracking and logging

### Claude Code Plugin (`.clinerules` + `.claude/thop-plugin.md`)
- **Essential instructions** in `.clinerules`:
  - Session management commands
  - File operations
  - Environment & jobs
  - Best practices and workflow guidelines
  
- **Extended documentation** in `.claude/thop-plugin.md`:
  - Comprehensive command reference
  - Detailed workflow examples
  - Advanced error handling
  - Configuration examples
  - Quick reference table
  - Performance & security notes

Both plugins include:
- Complete command documentation
- Workflow patterns and examples
- Error handling guidance
- Best practices and common pitfalls
- Configuration information
- Performance and security notes

## Testing the Plugins

### Test OpenCode Plugin

```bash
# Run OpenCode in a directory with the plugin
cd ~/your-project
opencode

# If running inside thop, the plugin will:
# - Log "thop detected - active session: <name>"
# - Inject thop instructions into the session
```

### Test Claude Code Plugin

```bash
# Run Claude Code with thop as shell
cd ~/your-project
SHELL="thop --proxy" claude

# Claude will automatically:
# - Read .clinerules for essential instructions
# - Access .claude/thop-plugin.md for extended documentation
```

## For thop Project Contributors

These plugins are designed to work in ANY project that uses thop, not just this project. When testing or improving them:

1. ✅ **DO**: Keep all instructions embedded in the plugin files
2. ✅ **DO**: Test by copying to another project directory
3. ❌ **DON'T**: Reference project-specific files or paths
4. ❌ **DON'T**: Add dependencies on external documentation

## Contributing

If you improve these plugins:
1. Ensure they remain self-contained (no external file references)
2. Test by copying to a different project directory
3. Verify both OpenCode and Claude Code work correctly
4. Update this README with any new features

