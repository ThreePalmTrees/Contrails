import { useState, useEffect, useCallback } from "react";
import { Project, ProcessingProgress, AppError } from "../types";
import {
  GetProjects,
  AddProject,
  UpdateProject,
  RemoveProject,
  ProcessChatSessions,
  ProcessClaudeCodeSessions,
  ProcessFileIfNeeded,
  HandleDeletedFile,
} from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime/runtime";

const SELECTED_PROJECT_KEY = "contrails:selectedProjectId";
const BADGE_COUNTS_KEY = "contrails:badgeCounts";
const ONBOARDING_KEY = "contrails:onboardingComplete";

function loadBadgeFiles(): Record<string, string[]> {
  try {
    const raw = localStorage.getItem(BADGE_COUNTS_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    // Migrate from old format (number counts) to new format (file arrays)
    const result: Record<string, string[]> = {};
    for (const [key, value] of Object.entries(parsed)) {
      if (Array.isArray(value)) {
        result[key] = value;
      }
      // Discard old numeric counts — can't recover file names
    }
    return result;
  } catch {
    return {};
  }
}

function saveBadgeFiles(files: Record<string, string[]>) {
  localStorage.setItem(BADGE_COUNTS_KEY, JSON.stringify(files));
}

export function useProjects() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(
    () => localStorage.getItem(SELECTED_PROJECT_KEY)
  );
  const [loading, setLoading] = useState(true);
  const [processing, setProcessing] = useState<string | null>(null);
  const [processingProgress, setProcessingProgress] = useState<ProcessingProgress | null>(null);
  const [badgeFiles, setBadgeFiles] = useState<Record<string, string[]>>(loadBadgeFiles);
  const [appError, setAppError] = useState<AppError | null>(null);
  const [onboardingComplete, setOnboardingComplete] = useState(
    () => localStorage.getItem(ONBOARDING_KEY) === "true"
  );

  const loadProjects = useCallback(async () => {
    try {
      const result = await GetProjects();
      setProjects(result || []);
    } catch (err) {
      console.error("Failed to load projects:", err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadProjects();
  }, [loadProjects]);

  // Persist selected project
  useEffect(() => {
    if (selectedId) {
      localStorage.setItem(SELECTED_PROJECT_KEY, selectedId);
    } else {
      localStorage.removeItem(SELECTED_PROJECT_KEY);
    }
  }, [selectedId]);

  // Clear badge when a project is selected
  useEffect(() => {
    if (selectedId && badgeFiles[selectedId]?.length) {
      setBadgeFiles((prev) => {
        const next = { ...prev };
        delete next[selectedId];
        saveBadgeFiles(next);
        return next;
      });
    }
  }, [selectedId]); // eslint-disable-line react-hooks/exhaustive-deps

  // Listen for watcher events and process incrementally
  useEffect(() => {
    const cleanup = EventsOn("watcher:event", (event: { projectId: string; fileName: string; eventType: string }) => {
      const project = projects.find((p) => p.id === event.projectId);
      if (!project) return;

      // Only process VS Code watcher events via ProcessFileIfNeeded.
      // Claude Code events are handled internally by the signal watcher.
      const hasVSCodeSource = project.sources?.some((s) => s.type === "vscode");
      if (!hasVSCodeSource) return;

      if (event.eventType === "created" || event.eventType === "modified") {
        const watchDir = project.sources?.find((s) => s.type === "vscode")?.watchDir || project.watchDir;
        const filePath = `${watchDir}/${event.fileName}`;
        ProcessFileIfNeeded(project.id, filePath, project.outputDir).catch(
          (err) => {
            console.error("Processing failed:", err);
            setAppError({
              projectId: project.id,
              projectName: project.name,
              message: `Failed to process ${event.fileName}: ${err}`,
            });
          }
        );
      } else if (event.eventType === "removed") {
        HandleDeletedFile(event.fileName, project.outputDir).catch(
          console.error
        );
      }
    });
    return cleanup;
  }, [projects]);

  // Listen for file-processed events (for badge tracking)
  useEffect(() => {
    const cleanup = EventsOn("file:processed", (event: { projectId: string; fileName: string }) => {
      // Only track badge if project is not currently selected
      if (event.projectId !== selectedId) {
        setBadgeFiles((prev) => {
          const existing = prev[event.projectId] || [];
          // Only add if this file isn't already tracked
          if (existing.includes(event.fileName)) {
            return prev;
          }
          const next = { ...prev, [event.projectId]: [...existing, event.fileName] };
          saveBadgeFiles(next);
          return next;
        });
      }
    });
    return cleanup;
  }, [selectedId]);

  // Listen for processing progress events
  useEffect(() => {
    const cleanup = EventsOn("processing:progress", (progress: ProcessingProgress) => {
      setProcessingProgress(progress);
    });
    return cleanup;
  }, []);

  // Listen for app errors
  useEffect(() => {
    const cleanup = EventsOn("app:error", (error: AppError) => {
      setAppError(error);
    });
    return cleanup;
  }, []);

  const addProject = useCallback(async (project: Project) => {
    await AddProject(main.Project.createFrom(project));
    await loadProjects();
    setSelectedId(project.id);
  }, [loadProjects]);

  const updateProject = useCallback(async (project: Project) => {
    await UpdateProject(main.Project.createFrom(project));
    await loadProjects();
  }, [loadProjects]);

  const removeProject = useCallback(async (id: string) => {
    await RemoveProject(id);
    if (selectedId === id) {
      setSelectedId(null);
    }
    // Clear badge for removed project
    setBadgeFiles((prev) => {
      const next = { ...prev };
      delete next[id];
      saveBadgeFiles(next);
      return next;
    });
    await loadProjects();
  }, [selectedId, loadProjects]);

  const processNow = useCallback(async (project: Project) => {
    setProcessing(project.id);
    setProcessingProgress(null);
    try {
      let totalCount = 0;

      // Process VS Code sessions if applicable
      const vscodeSource = project.sources?.find((s) => s.type === "vscode");
      if (vscodeSource?.watchDir || project.watchDir) {
        const watchDir = vscodeSource?.watchDir || project.watchDir;
        const count = await ProcessChatSessions(project.id, watchDir, project.outputDir);
        totalCount += count;
      }

      // Process Claude Code sessions if applicable
      const claudeSource = project.sources?.find((s) => s.type === "claudecode");
      if (claudeSource?.watchDir) {
        const count = await ProcessClaudeCodeSessions(project.id, claudeSource.watchDir, project.outputDir);
        totalCount += count;
      }

      return totalCount;
    } catch (err) {
      setAppError({
        projectId: project.id,
        projectName: project.name,
        message: `Processing failed: ${err}`,
      });
      throw err;
    } finally {
      setProcessing(null);
      setProcessingProgress(null);
    }
  }, []);

  const dismissError = useCallback(() => {
    setAppError(null);
  }, []);

  const completeOnboarding = useCallback(() => {
    localStorage.setItem(ONBOARDING_KEY, "true");
    setOnboardingComplete(true);
  }, []);

  const selectedProject = projects.find((p) => p.id === selectedId) || null;

  // Derive badge counts (number of unique files) for the UI
  const badgeCounts: Record<string, number> = {};
  for (const [key, files] of Object.entries(badgeFiles)) {
    if (files.length > 0) {
      badgeCounts[key] = files.length;
    }
  }

  return {
    projects,
    selectedProject,
    selectedId,
    setSelectedId,
    addProject,
    updateProject,
    removeProject,
    processNow,
    loading,
    processing,
    processingProgress,
    badgeCounts,
    appError,
    dismissError,
    onboardingComplete,
    completeOnboarding,
    loadProjects,
  };
}
