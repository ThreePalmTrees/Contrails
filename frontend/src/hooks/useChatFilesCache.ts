import { useState, useEffect, useCallback, useRef } from "react";
import { ChatFileInfo } from "../types";
import { ListChatFiles } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

const DEBOUNCE_MS = 2000;

interface CacheEntry {
  data: ChatFileInfo[];
  timestamp: number;
}

/**
 * Centralized cache for chat file lists, keyed by project ID.
 * Lives above ProjectDetail so data survives project switching.
 *
 * - Returns cached data instantly on project switch (no IPC roundtrip).
 * - Debounces watcher/processed events so rapid agent actions don't
 *   each trigger a full ListChatFiles scan.
 * - Exposes update/invalidate methods for optimistic UI writes.
 */
export function useChatFilesCache() {
  const cacheRef = useRef<Map<string, CacheEntry>>(new Map());
  // Incrementing version per project triggers consumers to re-read from cache
  const [versions, setVersions] = useState<Record<string, number>>({});
  const debounceTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());

  // Bump the version for a project so consumers re-render
  const bumpVersion = useCallback((projectId: string) => {
    setVersions((prev) => ({ ...prev, [projectId]: (prev[projectId] || 0) + 1 }));
  }, []);

  // Schedule a debounced refetch for a project
  const scheduleRefetch = useCallback((projectId: string) => {
    const existing = debounceTimers.current.get(projectId);
    if (existing) clearTimeout(existing);

    const timer = setTimeout(async () => {
      debounceTimers.current.delete(projectId);
      try {
        const files = await ListChatFiles(projectId);
        cacheRef.current.set(projectId, { data: files ?? [], timestamp: Date.now() });
        bumpVersion(projectId);
      } catch {
        // If fetch fails, don't evict existing cache — stale data is better than none
      }
    }, DEBOUNCE_MS);

    debounceTimers.current.set(projectId, timer);
  }, [bumpVersion]);

  // Listen for events that should trigger cache invalidation
  useEffect(() => {
    const unbindProcessed = EventsOn("file:processed", (event: { projectId: string }) => {
      scheduleRefetch(event.projectId);
    });

    const unbindWatcher = EventsOn("watcher:event", (event: { projectId: string; eventType: string }) => {
      // For "created" or "removed" events, always schedule refetch (list changed)
      // For "modified" events, also schedule but debounce will batch them
      scheduleRefetch(event.projectId);
    });

    const unbindCursor = EventsOn("cursor:changed", (event: { projectId: string }) => {
      scheduleRefetch(event.projectId);
    });

    return () => {
      unbindProcessed();
      unbindWatcher();
      unbindCursor();
      // Clean up any pending timers
      for (const timer of debounceTimers.current.values()) {
        clearTimeout(timer);
      }
    };
  }, [scheduleRefetch]);

  /**
   * Get chat files for a project. Returns cached data if available,
   * otherwise fetches from backend. Returns { data, loading }.
   */
  const getChatFiles = useCallback(
    async (projectId: string): Promise<ChatFileInfo[]> => {
      const cached = cacheRef.current.get(projectId);
      if (cached) return cached.data;

      // No cache — fetch synchronously and populate
      const files = await ListChatFiles(projectId);
      const data = files ?? [];
      cacheRef.current.set(projectId, { data, timestamp: Date.now() });
      bumpVersion(projectId);
      return data;
    },
    [bumpVersion]
  );

  /**
   * Force a full refetch for a project (e.g., after batch processing).
   * Bypasses debounce.
   */
  const invalidate = useCallback(
    async (projectId: string) => {
      // Cancel any pending debounced refetch
      const existing = debounceTimers.current.get(projectId);
      if (existing) {
        clearTimeout(existing);
        debounceTimers.current.delete(projectId);
      }
      try {
        const files = await ListChatFiles(projectId);
        cacheRef.current.set(projectId, { data: files ?? [], timestamp: Date.now() });
        bumpVersion(projectId);
      } catch {
        // Keep stale cache on error
      }
    },
    [bumpVersion]
  );

  /**
   * Update cached entries in place (for optimistic UI updates).
   * The updater receives the current array and should return the new one.
   */
  const updateCache = useCallback(
    (projectId: string, updater: (prev: ChatFileInfo[]) => ChatFileInfo[]) => {
      const cached = cacheRef.current.get(projectId);
      if (!cached) return;
      cacheRef.current.set(projectId, {
        data: updater(cached.data),
        timestamp: cached.timestamp,
      });
      bumpVersion(projectId);
    },
    [bumpVersion]
  );

  /**
   * Read current cached data synchronously (for initial render).
   * Returns undefined if not cached yet.
   */
  const peekCache = useCallback((projectId: string): ChatFileInfo[] | undefined => {
    return cacheRef.current.get(projectId)?.data;
  }, []);

  return {
    getChatFiles,
    invalidate,
    updateCache,
    peekCache,
    versions,
  };
}
