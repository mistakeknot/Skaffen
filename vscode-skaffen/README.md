# Skaffen for VS Code

Integrates the Skaffen AI agent into VS Code's integrated terminal.

## Features

- **Open Skaffen** (`Ctrl+Shift+S` / `Cmd+Shift+S`): Launch Skaffen in a VS Code terminal
- **Send File**: Send the active editor file to Skaffen as an @mention
- **Status Bar**: Shows Skaffen status (click to focus)
- **Auto-activate**: Activates when `.skaffen/` directory is detected in workspace

## Environment Variables

The extension sets these environment variables for Skaffen:

| Variable | Description |
|----------|-------------|
| `SKAFFEN_VSCODE_ROOT` | Workspace root directory |
| `SKAFFEN_VSCODE_FILE` | Active editor file path |

## Installation

### From VSIX (local)

1. Build: `npm install && npm run package`
2. Install: `code --install-extension vscode-skaffen-0.1.0.vsix`

### Prerequisites

- `skaffen` binary on PATH
- VS Code >= 1.85
