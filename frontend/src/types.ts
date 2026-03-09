export interface AgentSource {
  type: string;
  watchDir?: string;
}

export interface Project {
  id: string;
  name: string;
  watchDir: string;
  outputDir: string;
  active: boolean;
  workspacePath?: string;
  sources?: AgentSource[];
  lastProcessed?: number;
  pausedAt?: number;
}

export interface WorkspaceInfo {
  id: string;
  chatSessionsDir: string;
  name: string;
  workspacePath?: string;
  sessionCount?: number;
}

export interface WatcherEvent {
  projectId: string;
  fileName: string;
  eventType: "created" | "modified" | "removed";
}

export interface ProcessingProgress {
  projectId: string;
  current: number;
  total: number;
}

export interface FileProcessedEvent {
  projectId: string;
  fileName: string;
}

export interface ChatFileInfo {
  fileName: string;
  filePath: string;
  sourceType: string;
  parsed: boolean;
  partiallyParsed: boolean;
  title: string;
  lastMessageAt: string;
  processedAt: number;
  createdAt: number;
}

export interface AppError {
  projectId: string;
  projectName: string;
  message: string;
}
