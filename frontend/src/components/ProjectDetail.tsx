import React, { useState, useEffect, useLayoutEffect, useMemo, useRef, useCallback } from "react";
import { createPortal } from "react-dom";
import { FolderOpen, FolderUp, Eye, EyeOff, MapPin, Play, Loader2, Layers, CheckCircle2, ChevronLeft, Pencil, Trash2, ExternalLink, ChevronDown, ChevronRight, Search, MoreHorizontal, Tag, X, Plus, Check } from "lucide-react";
import { OpenDirectoryWith, GetDirectoryOpener } from "../../wailsjs/go/main/App";
import { DirectoryOpenerDialog } from "./DirectoryOpenerDialog";
import { Project, ProcessingProgress, ChatFileInfo, Category } from "../types";
import copilotLogo from "../assets/images/gh-copilot.png";
import claudeLogo from "../assets/images/claude.png";
import cursorLogo from "../assets/images/cursor.png";
import { PreviewChatFile, ProcessSingleFile, ReadExistingContrail, IgnoreChat, UnignoreChat, CreateCategory, RenameCategory, DeleteCategory, AssignCategory, UnassignCategory } from "../../wailsjs/go/main/App";
import type { useChatFilesCache } from "../hooks/useChatFilesCache";

function renderTextSegment(text: string, key: number): React.ReactNode {
  if (!text.includes('|')) {
    return <span key={key} className="chat-preview-text">{text}</span>;
  }

  const lines = text.split('\n');
  const parts: React.ReactNode[] = [];
  let i = 0;
  let partKey = 0;
  let textBuffer = '';

  while (i < lines.length) {
    const line = lines[i];
    const nextLine = i + 1 < lines.length ? lines[i + 1] : '';

    if (line.trim().startsWith('|') && /^\|[\s\-:|]+\|/.test(nextLine.trim())) {
      if (textBuffer !== '') {
        parts.push(<span key={partKey++} className="chat-preview-text">{textBuffer}</span>);
        textBuffer = '';
      }

      const tableLines: string[] = [];
      while (i < lines.length && lines[i].trim().startsWith('|')) {
        tableLines.push(lines[i]);
        i++;
      }

      const headers = tableLines[0].split('|').slice(1, -1).map(h => h.trim());
      const bodyLines = tableLines.slice(2);
      const rows = bodyLines.map(row => row.split('|').slice(1, -1).map(c => c.trim()));

      parts.push(
        <table key={partKey++} className="chat-preview-table">
          <thead><tr>{headers.map((h, hi) => <th key={hi}>{h}</th>)}</tr></thead>
          <tbody>
            {rows.map((row, ri) => (
              <tr key={ri}>{row.map((cell, ci) => <td key={ci}>{cell}</td>)}</tr>
            ))}
          </tbody>
        </table>
      );
    } else {
      textBuffer += line + (i < lines.length - 1 ? '\n' : '');
      i++;
    }
  }

  if (textBuffer !== '') {
    parts.push(<span key={partKey++} className="chat-preview-text">{textBuffer}</span>);
  }

  return <React.Fragment key={key}>{parts}</React.Fragment>;
}

function renderSectionContent(markdown: string, keyOffset: number): React.ReactNode[] {
  const blockPattern = /(<details>\n<summary>(.*?)<\/summary>\n\n([\s\S]*?)\n<\/details>|<thinking>\n([\s\S]*?)\n<\/thinking>|\{\{CONTRAIL_IMAGE:(image\/[a-z]+;base64,[^}]{100,})\}\})/g;
  const nodes: React.ReactNode[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;
  let key = keyOffset;

  while ((match = blockPattern.exec(markdown)) !== null) {
    if (match.index > lastIndex) {
      nodes.push(renderTextSegment(markdown.slice(lastIndex, match.index), key++));
    }
    if (match[0].startsWith("<details>")) {
      nodes.push(
        <details key={key++} className="chat-preview-details">
          <summary>{match[2]}</summary>
          <span>{match[3]}</span>
        </details>
      );
    } else if (match[0].startsWith("{{CONTRAIL_IMAGE:")) {
      nodes.push(
        <img key={key++} src={`data:${match[5]}`} className="chat-preview-image" style={{ maxWidth: '100%', borderRadius: 6, margin: '8px 0' }} />
      );
    } else {
      nodes.push(<span key={key++} className="chat-preview-thinking">{match[0]}</span>);
    }
    lastIndex = match.index + match[0].length;
  }

  if (lastIndex < markdown.length) {
    nodes.push(renderTextSegment(markdown.slice(lastIndex), key++));
  }

  return nodes;
}

interface MarkdownSection {
  header: string;  // raw header text (without "## "), empty for preamble
  body: string;    // section body
  raw: string;     // full raw text (header line + body) for diffing
}

function splitIntoSections(markdown: string): MarkdownSection[] {
  const sectionPattern = /^(## (?:🧑 User|🤖 Assistant)[^\n]*)/m;
  const parts = markdown.split(sectionPattern);
  const sections: MarkdownSection[] = [];

  // parts alternates: [preamble, header1, body1, header2, body2, ...]
  if (parts[0].trim()) {
    sections.push({ header: '', body: parts[0], raw: parts[0] });
  }

  for (let i = 1; i < parts.length; i += 2) {
    const headerLine = parts[i];
    const header = headerLine.replace(/^## /, '');
    const body = (parts[i + 1] || '').replace(/^\n+/, '');
    sections.push({ header, body, raw: headerLine + '\n' + (parts[i + 1] || '') });
  }

  return sections;
}

function renderMarkdownContent(markdown: string): React.ReactNode {
  const sections = splitIntoSections(markdown);
  const nodes: React.ReactNode[] = [];
  let key = 0;

  for (const section of sections) {
    nodes.push(
      <div key={key++} className="chat-message-section">
        {section.header && <div className="chat-preview-role-header">{section.header}</div>}
        {renderSectionContent(section.body, key * 100)}
      </div>
    );
  }

  return <>{nodes}</>;
}

function renderDiffContent(oldMarkdown: string, newMarkdown: string): React.ReactNode {
  const oldSections = splitIntoSections(oldMarkdown);
  const newSections = splitIntoSections(newMarkdown);

  // Build a map of old sections by raw content for matching
  const oldSet = new Set(oldSections.map(s => s.raw));
  const newSet = new Set(newSections.map(s => s.raw));

  // Walk through both lists to produce a merged diff view.
  // Sections present only in old = removed, only in new = added, in both = unchanged.
  const result: React.ReactNode[] = [];
  let key = 0;
  let oldIdx = 0;
  let newIdx = 0;

  while (oldIdx < oldSections.length || newIdx < newSections.length) {
    const oldSec = oldSections[oldIdx];
    const newSec = newSections[newIdx];

    if (oldSec && newSec && oldSec.raw === newSec.raw) {
      // Unchanged section
      result.push(
        <div key={key++} className="chat-message-section diff-section-unchanged">
          {newSec.header && <div className="chat-preview-role-header">{newSec.header}</div>}
          {renderSectionContent(newSec.body, key * 100)}
        </div>
      );
      oldIdx++;
      newIdx++;
    } else if (oldSec && !newSet.has(oldSec.raw)) {
      // Removed section (in old but not in new)
      result.push(
        <div key={key++} className="chat-message-section diff-section-removed">
          {oldSec.header && <div className="chat-preview-role-header">{oldSec.header}</div>}
          {renderSectionContent(oldSec.body, key * 100)}
        </div>
      );
      oldIdx++;
    } else if (newSec && !oldSet.has(newSec.raw)) {
      // Added section (in new but not in old)
      result.push(
        <div key={key++} className="chat-message-section diff-section-added">
          {newSec.header && <div className="chat-preview-role-header">{newSec.header}</div>}
          {renderSectionContent(newSec.body, key * 100)}
        </div>
      );
      newIdx++;
    } else {
      // Sections exist in both but are out of order — advance whichever
      // appears earlier in the other list (or just advance new)
      if (newSec) {
        result.push(
          <div key={key++} className="chat-message-section diff-section-added">
            {newSec.header && <div className="chat-preview-role-header">{newSec.header}</div>}
            {renderSectionContent(newSec.body, key * 100)}
          </div>
        );
        newIdx++;
      } else {
        oldIdx++;
      }
    }
  }

  return <>{result}</>;
}

type ChatFilesCache = ReturnType<typeof useChatFilesCache>;

interface Props {
  project: Project;
  onToggle: (project: Project) => void;
  onProcess: (project: Project) => void;
  onEdit?: (project: Project, tab?: "vscode" | "claudecode" | "cursor" | "output") => void;
  onUpdateProject?: (project: Project) => void;
  onProjectDataChanged?: () => void;
  processing: string | null;
  processingProgress: ProcessingProgress | null;
  chatFilesCache: ChatFilesCache;
}

function formatDateTime(ms: number): string {
  if (!ms) return "";
  return new Date(ms).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function getFileDisplayName(file: ChatFileInfo): string {
  return file.title || file.fileName;
}

interface PreviewState {
  file: ChatFileInfo;
  markdown: string;
  oldMarkdown?: string;
  loading: boolean;
  processing: boolean;
  processed: boolean;
}

export function ProjectDetail({ project, onToggle, onProcess, onEdit, onUpdateProject, onProjectDataChanged, processing, processingProgress, chatFilesCache }: Props) {
  const isProcessing = processing === project.id;
  const progress = processingProgress?.projectId === project.id ? processingProgress : null;
  const [chatFiles, setChatFiles] = useState<ChatFileInfo[]>(() => chatFilesCache.peekCache(project.id) ?? []);
  const [chatFilesLoading, setChatFilesLoading] = useState(() => !chatFilesCache.peekCache(project.id));
  const [preview, setPreview] = useState<PreviewState | null>(null);
  const [openDirPath, setOpenDirPath] = useState<string | null>(null);
  const [showOpenerDialog, setShowOpenerDialog] = useState(false);
  const [ignoredExpanded, setIgnoredExpanded] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const categories = project.categories ?? [];

  async function handleOpenDir(dirPath: string) {
    const saved = await GetDirectoryOpener();
    if (saved) {
      await OpenDirectoryWith(dirPath, saved);
    } else {
      setOpenDirPath(dirPath);
      setShowOpenerDialog(true);
    }
  }

  const hasVSCode = useMemo(() => project.sources?.some((s) => s.type === "vscode") ?? (project.watchDir !== ""), [project]);
  const hasClaude = useMemo(() => project.sources?.some((s) => s.type === "claudecode") ?? false, [project]);
  const hasCursor = useMemo(() => project.sources?.some((s) => s.type === "cursor") ?? false, [project]);

  const handleRemoveAgent = (type: "vscode" | "claudecode" | "cursor") => {
    if (!onUpdateProject) return;
    const newSources = project.sources?.filter(s => s.type !== type) || [];
    const updated = { ...project, sources: newSources };
    if (type === "vscode") {
      updated.watchDir = "";
    }
    onUpdateProject(updated);
  };

  // Fetch from cache on mount (cache miss triggers IPC, hit returns instantly)
  useEffect(() => {
    let mounted = true;
    if (!chatFilesCache.peekCache(project.id)) {
      setChatFilesLoading(true);
    }
    chatFilesCache.getChatFiles(project.id).then((files) => {
      if (mounted) {
        setChatFiles(files);
        setChatFilesLoading(false);
      }
    }).catch(() => {
      if (mounted) setChatFilesLoading(false);
    });
    return () => { mounted = false; };
  }, [project.id, chatFilesCache]);

  // Sync from cache when it updates (debounced watcher/processed events)
  const cacheVersion = chatFilesCache.versions[project.id];
  useEffect(() => {
    const cached = chatFilesCache.peekCache(project.id);
    if (cached) {
      setChatFiles(cached);
      setChatFilesLoading(false);
    }
  }, [cacheVersion, project.id, chatFilesCache]);

  // Force invalidate cache after batch processing completes
  const prevProcessing = useRef(isProcessing);
  useEffect(() => {
    if (prevProcessing.current && !isProcessing) {
      chatFilesCache.invalidate(project.id);
    }
    prevProcessing.current = isProcessing;
  }, [isProcessing, project.id, chatFilesCache]);

  async function handleShowDetails(file: ChatFileInfo) {
    setPreview({ file, markdown: "", loading: true, processing: false, processed: false });
    try {
      const md = await PreviewChatFile(file.filePath, file.sourceType);

      let oldMarkdown: string | undefined = undefined;
      if (file.partiallyParsed) {
        const oldMd = await ReadExistingContrail(file.fileName, project.outputDir);
        if (oldMd) {
          oldMarkdown = oldMd;
        }
      }

      setPreview((prev) => prev ? { ...prev, markdown: md, oldMarkdown, loading: false } : null);
    } catch {
      setPreview((prev) => prev ? { ...prev, markdown: "Failed to parse file.", loading: false } : null);
    }
  }

  async function handleProcessSingleInPreview(file: ChatFileInfo) {
    setPreview((prev) => prev ? { ...prev, processing: true } : null);
    try {
      await ProcessSingleFile(file.filePath, file.sourceType, project.outputDir);
      setPreview((prev) => prev ? { ...prev, processing: false, processed: true } : null);
      chatFilesCache.invalidate(project.id);
    } catch {
      setPreview((prev) => prev ? { ...prev, processing: false } : null);
    }
  }

  async function handleIgnoreChat(file: ChatFileInfo) {
    await IgnoreChat(project.id, file.filePath, getFileDisplayName(file));
    const updater = (prev: ChatFileInfo[]) => prev.map((f) => f.filePath === file.filePath ? { ...f, ignored: true } : f);
    setChatFiles(updater);
    chatFilesCache.updateCache(project.id, updater);
  }

  async function handleUnignoreChat(file: ChatFileInfo) {
    await UnignoreChat(project.id, file.filePath);
    const updater = (prev: ChatFileInfo[]) => prev.map((f) => f.filePath === file.filePath ? { ...f, ignored: false } : f);
    setChatFiles(updater);
    chatFilesCache.updateCache(project.id, updater);
  }

  function markFileProcessed(filePath: string) {
    const updater = (prev: ChatFileInfo[]) =>
      prev.map((f) =>
        f.filePath === filePath
          ? { ...f, parsed: true, partiallyParsed: false, processedAt: Date.now() }
          : f
      );
    setChatFiles(updater);
    chatFilesCache.updateCache(project.id, updater);
  }

  const handleCreateCategory = useCallback(async (name: string): Promise<Category> => {
    const cat = await CreateCategory(project.id, name);
    onProjectDataChanged?.();
    return cat;
  }, [project.id, onProjectDataChanged]);

  const handleRenameCategory = useCallback(async (categoryId: string, newName: string) => {
    await RenameCategory(project.id, categoryId, newName);
    onProjectDataChanged?.();
  }, [project.id, onProjectDataChanged]);

  const handleDeleteCategory = useCallback(async (categoryId: string) => {
    await DeleteCategory(project.id, categoryId);
    const updater = (prev: ChatFileInfo[]) => prev.map((f) => f.categoryId === categoryId ? { ...f, categoryId: undefined } : f);
    setChatFiles(updater);
    chatFilesCache.updateCache(project.id, updater);
    onProjectDataChanged?.();
  }, [project.id, onProjectDataChanged, chatFilesCache]);

  const handleAssignCategory = useCallback(async (file: ChatFileInfo, categoryId: string) => {
    await AssignCategory(project.id, file.filePath, categoryId);
    const updater = (prev: ChatFileInfo[]) => prev.map((f) => f.filePath === file.filePath ? { ...f, categoryId } : f);
    setChatFiles(updater);
    chatFilesCache.updateCache(project.id, updater);
    onProjectDataChanged?.();
  }, [project.id, onProjectDataChanged, chatFilesCache]);

  const handleUnassignCategory = useCallback(async (file: ChatFileInfo) => {
    await UnassignCategory(project.id, file.filePath);
    const updater = (prev: ChatFileInfo[]) => prev.map((f) => f.filePath === file.filePath ? { ...f, categoryId: undefined } : f);
    setChatFiles(updater);
    chatFilesCache.updateCache(project.id, updater);
    onProjectDataChanged?.();
  }, [project.id, onProjectDataChanged, chatFilesCache]);

  const sortByCreated = (a: ChatFileInfo, b: ChatFileInfo) => (b.createdAt || 0) - (a.createdAt || 0);
  const matchesSearch = (f: ChatFileInfo) => {
    if (!searchQuery) return true;
    const q = searchQuery.toLowerCase();
    return (f.title || f.fileName || "").toLowerCase().includes(q);
  };
  const activeFiles = chatFiles.filter((f) => !f.ignored);
  const ignoredFiles = chatFiles.filter((f) => f.ignored && matchesSearch(f)).sort(sortByCreated);
  const parsedFiles = activeFiles.filter((f) => f.parsed && !f.partiallyParsed && matchesSearch(f)).sort(sortByCreated);
  const partiallyParsedFiles = activeFiles.filter((f) => f.partiallyParsed && matchesSearch(f)).sort(sortByCreated);
  const unparsedFiles = activeFiles.filter((f) => !f.parsed && !f.partiallyParsed && matchesSearch(f)).sort(sortByCreated);

  // Group processed files by category (only non-empty groups, sorted alphabetically)
  const categoryGroups = useMemo(() => {
    return [...categories]
      .sort((a, b) => a.name.localeCompare(b.name))
      .map((cat) => ({ category: cat, files: parsedFiles.filter((f) => f.categoryId === cat.id) }))
      .filter((g) => g.files.length > 0);
  }, [categories, parsedFiles]);

  const uncategorizedParsed = useMemo(
    () => parsedFiles.filter((f) => !f.categoryId),
    [parsedFiles]
  );

  return (
    <div className="project-detail">
      {preview && (
        <div className="chat-preview" style={{ position: 'fixed', top: 36, left: 'var(--sidebar-width)', right: 0, bottom: 0, zIndex: 10, background: 'var(--bg-base)' }}>
          <div className="chat-preview-header">
            <button className="btn btn-ghost btn-sm" onClick={() => setPreview(null)}>
              <ChevronLeft size={14} /> Back
            </button>
            <span className="chat-preview-title mono">{getFileDisplayName(preview.file)}</span>
            {!preview.file.parsed && !preview.processed && (
              <button
                className="btn btn-primary btn-sm"
                onClick={() => handleProcessSingleInPreview(preview.file)}
                disabled={preview.processing || preview.loading}
              >
                {preview.processing ? <><Loader2 size={12} className="spin" /> Processing…</> : <><Play size={12} /> Process Now</>}
              </button>
            )}
            {preview.processed && (
              <span className="chat-processed-badge"><CheckCircle2 size={12} /> Processed</span>
            )}
          </div>
          <div className="chat-preview-body">
            {preview.loading ? (
              <div className="chat-preview-loading"><Loader2 size={18} className="spin" /> Parsing…</div>
            ) : preview.oldMarkdown ? (
              <div className="chat-preview-markdown diff-view">{renderDiffContent(preview.oldMarkdown, preview.markdown)}</div>
            ) : (
              <div className="chat-preview-markdown">{renderMarkdownContent(preview.markdown)}</div>
            )}
          </div>
        </div>
      )}
      <div>
      <div className="detail-header">
        <h1>{project.name}</h1>
        <div className="detail-badges">
          <span className={`badge ${project.active ? "badge-active" : "badge-paused"}`}>
            {project.active ? (
              <>
                <Eye size={12} /> Watching
              </>
            ) : (
              <>
                <EyeOff size={12} /> Paused
              </>
            )}
          </span>
          {project.workspacePath?.endsWith(".code-workspace") && (
            <span className="badge badge-workspace">
              <Layers size={12} /> Workspace
            </span>
          )}
        </div>
      </div>

      <div className="detail-cards">
        {hasVSCode && (project.sources?.find(s => s.type === "vscode")?.watchDir || project.watchDir) && (
          <div className="detail-card">
            <div className="detail-card-icon">
              <FolderOpen size={18} />
            </div>
            <div className="detail-card-content" style={{ flex: 1 }}>
              <span className="detail-card-label" style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                Watching Directory
                <img src={copilotLogo} alt="GitHub Copilot" className="icon-invert" style={{ height: '22px', width: '22px', objectFit: 'contain' }} title="GitHub Copilot" />
              </span>
              <span className="detail-card-value mono">{project.sources?.find(s => s.type === "vscode")?.watchDir || project.watchDir}</span>
            </div>
            {onEdit && (
              <div style={{ display: 'flex', gap: '4px', opacity: 0.7 }}>
                <button className="btn btn-ghost btn-sm" style={{ padding: '0 6px' }} onClick={() => onEdit(project, 'vscode')} title="Edit watching directory">
                  <Pencil size={14} />
                </button>
                {(hasClaude || hasCursor) && onUpdateProject && (
                  <button className="btn btn-ghost btn-sm" style={{ padding: '0 6px', color: 'var(--red)' }} onClick={() => handleRemoveAgent('vscode')} title="Remove GitHub Copilot">
                    <Trash2 size={14} />
                  </button>
                )}
              </div>
            )}
          </div>
        )}

        {hasClaude && project.sources?.find(s => s.type === "claudecode")?.watchDir && (
          <div className="detail-card">
            <div className="detail-card-icon">
              <FolderOpen size={18} />
            </div>
            <div className="detail-card-content" style={{ flex: 1 }}>
              <span className="detail-card-label" style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                Watching Directory
                <img src={claudeLogo} alt="Claude Code" style={{ height: '18px', width: '32px', objectFit: 'contain' }} title="Claude Code" />
              </span>
              <span className="detail-card-value mono">{project.sources?.find(s => s.type === "claudecode")?.watchDir}</span>
            </div>
            {onEdit && (
              <div style={{ display: 'flex', gap: '4px', opacity: 0.7 }}>
                <button className="btn btn-ghost btn-sm" style={{ padding: '0 6px' }} onClick={() => onEdit(project, 'claudecode')} title="Edit watching directory">
                  <Pencil size={14} />
                </button>
                {(hasVSCode || hasCursor) && onUpdateProject && (
                  <button className="btn btn-ghost btn-sm" style={{ padding: '0 6px', color: 'var(--red)' }} onClick={() => handleRemoveAgent('claudecode')} title="Remove Claude Code">
                    <Trash2 size={14} />
                  </button>
                )}
              </div>
            )}
          </div>
        )}

        {hasCursor && project.sources?.find(s => s.type === "cursor")?.watchDir && (
          <>
          <div className="detail-card">
            <div className="detail-card-icon">
              <FolderOpen size={18} />
            </div>
            <div className="detail-card-content" style={{ flex: 1 }}>
              <span className="detail-card-label" style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                Watching Directory
                <img src={cursorLogo} alt="Cursor" style={{ height: '20px', width: '20px', objectFit: 'contain', borderRadius: '4px' }} title="Cursor" />
              </span>
              <span className="detail-card-value mono">{project.sources?.find(s => s.type === "cursor")?.watchDir}</span>
            </div>

            {onEdit && (
              <div style={{ display: 'flex', gap: '4px', opacity: 0.7 }}>
                <button className="btn btn-ghost btn-sm" style={{ padding: '0 6px' }} onClick={() => onEdit(project, 'cursor')} title="Edit watching directory">
                  <Pencil size={14} />
                </button>
                {(hasVSCode || hasClaude) && onUpdateProject && (
                  <button className="btn btn-ghost btn-sm" style={{ padding: '0 6px', color: 'var(--red)' }} onClick={() => handleRemoveAgent('cursor')} title="Remove Cursor">
                    <Trash2 size={14} />
                  </button>
                )}
              </div>
            )}
          </div>
          {hasCursor && project.sources?.find(s => s.type === "cursor")?.watchDir &&
          <div className="detail-info-message">
            <p>Cursor contrails might take up to 1 minute to appear</p>
          </div>
          }
          </>
        )}

        {(!hasVSCode || !hasClaude || !hasCursor) && onEdit && (
            <button className="btn btn-outline btn-sm" style={{ width: 'fit-content', alignSelf: 'center' }} onClick={() => onEdit(project)}>+ Configure new agent</button>
        )}

        <div className="detail-card">
          <div className="detail-card-icon">
            <MapPin size={18} />
          </div>
          <div className="detail-card-content" style={{ flex: 1 }}>
            <span className="detail-card-label">
              Output Directory
              <span style={{ display: 'inline-flex', gap: '2px', marginLeft: '6px', verticalAlign: 'middle' }}>
                <button className="btn btn-ghost btn-sm" style={{ padding: '0 4px', height: '20px', minHeight: 'unset' }} onClick={() => handleOpenDir(project.outputDir)} title="Open output directory">
                  <ExternalLink size={12} />
                </button>
                <button className="btn btn-ghost btn-sm" style={{ padding: '0 4px', height: '20px', minHeight: 'unset' }} onClick={() => handleOpenDir(project.outputDir.replace(/\/[^/]*\/?$/, ''))} title="Open parent directory">
                  <FolderUp size={12} />
                </button>
              </span>
            </span>
            <span className="detail-card-value mono">{project.outputDir}</span>
          </div>
          {onEdit && (
            <div style={{ display: 'flex', gap: '4px', opacity: 0.7 }}>
              <button className="btn btn-ghost btn-sm" style={{ padding: '0 6px' }} onClick={() => onEdit(project, 'output')} title="Edit output directory">
                <Pencil size={14} />
              </button>
            </div>
          )}
        </div>
      </div>

      <div className="detail-info-message">
        {project.active ? (
          <p>
            Watching for <strong>new</strong> changes.
            <br />
            Use &ldquo;Process All Now&rdquo; to import existing sessions, or process individual chats below.
          </p>
        ) : project.pausedAt ? (
          <p className="paused-message">
            <EyeOff size={12} />
            Watching paused on {formatDateTime(project.pausedAt)}
          </p>
        ) : (
          <p>Watching is paused. Resume to start tracking changes.</p>
        )}
      </div>

      <div className="detail-actions">
        <button
          className="btn btn-outline"
          onClick={() => onToggle(project)}
        >
          {project.active ? (
            <>
              <EyeOff size={14} /> Pause Watching
            </>
          ) : (
            <>
              <Eye size={14} /> Resume Watching
            </>
          )}
        </button>

        {(chatFilesLoading || chatFiles.length > 0) && (
          <button
            className="btn btn-primary"
            onClick={() => onProcess(project)}
            disabled={isProcessing}
          >
            {isProcessing ? (
              <>
                <Loader2 size={14} className="spin" />
                {progress
                  ? `Processing ${progress.current}/${progress.total}…`
                  : "Processing…"}
              </>
            ) : (
              <>
                <Play size={14} /> Process All Now
              </>
            )}
          </button>
        )}
      </div>
</div>
<div>

      {/* Chat files list */}
      {chatFilesLoading ? (
        <div className="chat-files-section">
          <h3 className="chat-files-heading">Chat Sessions</h3>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '16px 0', opacity: 0.6 }}>
            <Loader2 size={16} className="spin" /> Loading…
          </div>
        </div>
      ) : chatFiles.length > 0 ? (
        <div className="chat-files-section">
          <div className="chat-files-header">
            <h3 className="chat-files-heading">Chat Sessions ({activeFiles.length})</h3>
            <div className="chat-files-search">
              <Search size={12} />
              <input
                type="text"
                placeholder="Search…"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="chat-files-search-input"
              />
            </div>
          </div>
          {parsedFiles.length > 0 && (
            <div className="chat-files-group chat-files-group-processed">
              <div className="chat-files-group-label">Processed ({parsedFiles.length})</div>
              <div className="chat-files-group-hint">Click any item below to show its content</div>

              {/* Uncategorized */}
              {uncategorizedParsed.length > 0 && (
                <div className="chat-files-category-group">
                {categoryGroups.length > 0 && (
                  <div className="chat-files-category-header">
                    <span>- Unassigned -</span>
                  </div>
                )}
                <div className="chat-files-scroll">
                  {uncategorizedParsed.map((file) => (
                    <ChatFileRow
                      key={file.filePath}
                      file={file}
                      onShowDetails={() => handleShowDetails(file)}
                      onProcessed={() => markFileProcessed(file.filePath)}
                      outputDir={project.outputDir}
                      onIgnore={() => handleIgnoreChat(file)}
                      categories={categories}
                      onAssignCategory={(catId) => handleAssignCategory(file, catId)}
                      onUnassignCategory={() => handleUnassignCategory(file)}
                      onCreateCategory={handleCreateCategory}
                      onRenameCategory={handleRenameCategory}
                    />
                  ))}
                </div>
                </div>
              )}

              {/* Category groups */}
              {categoryGroups.map(({ category, files }) => (
                <div key={category.id} className="chat-files-category-group">
                  <CategoryGroupHeader
                    category={category}
                    onRename={(newName) => handleRenameCategory(category.id, newName)}
                    onDelete={() => handleDeleteCategory(category.id)}
                  />
                  <div className="chat-files-scroll">
                    {files.map((file) => (
                      <ChatFileRow
                        key={file.filePath}
                        file={file}
                        onShowDetails={() => handleShowDetails(file)}
                        onProcessed={() => markFileProcessed(file.filePath)}
                        outputDir={project.outputDir}
                        onIgnore={() => handleIgnoreChat(file)}
                        categories={categories}
                        onAssignCategory={(catId) => handleAssignCategory(file, catId)}
                        onUnassignCategory={() => handleUnassignCategory(file)}
                        onCreateCategory={handleCreateCategory}
                        onRenameCategory={handleRenameCategory}
                      />
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}
          {partiallyParsedFiles.length > 0 && (
            <div className="chat-files-group chat-files-group-partially-processed">
              <div className="chat-files-group-label">Partially processed ({partiallyParsedFiles.length})</div>
              <div className="chat-files-scroll">
                {partiallyParsedFiles.map((file) => (
                  <ChatFileRow
                    key={file.filePath}
                    file={file}
                    onShowDetails={() => handleShowDetails(file)}
                    onProcessed={() => markFileProcessed(file.filePath)}
                    outputDir={project.outputDir}
                    onIgnore={() => handleIgnoreChat(file)}
                    categories={categories}
                  />
                ))}
              </div>
            </div>
          )}
          {unparsedFiles.length > 0 && (
            <div className="chat-files-group chat-files-group-unprocessed">
              <div className="chat-files-group-label">Not yet processed ({unparsedFiles.length})</div>
              <div className="chat-files-group-hint">Click any item below to show its content</div>
              <div className="chat-files-scroll">
                {unparsedFiles.map((file) => (
                  <ChatFileRow
                    key={file.filePath}
                    file={file}
                    onShowDetails={() => handleShowDetails(file)}
                    onProcessed={() => markFileProcessed(file.filePath)}
                    outputDir={project.outputDir}
                    onIgnore={() => handleIgnoreChat(file)}
                  />
                ))}
              </div>
            </div>
          )}
          {ignoredFiles.length > 0 && (
            <div className="chat-files-group chat-files-group-ignored">
              <div
                className="chat-files-group-label chat-files-group-label-collapsible"
                onClick={() => setIgnoredExpanded((v) => !v)}
                style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '4px' }}
              >
                {ignoredExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                Ignored ({ignoredFiles.length})
              </div>
              {ignoredExpanded && (
                <div className="chat-files-scroll">
                  {ignoredFiles.map((file) => (
                    <ChatFileRow
                      key={file.filePath}
                      file={file}
                      onShowDetails={() => handleShowDetails(file)}
                      onProcessed={() => markFileProcessed(file.filePath)}
                      outputDir={project.outputDir}
                      onUnignore={() => handleUnignoreChat(file)}
                    />
                  ))}
                </div>
              )}
            </div>
          )}
          {searchQuery && parsedFiles.length === 0 && partiallyParsedFiles.length === 0 && unparsedFiles.length === 0 && ignoredFiles.length === 0 && (
            <div style={{ padding: '8px 0', opacity: 0.5, fontSize: '13px' }}>No results for "{searchQuery}"</div>
          )}
        </div>
      ) : (
        <div className="chat-files-section">
          <h3 className="chat-files-heading">Chat Sessions</h3>
          <div style={{ padding: '16px 0', opacity: 0.5 }}>No chats yet…</div>
        </div>
      )}
    </div>
    {showOpenerDialog && (
      <DirectoryOpenerDialog
        dirPath={openDirPath}
        onClose={() => setShowOpenerDialog(false)}
      />
    )}
    </div>
  );
}

// ─── Category group header with inline rename ────────────────────────────────

interface CategoryGroupHeaderProps {
  category: Category;
  onRename: (newName: string) => void;
  onDelete: () => void;
}

function CategoryGroupHeader({ category, onRename, onDelete }: CategoryGroupHeaderProps) {
  const [renaming, setRenaming] = useState(false);
  const [value, setValue] = useState(category.name);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (renaming) inputRef.current?.focus();
  }, [renaming]);

  function commit() {
    const trimmed = value.trim();
    if (trimmed && trimmed !== category.name) onRename(trimmed);
    else setValue(category.name);
    setRenaming(false);
  }

  return (
    <div className="chat-files-category-header">
      {renaming ? (
        <input
          ref={inputRef}
          className="chat-files-category-rename-input"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onBlur={commit}
          onKeyDown={(e) => { if (e.key === "Enter") commit(); if (e.key === "Escape") { setValue(category.name); setRenaming(false); } }}
        />
      ) : (
        <span style={{ flex: 1 }}>{category.name}</span>
      )}
      <div className="chat-files-category-header-actions">
        {renaming ? (
          <button
            className="btn btn-ghost btn-sm"
            style={{ padding: '0 3px', opacity: 0.6 }}
            title="Confirm rename"
            onMouseDown={(e) => { e.preventDefault(); commit(); }}
          >
            <Check size={11} />
          </button>
        ) : (
          <>
            <button
              className="btn btn-ghost btn-sm"
              style={{ padding: '0 3px', opacity: 0.6 }}
              title="Rename category"
              onClick={() => { setValue(category.name); setRenaming(true); }}
            >
              <Pencil size={11} />
            </button>
            <button
              className="btn btn-ghost btn-sm"
              style={{ padding: '0 3px', opacity: 0.6, color: 'var(--red)' }}
              title="Delete category"
              onClick={onDelete}
            >
              <Trash2 size={11} />
            </button>
          </>
        )}
      </div>
    </div>
  );
}

// ─── Shared positioning helper ───────────────────────────────────────────────

function computePosition(rect: DOMRect, width: number, estimatedHeight: number): { top: number; left: number } {
  const margin = 8;
  const gap = 4;
  const spaceBelow = window.innerHeight - rect.bottom - margin;
  const top = spaceBelow >= estimatedHeight
    ? rect.bottom + gap
    : Math.max(margin, rect.top - estimatedHeight - gap);
  const left = Math.max(margin, Math.min(rect.right - width, window.innerWidth - width - margin));
  return { top, left };
}

// ─── ChatFileRow ─────────────────────────────────────────────────────────────

interface ChatFileRowProps {
  file: ChatFileInfo;
  onShowDetails: () => void;
  onProcessed: () => void;
  outputDir: string;
  onIgnore?: () => void;
  onUnignore?: () => void;
  // Category props — only provided for processed rows; partially-processed rows
  // get categories for the badge display only (no assign actions)
  categories?: Category[];
  onAssignCategory?: (categoryId: string) => void;
  onUnassignCategory?: () => void;
  onCreateCategory?: (name: string) => Promise<Category>;
  onRenameCategory?: (id: string, newName: string) => void;
}

function ChatFileRow({ file, onShowDetails, onProcessed, outputDir, onIgnore, onUnignore, categories, onAssignCategory, onUnassignCategory, onCreateCategory, onRenameCategory }: ChatFileRowProps) {
  const [processing, setProcessing] = useState(false);
  const [menuPos, setMenuPos] = useState<{ top: number; left: number } | null>(null);
  const [pickerAnchorRect, setPickerAnchorRect] = useState<DOMRect | null>(null);
  const menuBtnRef = useRef<HTMLButtonElement>(null);

  async function handleProcess() {
    setProcessing(true);
    try {
      await ProcessSingleFile(file.filePath, file.sourceType, outputDir);
      onProcessed();
    } finally {
      setProcessing(false);
    }
  }

  const isParsed = file.parsed && !file.partiallyParsed;
  const isPartiallyParsed = file.partiallyParsed;
  const canManageCategory = isParsed && !!onAssignCategory;

  function openMenu() {
    if (!menuBtnRef.current) return;
    setMenuPos(computePosition(menuBtnRef.current.getBoundingClientRect(), 160, 130));
  }

  function openPicker() {
    if (!menuBtnRef.current) return;
    setPickerAnchorRect(menuBtnRef.current.getBoundingClientRect());
    setMenuPos(null);
  }

  const currentCategory = categories?.find((c) => c.id === file.categoryId);

  return (
    <div className={`chat-file-row ${isParsed ? "chat-file-row-parsed" : ""}`}>
      <div className="chat-file-icon">
        {isParsed
          ? <CheckCircle2 size={14} className="chat-file-check" />
          : isPartiallyParsed
          ? <CheckCircle2 size={14} style={{ color: "orange" }} />
          : null}
      </div>
      <div style={{ display: 'flex', alignItems: 'center', marginLeft: '2px', marginRight: '6px' }} title={file.sourceType === 'vscode' ? 'GitHub Copilot' : file.sourceType === 'claudecode' ? 'Claude Code' : ''}>
        {file.sourceType === 'vscode' && <img src={copilotLogo} alt="GitHub Copilot" className="icon-invert" style={{ height: '20px', width: '20px', objectFit: 'contain' }} />}
        {file.sourceType === 'claudecode' && <img src={claudeLogo} alt="Claude Code" style={{ height: '18px', width: '26px', objectFit: 'contain' }} />}
        {file.sourceType === 'cursor' && <img src={cursorLogo} alt="Cursor" style={{ height: '18px', width: '18px', objectFit: 'contain', borderRadius: '4px' }} />}
      </div>
      <span className="chat-file-name chat-file-name-clickable" title={file.filePath} onClick={onShowDetails}>{getFileDisplayName(file)}</span>

      {/* Category badge on partially-processed rows */}
      {isPartiallyParsed && currentCategory && (
        <span className="category-tag" title={`Category: ${currentCategory.name}`}>
          <Tag size={9} />
          {currentCategory.name}
        </span>
      )}

      <div className="chat-file-actions">
        {onUnignore ? (
          <button className="btn btn-outline btn-sm" onClick={onUnignore} title="Stop ignoring this chat">
            <Eye size={12} /> Unignore
          </button>
        ) : isParsed ? (
          <>
            {file.processedAt > 0 && (
              <span className="chat-file-processed-time">{formatDateTime(file.processedAt)}</span>
            )}
            <button
              ref={menuBtnRef}
              className="btn btn-ghost btn-sm"
              style={{ padding: '0 4px' }}
              title="More actions"
              onClick={openMenu}
            >
              <MoreHorizontal size={14} />
            </button>
            {menuPos && createPortal(
              <>
                <div className="chat-menu-overlay" onClick={() => setMenuPos(null)} />
                <div className="chat-menu" style={{ top: menuPos.top, left: menuPos.left }}>
                  <div className="chat-menu-item" onClick={() => { setMenuPos(null); handleProcess(); }}>
                    <Play size={12} />
                    {processing ? "Processing…" : "Re-process"}
                  </div>
                  {canManageCategory && (
                    <div className="chat-menu-item" onClick={() => openPicker()}>
                      <Tag size={12} />
                      {file.categoryId ? "Change category" : "Add to category"}
                    </div>
                  )}
                  <div className="chat-menu-divider" />
                  {onIgnore && (
                    <div className="chat-menu-item chat-menu-item-danger" onClick={() => { setMenuPos(null); onIgnore(); }}>
                      <EyeOff size={12} />
                      Ignore
                    </div>
                  )}
                </div>
              </>,
              document.body
            )}
            {pickerAnchorRect && canManageCategory && createPortal(
              <CategoryPicker
                categories={categories!}
                currentCategoryId={file.categoryId}
                anchorRect={pickerAnchorRect}
                onAssign={(catId) => { setPickerAnchorRect(null); onAssignCategory!(catId); }}
                onUnassign={() => { setPickerAnchorRect(null); onUnassignCategory!(); }}
                onCreate={onCreateCategory!}
                onRename={onRenameCategory!}
                onClose={() => setPickerAnchorRect(null)}
              />,
              document.body
            )}
          </>
        ) : (
          <>
            {isPartiallyParsed && (
              <button className="btn btn-outline btn-sm" onClick={onShowDetails}>
                Show Diff
              </button>
            )}
            <button
              className="btn btn-primary btn-sm"
              onClick={handleProcess}
              disabled={processing}
            >
              {processing ? (
                <><Loader2 size={12} className="spin" /> Processing…</>
              ) : (
                <><Play size={12} /> Process Now</>
              )}
            </button>
            {onIgnore && (
              <button className="btn btn-ghost btn-sm" onClick={onIgnore} title="Ignore this chat" style={{ opacity: 0.5, padding: '0 4px' }}>
                <EyeOff size={12} />
              </button>
            )}
          </>
        )}
      </div>
    </div>
  );
}

// ─── CategoryPicker popover ───────────────────────────────────────────────────

interface CategoryPickerProps {
  categories: Category[];
  currentCategoryId?: string;
  anchorRect: DOMRect;
  onAssign: (categoryId: string) => void;
  onUnassign: () => void;
  onCreate: (name: string) => Promise<Category>;
  onRename: (id: string, newName: string) => void;
  onClose: () => void;
}

const PICKER_WIDTH = 224;

function CategoryPicker({ categories, currentCategoryId, anchorRect, onAssign, onUnassign, onCreate, onRename, onClose }: CategoryPickerProps) {
  const [newName, setNewName] = useState("");
  const [renamingId, setRenamingId] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState("");
  const pickerRef = useRef<HTMLDivElement>(null);
  const renameInputRef = useRef<HTMLInputElement>(null);
  const newInputRef = useRef<HTMLInputElement>(null);

  // Compute left (horizontal) immediately — doesn't depend on height
  const left = Math.max(8, Math.min(anchorRect.right - PICKER_WIDTH, window.innerWidth - PICKER_WIDTH - 8));

  // Start off-screen invisible, then measure actual height and snap into place
  const [top, setTop] = useState<number | null>(null);

  useLayoutEffect(() => {
    if (!pickerRef.current) return;
    const height = pickerRef.current.offsetHeight;
    const margin = 8;
    const gap = 4;
    const spaceBelow = window.innerHeight - anchorRect.bottom - margin;
    const computedTop = spaceBelow >= height
      ? anchorRect.bottom + gap
      : Math.max(margin, anchorRect.top - height - gap);
    setTop(computedTop);
    // Focus the new-category input after positioning
    newInputRef.current?.focus();
  }, [anchorRect]);

  useEffect(() => {
    if (renamingId) renameInputRef.current?.focus();
  }, [renamingId]);

  async function handleCreate(e: React.KeyboardEvent) {
    if (e.key !== "Enter") return;
    const trimmed = newName.trim();
    if (!trimmed) return;
    const cat = await onCreate(trimmed);
    setNewName("");
    onAssign(cat.id);
  }

  function startRename(cat: Category) {
    setRenamingId(cat.id);
    setRenameValue(cat.name);
  }

  function commitRename(id: string) {
    const trimmed = renameValue.trim();
    if (trimmed && trimmed !== categories.find((c) => c.id === id)?.name) {
      onRename(id, trimmed);
    }
    setRenamingId(null);
  }

  return (
    <>
      <div className="category-picker-overlay" onClick={onClose} />
      <div
        ref={pickerRef}
        className="category-picker"
        style={{ left, top: top ?? -9999, visibility: top === null ? 'hidden' : 'visible' }}
      >
        <div className="category-picker-title">
          {currentCategoryId ? "Change category" : "Add to category"}
        </div>

        {categories.length === 0 && (
          <div style={{ padding: '4px 12px 4px 14px', fontSize: '11px', color: 'var(--text-muted)', fontStyle: 'italic', opacity: 0.7 }}>
            No categories yet
          </div>
        )}

        {categories.map((cat) => (
          <div key={cat.id} className="category-picker-item">
            {renamingId === cat.id ? (
              <input
                ref={renameInputRef}
                className="category-picker-rename-input"
                value={renameValue}
                onChange={(e) => setRenameValue(e.target.value)}
                onBlur={() => commitRename(cat.id)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") commitRename(cat.id);
                  if (e.key === "Escape") setRenamingId(null);
                  e.stopPropagation();
                }}
              />
            ) : (
              <span
                className={`category-picker-item-name ${cat.id === currentCategoryId ? "category-picker-item-selected" : ""}`}
                onClick={() => cat.id !== currentCategoryId && onAssign(cat.id)}
              >
                {cat.name}
              </span>
            )}
            {cat.id === currentCategoryId && renamingId !== cat.id && (
              <Check size={12} style={{ color: 'var(--accent)', flexShrink: 0 }} />
            )}
            <div className="category-picker-item-actions">
              <button
                className="btn btn-ghost btn-sm"
                style={{ padding: '0 3px' }}
                title="Rename"
                onClick={(e) => { e.stopPropagation(); startRename(cat); }}
              >
                <Pencil size={11} />
              </button>
            </div>
          </div>
        ))}

        {currentCategoryId && (
          <>
            <div className="category-picker-divider" />
            <div className="category-picker-unassign" onClick={onUnassign}>
              <X size={12} /> Remove from category
            </div>
          </>
        )}

        <div className="category-picker-divider" />
        <div className="category-picker-new">
          <Plus size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
          <input
            ref={newInputRef}
            className="category-picker-new-input"
            placeholder="New category…"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            onKeyDown={(e) => { handleCreate(e); e.stopPropagation(); }}
          />
        </div>
      </div>
    </>
  );
}
