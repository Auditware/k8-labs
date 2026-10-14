import { createContext, useContext } from "react";

// Bridges the course's runnable-command chips (CoursePlayer) to the live lab
// terminal (NestedTerminal), which are in sibling panes of LabPage and share no
// parent but it. The terminal registers a raw "send to the shell" function when
// its socket opens; a Run button calls runCommand, which makes sure a session
// and the terminal tab exist and then pastes the command + Enter - queueing it
// until the shell connects if the lab is still provisioning.
export type TerminalRun = {
  // Paste a command into the lab terminal and run it. Auto-starts the session
  // and opens the terminal tab; queues until the shell is connected. The
  // promise resolves once the command has actually been written to the shell -
  // which may be seconds later while a nested lab provisions - so a Run button
  // can show a pending/loading state for exactly that wait instead of looking
  // inert until the command silently appears.
  runCommand: (cmd: string) => Promise<void>;
  // Called by the terminal when its socket opens. `send` writes raw bytes to
  // the shell (same path as a keystroke). Returns an unregister function.
  registerTerminal: (send: (data: string) => void) => () => void;
};

export const TerminalRunContext = createContext<TerminalRun | null>(null);

export function useTerminalRun(): TerminalRun | null {
  return useContext(TerminalRunContext);
}
