# AI Agents

This document describes the AI agents working on the WiFi Diagnostic & Troubleshooting App and their collaboration protocols.

## Primary Agent

**Sisyphus** - Orchestration Agent
- **Role**: Senior engineer coordinating development tasks
- **Capabilities**: Code generation, debugging, architecture decisions, multi-file refactoring
- **Operating Principles**: Parse implicit requirements, adapt to codebase maturity, delegate to specialists
- **Command Pattern**: Direct execution without status updates. Just work.

## Specialist Agents

### Oracle
- **Role**: Senior technical advisor with deep reasoning
- **Usage**: Complex architecture, multi-system tradeoffs, security/performance concerns, failed debugging attempts
- **Cost**: EXPENSIVE - use judiciously

### Explore
- **Role**: Codebase context researcher (contextual grep)
- **Usage**: Finding code patterns, module structures, cross-layer pattern discovery
- **Behavior**: Background task, always parallel, non-blocking

### Librarian
- **Role**: External reference researcher
- **Usage**: Official documentation, OSS implementation examples, library best practices
- **Behavior**: Background task, always parallel, non-blocking

### Frontend UI/UX Engineer
- **Role**: Designer-turned-developer for visual/styling work
- **Usage**: Visual changes only - colors, spacing, layout, typography, animations, responsive design
- **Excluded**: Pure logic changes in frontend files (handle directly)

### Document Writer
- **Role**: Technical documentation specialist
- **Usage**: README files, API docs, architecture guides, user guides

## Project Context

### Technology Stack
- **Backend**: Go with Wails v2
- **Frontend**: Svelte + Vite
- **WiFi Backend**: Multiple implementations (Linux: `iw`, macOS: CoreWLAN, Windows: Native WiFi)
- **Key Dependencies**: mdlayher/wifi (vendored)

### Architecture
```
wifi-app/
├── Go Backend (Wails service)
│   ├── WiFi Scanner (platform-specific)
│   ├── Data Polling & Aggregation
│   ├── Real-time Events
│   └── API Bindings
└── Svelte Frontend
    ├── Network List & Analysis
    ├── Signal Charts (Chart.js)
    ├── Channel Analyzer
    └── Client Stats Panel
```

### Current State
- ✅ Backend WiFi scanning implementations (Linux, macOS, Windows)
- ✅ Wails service bindings for real-time data
- ✅ Svelte components for network visualization
- ✅ Channel analysis and signal monitoring
- 🔄 Ongoing: Cross-platform testing and refinement

## Collaboration Protocols

### When to Fire Explore
- Multiple search angles needed for unfamiliar module structure
- Cross-layer pattern discovery
- Finding existing implementations across the codebase

### When to Fire Librarian
- Unfamiliar libraries or packages
- Best practices for Wails/Svelte integration
- WiFi scanning API documentation
- Official implementation examples

### When to Delegate to Frontend UI/UX
- **VISUAL** changes only: styling, layout, animation, responsive design
- **DO NOT** delegate: Pure logic, API calls, state management, business logic

### When to Consult Oracle
- Complex architecture decisions before implementation
- After completing significant work for self-review
- After 2+ failed fix attempts
- Unfamiliar code patterns
- Security or performance concerns

## Development Workflow

1. **Investigation**: Fire explore/librarian agents in parallel
2. **Planning**: Create detailed todo list for multi-step tasks
3. **Implementation**: Work through todos, updating status in real-time
4. **Verification**: Run lsp_diagnostics, build, tests
5. **Cleanup**: Cancel all background tasks before completion

## Codebase Discipline

**Current State**: Disciplined
- Consistent patterns established across Go backend and Svelte frontend
- Linting and formatting configured
- Follow existing conventions strictly

**Key Patterns**:
- Platform-specific WiFi scanner implementations via interface
- Wails runtime events for real-time updates
- Svelte stores for reactive state management
- Component-based frontend architecture

## Testing & Quality

- Run `lsp_diagnostics` on changed files before completion
- Verify build with `wails build`
- Test WiFi scanning functionality on supported platforms
- Check for regressions after changes

## Background Task Management

**Always** use `background_cancel(all=true)` before delivering final answer to conserve resources.

---

*Last Updated: 2026-01-18*
