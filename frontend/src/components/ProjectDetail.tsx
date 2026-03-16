import { useState, useEffect, useMemo } from "react";
import { FolderOpen, FolderUp, Eye, EyeOff, MapPin, Play, Loader2, Layers, CheckCircle2, ChevronLeft, FileText, Pencil, Trash2, ExternalLink, ChevronDown, ChevronRight } from "lucide-react";
import { OpenDirectoryWith, GetDirectoryOpener } from "../../wailsjs/go/main/App";
import { DirectoryOpenerDialog } from "./DirectoryOpenerDialog";
import { Project, ProcessingProgress, ChatFileInfo } from "../types";
import copilotLogo from "../assets/images/gh-copilot.png";
import claudeLogo from "../assets/images/claude.png";
import cursorLogo from "../assets/images/cursor.png";
import { ListChatFiles, PreviewChatFile, ProcessSingleFile, ReadExistingContrail, IgnoreChat, UnignoreChat } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { diffLines, Change } from "diff";

interface Props {
  project: Project;
  onToggle: (project: Project) => void;
  onProcess: (project: Project) => void;
  onEdit?: (project: Project, tab?: "vscode" | "claudecode" | "cursor" | "output") => void;
  onUpdateProject?: (project: Project) => void;
  processing: string | null;
  processingProgress: ProcessingProgress | null;
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
  diffs?: Change[];
  loading: boolean;
  processing: boolean;
  processed: boolean;
}

export function ProjectDetail({ project, onToggle, onProcess, onEdit, onUpdateProject, processing, processingProgress }: Props) {
  const isProcessing = processing === project.id;
  const progress = processingProgress?.projectId === project.id ? processingProgress : null;
  const [chatFiles, setChatFiles] = useState<ChatFileInfo[]>([]);
  const [chatFilesLoading, setChatFilesLoading] = useState(true);
  const [filesVersion, setFilesVersion] = useState(0);
  const [preview, setPreview] = useState<PreviewState | null>(null);
  const [openDirPath, setOpenDirPath] = useState<string | null>(null);
  const [showOpenerDialog, setShowOpenerDialog] = useState(false);
  const [ignoredExpanded, setIgnoredExpanded] = useState(false);

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

  useEffect(() => {
    let mounted = true;
    setChatFilesLoading(true);
    ListChatFiles(project.id).then((files) => {
      if (mounted) setChatFiles(files ?? []);
    }).catch(() => {
      if (mounted) setChatFiles([]);
    }).finally(() => {
      if (mounted) setChatFilesLoading(false);
    });
    return () => { mounted = false; };
  }, [project.id, isProcessing, filesVersion, project.sources, project.watchDir]);

  useEffect(() => {
    const unbindProcessed = EventsOn("file:processed", (event: { projectId: string }) => {
      if (event.projectId === project.id) {
        setFilesVersion((v) => v + 1);
      }
    });

    const unbindWatcher = EventsOn("watcher:event", (event: { projectId: string }) => {
      if (event.projectId === project.id) {
        // Refresh when files are created/modified so new titles show up
        setFilesVersion((v) => v + 1);
      }
    });

    const unbindCursor = EventsOn("cursor:changed", (event: { projectId: string }) => {
      if (event.projectId === project.id) {
        // Refresh list so new/updated chats appear immediately
        setFilesVersion((v) => v + 1);
      }
    });

    return () => {
      unbindProcessed();
      unbindWatcher();
      unbindCursor();
    };
  }, [project.id]);

  async function handleShowDetails(file: ChatFileInfo) {
    setPreview({ file, markdown: "", loading: true, processing: false, processed: false });
    try {
      const md = await PreviewChatFile(file.filePath, file.sourceType);
      
      let diffs: Change[] | undefined = undefined;
      if (file.partiallyParsed) {
        const oldMd = await ReadExistingContrail(file.fileName, project.outputDir);
        if (oldMd) {
          diffs = diffLines(oldMd, md);
        }
      }

      setPreview((prev) => prev ? { ...prev, markdown: md, diffs, loading: false } : null);
    } catch {
      setPreview((prev) => prev ? { ...prev, markdown: "Failed to parse file.", loading: false } : null);
    }
  }

  async function handleProcessSingleInPreview(file: ChatFileInfo) {
    setPreview((prev) => prev ? { ...prev, processing: true } : null);
    try {
      await ProcessSingleFile(file.filePath, file.sourceType, project.outputDir);
      setPreview((prev) => prev ? { ...prev, processing: false, processed: true } : null);
      setFilesVersion((v) => v + 1);
    } catch {
      setPreview((prev) => prev ? { ...prev, processing: false } : null);
    }
  }

  if (preview) {
    return (
      <div className="project-detail">
        <div className="chat-preview">
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
            ) : preview.diffs ? (
              <pre className="chat-preview-markdown diff-view">
                {preview.diffs.map((part, index) => {
                  const className = part.added ? "diff-added" : part.removed ? "diff-removed" : "diff-unchanged";
                  return <span key={index} className={className}>{part.value}</span>;
                })}
              </pre>
            ) : (
              <pre className="chat-preview-markdown">{preview.markdown}</pre>
            )}
          </div>
        </div>
      </div>
    );
  }

  async function handleIgnoreChat(file: ChatFileInfo) {
    await IgnoreChat(project.id, file.filePath, getFileDisplayName(file));
    setFilesVersion((v) => v + 1);
  }

  async function handleUnignoreChat(file: ChatFileInfo) {
    await UnignoreChat(project.id, file.filePath);
    setFilesVersion((v) => v + 1);
  }

  function markFileProcessed(filePath: string) {
    setChatFiles((prev) =>
      prev.map((f) =>
        f.filePath === filePath
          ? { ...f, parsed: true, partiallyParsed: false, processedAt: Date.now() }
          : f
      )
    );
  }

  const sortByCreated = (a: ChatFileInfo, b: ChatFileInfo) => (b.createdAt || 0) - (a.createdAt || 0);
  const activeFiles = chatFiles.filter((f) => !f.ignored);
  const ignoredFiles = chatFiles.filter((f) => f.ignored).sort(sortByCreated);
  const parsedFiles = activeFiles.filter((f) => f.parsed && !f.partiallyParsed).sort(sortByCreated);
  const partiallyParsedFiles = activeFiles.filter((f) => f.partiallyParsed).sort(sortByCreated);
  const unparsedFiles = activeFiles.filter((f) => !f.parsed && !f.partiallyParsed).sort(sortByCreated);

  return (
    <div className="project-detail">
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
          {/* display disclaimer that contrails might take up to 1 minute to appear */}
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
          <h3 className="chat-files-heading">Chat Sessions ({activeFiles.length})</h3>
          {parsedFiles.length > 0 && (
            <div className="chat-files-group">
              <div className="chat-files-group-label">Processed ({parsedFiles.length})</div>
              <div className="chat-files-group-hint">Click any item below to show its content</div>
              <div className="chat-files-scroll">
                {parsedFiles.map((file) => (
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
          {partiallyParsedFiles.length > 0 && (
            <div className="chat-files-group">
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
                  />
                ))}
              </div>
            </div>
          )}
          {unparsedFiles.length > 0 && (
            <div className="chat-files-group">
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

interface ChatFileRowProps {
  file: ChatFileInfo;
  onShowDetails: () => void;
  onProcessed: () => void;
  outputDir: string;
  onIgnore?: () => void;
  onUnignore?: () => void;
}

function ChatFileRow({ file, onShowDetails, onProcessed, outputDir, onIgnore, onUnignore }: ChatFileRowProps) {
  const [processing, setProcessing] = useState(false);

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
      <div className="chat-file-actions">
        {isParsed && file.processedAt > 0 && (
          <span className="chat-file-processed-time">{formatDateTime(file.processedAt)}</span>
        )}
        {onUnignore ? (
          <button className="btn btn-outline btn-sm" onClick={onUnignore} title="Stop ignoring this chat">
            <Eye size={12} /> Unignore
          </button>
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
              ) : isParsed && !isPartiallyParsed ? (
                <><Play size={12} /> Re-process</>
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
