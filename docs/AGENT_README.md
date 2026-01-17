# Agent Documentation for thop

This directory contains documentation specifically designed for AI coding agents (like Claude Code) working within thop sessions.

## Files

### THOP_FOR_AGENTS.md (236 lines)
**General agent guide** - Platform-agnostic, essential instructions

- Detection and core concepts
- Essential commands (session, file, env, jobs)
- 3 workflow examples
- Best practices and error handling
- Quick reference table
- Common pitfalls to avoid

**Use this for**: Any AI agent integration

### THOP_FOR_CLAUDE.md (141 lines)
**Claude-specific guide** - Concise, focused instructions

- Quick detection method
- Core commands with examples
- 1 complete workflow example
- Common patterns (debug, compare, deploy)
- Best practices summary
- Quick reference table

**Use this for**: Claude Code, Claude Desktop, claude.vim

## How to Use These Docs

**Copy to your project directory** so AI agents have access when working on that project:

```bash
# For Claude Code
cp /path/to/thop/docs/THOP_FOR_CLAUDE.md ~/myproject/

# For other AI agents
cp /path/to/thop/docs/THOP_FOR_AGENTS.md ~/myproject/
```

Then reference in your project README:

```markdown
## AI Agent Setup

This project uses thop for remote server management.

See [THOP_FOR_CLAUDE.md](./THOP_FOR_CLAUDE.md) for usage instructions.
```

## Why These Docs Were Simplified

Original versions were 730+ lines with extensive examples. We've distilled them to essentials:

- **Focus on what agents need to know**, not comprehensive documentation
- **Core commands and common patterns** instead of exhaustive examples
- **Quick reference format** for fast lookups
- **Reduced token usage** when agents read these files

Full documentation is in the main README.md and other docs.

## What's Included

Both docs cover essentials:
- ✅ Session detection (`$THOP_SESSION`)
- ✅ Core commands (`/status`, `/connect`, `/switch`, `/copy`)
- ✅ Workflow examples (deploy, debug, compare)
- ✅ Error handling (AUTH_KEY_FAILED, etc.)
- ✅ Best practices (always check `/status`, return to `/local`)
- ✅ Quick reference tables

---

**Quick Links**:
- [THOP_FOR_AGENTS.md](./THOP_FOR_AGENTS.md) - General agent guide
- [THOP_FOR_CLAUDE.md](./THOP_FOR_CLAUDE.md) - Claude-specific guide
- [MCP.md](./MCP.md) - MCP server documentation
- [MCP_IMPROVEMENTS.md](./MCP_IMPROVEMENTS.md) - Planned MCP enhancements
