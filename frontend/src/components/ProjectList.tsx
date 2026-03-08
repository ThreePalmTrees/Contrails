import { useState } from "react";
import {
  FolderPlus,
  MoreHorizontal,
  Eye,
  EyeOff,
  Pencil,
  Trash2,
  Play,
  Loader2,
  AlertTriangle,
  Layers,
} from "lucide-react";
import { Project, ProcessingProgress } from "../types";
import copilotLogo from "../assets/images/gh-copilot.png";
import claudeLogo from "../assets/images/claude.png";
import cursorLogo from "../assets/images/cursor.png";

interface Props {
  projects: Project[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  onAdd: () => void;
  onRename: (project: Project, name: string) => void;
  onToggle: (project: Project) => void;
  onRemove: (id: string) => void;
  onProcess: (project: Project) => void;
  processing: string | null;
  processingProgress: ProcessingProgress | null;
  badgeCounts: Record<string, number>;
}

export function ProjectList({
  projects,
  selectedId,
  onSelect,
  onAdd,
  onRename,
  onToggle,
  onRemove,
  onProcess,
  processing,
  processingProgress,
  badgeCounts,
}: Props) {
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState("");
  const [menuId, setMenuId] = useState<string | null>(null);
  const [confirmRemoveId, setConfirmRemoveId] = useState<string | null>(null);

  const startEditing = (project: Project) => {
    setEditingId(project.id);
    setEditName(project.name);
    setMenuId(null);
  };

  const submitRename = (project: Project) => {
    const trimmed = editName.trim();
    // If empty, keep the original name
    onRename(project, trimmed || project.name);
    setEditingId(null);
  };

  const handleRemoveClick = (e: React.MouseEvent, projectId: string) => {
    e.stopPropagation();
    setMenuId(null);
    setConfirmRemoveId(projectId);
  };

  const confirmRemove = () => {
    if (confirmRemoveId) {
      onRemove(confirmRemoveId);
      setConfirmRemoveId(null);
    }
  };

  return (
    <div className="project-list">
      <div className="project-list-header">
        <span className="project-list-title">Projects</span>
        <button
          className="icon-btn"
          onClick={onAdd}
          title="Add project"
        >
          <FolderPlus size={16} />
        </button>
      </div>

      <div className="project-list-items">
        {projects.length === 0 && (
          <div className="empty-state">
            <p>No projects yet</p>
            <button className="btn btn-ghost" onClick={onAdd}>
              <FolderPlus size={14} />
              Add your first project
            </button>
          </div>
        )}

        {projects.map((project) => {
          const badge = badgeCounts[project.id];
          const isItemProcessing = processing === project.id;
          const progress = processingProgress?.projectId === project.id ? processingProgress : null;
          const hasVSCode = project.sources?.some((s) => s.type === "vscode") ?? (project.watchDir !== "");
          const hasClaude = project.sources?.some((s) => s.type === "claudecode") ?? false;
          const hasCursor = project.sources?.some((s) => s.type === "cursor") ?? false;

          return (
            <div
              key={project.id}
              className={`project-item ${selectedId === project.id ? "selected" : ""} ${!project.active ? "inactive" : ""} ${menuId === project.id ? "menu-open" : ""}`}
              onClick={() => onSelect(project.id)}
            >
              <div className="project-item-content">
                <div className={`status-dot ${project.active ? "active" : ""}`} />
                {project.workspacePath?.endsWith(".code-workspace") && (
                  <Layers size={12} className="workspace-icon" />
                )}

                {editingId === project.id ? (
                  <input
                    className="rename-input"
                    value={editName}
                    onChange={(e) => setEditName(e.target.value)}
                    onBlur={() => submitRename(project)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") submitRename(project);
                      if (e.key === "Escape") setEditingId(null);
                    }}
                    autoFocus
                    onClick={(e) => e.stopPropagation()}
                  />
                ) : (
                  <span className="project-name">{project.name}</span>
                )}
              </div>

              <div className="project-item-right">
                {badge && badge > 0 && selectedId !== project.id && (
                  <span
                    className="badge-count"
                    title={`${badge} new contrail${badge > 1 ? "s" : ""} since you last checked`}
                  >
                    {badge > 99 ? "99+" : badge}
                  </span>
                )}

                <div className="project-item-actions">
                  <button
                    className="icon-btn icon-btn-sm"
                    onClick={(e) => {
                      e.stopPropagation();
                      setMenuId(menuId === project.id ? null : project.id);
                    }}
                  >
                    <MoreHorizontal size={14} />
                  </button>

                  {menuId === project.id && (
                    <div
                      className="context-menu"
                      onMouseLeave={() => setMenuId(null)}
                    >
                      <button
                        className="context-menu-item"
                        onClick={(e) => {
                          e.stopPropagation();
                          startEditing(project);
                        }}
                      >
                        <Pencil size={13} /> Rename
                      </button>
                      <button
                        className="context-menu-item"
                        onClick={(e) => {
                          e.stopPropagation();
                          onToggle(project);
                          setMenuId(null);
                        }}
                      >
                        {project.active ? (
                          <>
                            <EyeOff size={13} /> Pause watching
                          </>
                        ) : (
                          <>
                            <Eye size={13} /> Resume watching
                          </>
                        )}
                      </button>
                      <button
                        className="context-menu-item"
                        onClick={(e) => {
                          e.stopPropagation();
                          onProcess(project);
                          setMenuId(null);
                        }}
                        disabled={isItemProcessing}
                      >
                        {isItemProcessing ? (
                          <>
                            <Loader2 size={13} className="spin" />
                            {progress
                              ? `${progress.current}/${progress.total}…`
                              : "Processing…"}
                          </>
                        ) : (
                          <>
                            <Play size={13} /> Process now
                          </>
                        )}
                      </button>
                      <div className="context-menu-divider" />
                      <button
                        className="context-menu-item danger"
                        onClick={(e) => handleRemoveClick(e, project.id)}
                      >
                        <Trash2 size={13} /> Remove
                      </button>
                    </div>
                  )}
                </div>
              </div>
              <div style={{ display: 'flex', gap: '4px' }}>
                {hasVSCode && <img style={{ filter: 'invert(1)', height: '20px', width: '20px', objectFit: 'contain' }} src={copilotLogo} alt="VSCode" />}
                {hasClaude && <img style={{ height: '18px', width: '28px', objectFit: 'contain' }} src={claudeLogo} alt="ClaudeCode" />}
                {hasCursor && <img style={{ height: '18px', width: '18px', objectFit: 'contain' }} src={cursorLogo} alt="Cursor" />}
              </div>
            </div>
          );
        })}
      </div>

      {/* Remove confirmation dialog */}
      {confirmRemoveId && (
        <div className="dialog-overlay" onClick={() => setConfirmRemoveId(null)}>
          <div className="dialog confirm-dialog" onClick={(e) => e.stopPropagation()}>
            <div className="dialog-header">
              <h2>
                <AlertTriangle size={16} className="warning-icon" />
                Remove Project
              </h2>
            </div>
            <div className="dialog-body">
              <p>
                Are you sure you want to remove this project? The watch configuration will be lost and you&apos;ll need to re-add it.
              </p>
              <p className="confirm-info">
                Contrails in the target directory will NOT be removed. If you wish to remove them, you can do it manually.
              </p>
            </div>
            <div className="dialog-footer">
              <div className="spacer" />
              <button className="btn btn-ghost" onClick={() => setConfirmRemoveId(null)}>
                Cancel
              </button>
              <button className="btn btn-danger" onClick={confirmRemove}>
                Remove
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
