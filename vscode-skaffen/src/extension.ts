import * as vscode from "vscode";

let skaffenTerminal: vscode.Terminal | undefined;
let statusBarItem: vscode.StatusBarItem;

export function activate(context: vscode.ExtensionContext) {
  // Status bar item
  statusBarItem = vscode.window.createStatusBarItem(
    vscode.StatusBarAlignment.Left,
    100
  );
  statusBarItem.command = "skaffen.open";
  statusBarItem.text = "$(terminal) Skaffen";
  statusBarItem.tooltip = "Open Skaffen AI Agent";
  statusBarItem.show();
  context.subscriptions.push(statusBarItem);

  // Track terminal lifecycle
  vscode.window.onDidCloseTerminal((terminal) => {
    if (terminal === skaffenTerminal) {
      skaffenTerminal = undefined;
      statusBarItem.text = "$(terminal) Skaffen";
    }
  });

  // Open command
  const openCmd = vscode.commands.registerCommand("skaffen.open", () => {
    if (skaffenTerminal) {
      skaffenTerminal.show();
      return;
    }

    const workspaceRoot =
      vscode.workspace.workspaceFolders?.[0]?.uri.fsPath ?? "";
    const activeFile =
      vscode.window.activeTextEditor?.document.uri.fsPath ?? "";

    const env: Record<string, string> = {};
    if (workspaceRoot) {
      env["SKAFFEN_VSCODE_ROOT"] = workspaceRoot;
    }
    if (activeFile) {
      env["SKAFFEN_VSCODE_FILE"] = activeFile;
    }

    skaffenTerminal = vscode.window.createTerminal({
      name: "Skaffen",
      cwd: workspaceRoot || undefined,
      env,
    });
    skaffenTerminal.sendText("skaffen", true);
    skaffenTerminal.show();
    statusBarItem.text = "$(terminal-active) Skaffen";
  });

  // Send file command
  const sendFileCmd = vscode.commands.registerCommand(
    "skaffen.sendFile",
    () => {
      const activeFile =
        vscode.window.activeTextEditor?.document.uri.fsPath;
      if (!activeFile) {
        vscode.window.showWarningMessage("No active file to send");
        return;
      }
      if (!skaffenTerminal) {
        vscode.window.showWarningMessage("Skaffen is not running");
        return;
      }
      // Send file path as @mention to Skaffen's stdin
      skaffenTerminal.sendText(`@${activeFile}`, false);
      skaffenTerminal.show();
    }
  );

  context.subscriptions.push(openCmd, sendFileCmd);
}

export function deactivate() {
  skaffenTerminal?.dispose();
}
