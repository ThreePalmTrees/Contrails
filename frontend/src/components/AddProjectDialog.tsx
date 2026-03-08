import { useState, useEffect, useMemo } from "react";
import { FolderOpen, Search, ChevronRight, AlertCircle, Info, CheckCircle2, Layers, Check } from "lucide-react";
import { WorkspaceInfo, Project, AgentSource } from "../types";
import {
  BrowseWorkspaceStorages,
  BrowseClaudeCodeProjects,
  BrowseCursorProjects,
  SelectOutputDir,
  GetDefaultOutputDir,
  ValidateOutputDir,
  GetWorkspaceStoragePath,
} from "../../wailsjs/go/main/App";
import { claudecode, cursor } from "../../wailsjs/go/models";
import copilotLogo from "../assets/images/gh-copilot.png";
import claudeLogo from "../assets/images/claude.png";
import cursorLogo from "../assets/images/cursor.png";

/** Show only the last 2 directory segments of a path (e.g. "/workspaceStorage/abc123") */
function shortenPath(fullPath: string): string {
  const parts = fullPath.replace(/\/+$/, "").split("/");
  if (parts.length <= 2) return fullPath;
  return "…/" + parts.slice(-2).join("/");
}

type AgentTab = "vscode" | "claudecode" | "cursor";

interface Props {
  onAdd: (project: Project) => void;
  onCancel: () => void;
  existingWatchDirs: string[];
  existingProject?: Project;
  editTargetTab?: "vscode" | "claudecode" | "cursor" | "output";
}

export function AddProjectDialog({ onAdd, onCancel, existingWatchDirs, existingProject, editTargetTab }: Props) {
  const [step, setStep] = useState<"sources" | "configure">(editTargetTab === "output" ? "configure" : "sources");

  // Agent source selection
  const [activeTab, setActiveTab] = useState<AgentTab | null>(null);

  // VS Code state
  const [workspaces, setWorkspaces] = useState<WorkspaceInfo[]>([]);
  const [vscodeLoading, setVscodeLoading] = useState(false);
  const [vscodeSearch, setVscodeSearch] = useState("");
  const [storagePath, setStoragePath] = useState("");
  const [selectedVSCode, setSelectedVSCode] = useState<WorkspaceInfo | null>(null);

  // Claude Code state
  const [claudeProjects, setClaudeProjects] = useState<claudecode.ScannedProject[]>([]);
  const [claudeLoading, setClaudeLoading] = useState(false);
  const [claudeSearch, setClaudeSearch] = useState("");
  const [selectedClaude, setSelectedClaude] = useState<claudecode.ScannedProject | null>(null);

  // Cursor state
  const [cursorProjects, setCursorProjects] = useState<cursor.ScannedProject[]>([]);
  const [cursorLoading, setCursorLoading] = useState(false);
  const [cursorSearch, setCursorSearch] = useState("");
  const [selectedCursor, setSelectedCursor] = useState<cursor.ScannedProject | null>(null);

  // Configure step
  const [name, setName] = useState("");
  const [defaultName, setDefaultName] = useState("");
  const [outputDir, setOutputDir] = useState("");
  const [workspacePath, setWorkspacePath] = useState("");
  const [outputDirError, setOutputDirError] = useState("");
  const [validatingDir, setValidatingDir] = useState(false);

  useEffect(() => {
    GetWorkspaceStoragePath().then(setStoragePath).catch(() => {});
  }, []);

  useEffect(() => {
    if (existingProject) {
      const vscodeSource = existingProject.sources?.find(s => s.type === "vscode")?.watchDir || (!existingProject.sources ? existingProject.watchDir : undefined);
      const claudeSource = existingProject.sources?.find(s => s.type === "claudecode")?.watchDir;
      const cursorSource = existingProject.sources?.find(s => s.type === "cursor")?.watchDir;

      if (vscodeSource) {
        setSelectedVSCode({
          id: 'existing-vscode',
          name: existingProject.name,
          chatSessionsDir: vscodeSource,
          workspacePath: existingProject.workspacePath
        } as WorkspaceInfo);
      }
      if (claudeSource) {
        setSelectedClaude({
          encodedName: 'existing-claude',
          displayName: existingProject.name,
          transcriptDirectory: claudeSource,
          projectPath: existingProject.workspacePath,
          sessionCount: 0
        } as claudecode.ScannedProject);
      }
      if (cursorSource) {
        setSelectedCursor({
          workspacePath: cursorSource,
          displayName: existingProject.name,
          composerCount: 0,
          lastActivityAt: 0,
        } as cursor.ScannedProject);
      }

      setName(existingProject.name);
      setDefaultName(existingProject.name);
      setWorkspacePath(existingProject.workspacePath || "");
      setOutputDir(existingProject.outputDir);

      if (editTargetTab === "output") {
        // Just configure step
      } else if (editTargetTab) {
        handleTabClick(editTargetTab);
      } else if (vscodeSource && !claudeSource && !cursorSource) {
        handleTabClick("claudecode");
      } else if (claudeSource && !vscodeSource && !cursorSource) {
        handleTabClick("vscode");
      } else if (cursorSource && !vscodeSource && !claudeSource) {
        handleTabClick("vscode");
      }
    }
  }, [existingProject, editTargetTab]);

  const loadVSCodeWorkspaces = async () => {
    if (workspaces.length > 0) return; // already loaded
    setVscodeLoading(true);
    try {
      const results = await BrowseWorkspaceStorages();
      setWorkspaces(results as unknown as WorkspaceInfo[]);
    } catch (err) {
      console.error("Failed to browse workspaces:", err);
    } finally {
      setVscodeLoading(false);
    }
  };

  const loadClaudeProjects = async () => {
    if (claudeProjects.length > 0) return; // already loaded
    setClaudeLoading(true);
    try {
      const results = await BrowseClaudeCodeProjects();
      setClaudeProjects(results || []);
    } catch (err) {
      console.error("Failed to browse Claude Code projects:", err);
    } finally {
      setClaudeLoading(false);
    }
  };

  const loadCursorProjects = async () => {
    if (cursorProjects.length > 0) return; // already loaded
    setCursorLoading(true);
    try {
      const results = await BrowseCursorProjects();
      setCursorProjects(results || []);
    } catch (err) {
      console.error("Failed to browse Cursor projects:", err);
    } finally {
      setCursorLoading(false);
    }
  };

  const handleTabClick = (tab: AgentTab) => {
    setActiveTab(tab);
    if (tab === "vscode") {
      loadVSCodeWorkspaces();
    } else if (tab === "claudecode") {
      loadClaudeProjects();
    } else {
      loadCursorProjects();
    }
  };

  // Filtered VS Code workspaces
  const { available: vscodeAvailable, configured: vscodeConfigured } = useMemo(() => {
    const existingSet = new Set(existingWatchDirs);
    const filtered = workspaces.filter((ws) =>
      ws.name.toLowerCase().includes(vscodeSearch.toLowerCase())
    );
    const avail: WorkspaceInfo[] = [];
    const conf: WorkspaceInfo[] = [];
    for (const ws of filtered) {
      if (existingSet.has(ws.chatSessionsDir)) {
        conf.push(ws);
      } else {
        avail.push(ws);
      }
    }
    return { available: avail, configured: conf };
  }, [workspaces, vscodeSearch, existingWatchDirs]);

  // Filtered Cursor projects
  const { available: cursorAvailable, configured: cursorConfigured } = useMemo(() => {
    const existingSet = new Set(existingWatchDirs);
    const filtered = cursorProjects.filter((p) =>
      p.displayName.toLowerCase().includes(cursorSearch.toLowerCase())
    );
    const avail: cursor.ScannedProject[] = [];
    const conf: cursor.ScannedProject[] = [];
    for (const p of filtered) {
      if (existingSet.has(p.workspacePath)) {
        conf.push(p);
      } else {
        avail.push(p);
      }
    }
    return { available: avail, configured: conf };
  }, [cursorProjects, cursorSearch, existingWatchDirs]);

  // Filtered Claude Code projects
  const { available: claudeAvailable, configured: claudeConfigured } = useMemo(() => {
    const existingSet = new Set(existingWatchDirs);
    const filtered = claudeProjects.filter((p) =>
      p.displayName.toLowerCase().includes(claudeSearch.toLowerCase())
    );
    const avail: claudecode.ScannedProject[] = [];
    const conf: claudecode.ScannedProject[] = [];
    for (const p of filtered) {
      if (existingSet.has(p.transcriptDirectory)) {
        conf.push(p);
      } else {
        avail.push(p);
      }
    }
    return { available: avail, configured: conf };
  }, [claudeProjects, claudeSearch, existingWatchDirs]);

  const selectVSCodeWorkspace = (ws: WorkspaceInfo) => {
    setSelectedVSCode(ws);
    // Pre-fill name from first selection
    if (!selectedClaude) {
      setDefaultName(ws.name);
      setWorkspacePath(ws.workspacePath || "");
    }
  };

  const selectClaudeProject = (project: claudecode.ScannedProject) => {
    setSelectedClaude(project);
    // Pre-fill name from first selection
    if (!selectedVSCode && !selectedCursor) {
      setDefaultName(project.displayName);
      setWorkspacePath(project.projectPath || "");
    }
  };

  const selectCursorProject = (project: cursor.ScannedProject) => {
    setSelectedCursor(project);
    // Pre-fill name from first selection
    if (!selectedVSCode && !selectedClaude) {
      setDefaultName(project.displayName);
      setWorkspacePath(project.workspacePath);
    }
  };

  const hasAnySelection = selectedVSCode !== null || selectedClaude !== null || selectedCursor !== null;

  const hasNewSelection = useMemo(() => {
    if (!existingProject) return false;
    const vscodeSource = existingProject.sources?.find(s => s.type === "vscode")?.watchDir || (!existingProject.sources ? existingProject.watchDir : undefined);
    const claudeSource = existingProject.sources?.find(s => s.type === "claudecode")?.watchDir;
    const cursorSource = existingProject.sources?.find(s => s.type === "cursor")?.watchDir;

    const currVscode = selectedVSCode?.chatSessionsDir;
    const currClaude = selectedClaude?.transcriptDirectory;
    const currCursor = selectedCursor?.workspacePath;

    const currName = name.trim() || defaultName;
    const sourcesChanged =
      (currVscode && currVscode !== vscodeSource) ||
      (currClaude && currClaude !== claudeSource) ||
      (currCursor && currCursor !== cursorSource);
    const configChanged = currName !== existingProject.name || outputDir !== existingProject.outputDir;
    return sourcesChanged || configChanged;
  }, [existingProject, selectedVSCode, selectedClaude, selectedCursor, name, defaultName, outputDir]);

  const goToConfigure = async () => {
    if (!hasAnySelection) return;

    // Derive name and workspace path from selections
    const derivedName = selectedVSCode?.name || selectedClaude?.displayName || selectedCursor?.displayName || "";
    const derivedWorkspace = selectedVSCode?.workspacePath || selectedClaude?.projectPath || selectedCursor?.workspacePath || "";

    setName("");
    setDefaultName(derivedName);
    setWorkspacePath(derivedWorkspace);
    setOutputDirError("");

    if (derivedWorkspace) {
      const defaultOut = await GetDefaultOutputDir(derivedWorkspace);
      setOutputDir(defaultOut);
    }

    setStep("configure");
  };

  const pickOutputDir = async () => {
    const dir = await SelectOutputDir();
    if (dir) {
      setOutputDir(dir);
      setOutputDirError("");
    }
  };

  const submit = async () => {
    const finalName = name.trim() || defaultName;
    if (!finalName || !outputDir || !hasAnySelection) return;

    // Validate output dir
    setValidatingDir(true);
    try {
      const error = await ValidateOutputDir(outputDir);
      if (error) {
        setOutputDirError(error);
        setValidatingDir(false);
        return;
      }
    } catch {
      setOutputDirError("Failed to validate directory");
      setValidatingDir(false);
      return;
    }
    setValidatingDir(false);

    // Build sources array
    const sources: AgentSource[] = [];
    if (selectedVSCode) {
      sources.push({ type: "vscode", watchDir: selectedVSCode.chatSessionsDir });
    }
    if (selectedClaude) {
      sources.push({ type: "claudecode", watchDir: selectedClaude.transcriptDirectory });
    }
    if (selectedCursor) {
      sources.push({ type: "cursor", watchDir: selectedCursor.workspacePath });
    }

    onAdd({
      id: existingProject ? existingProject.id : crypto.randomUUID(),
      name: finalName,
      watchDir: selectedVSCode?.chatSessionsDir || "",
      outputDir,
      active: existingProject ? existingProject.active : true,
      workspacePath: workspacePath || undefined,
      sources,
      lastProcessed: existingProject?.lastProcessed,
      pausedAt: existingProject?.pausedAt,
    });
  };

  const vscodeNoResults = !vscodeLoading && vscodeAvailable.length === 0 && vscodeConfigured.length === 0;
  const claudeNoResults = !claudeLoading && claudeAvailable.length === 0 && claudeConfigured.length === 0;
  const cursorNoResults = !cursorLoading && cursorAvailable.length === 0 && cursorConfigured.length === 0;

  return (
    <div className="dialog-overlay">
      <div className="dialog">
        <div className="dialog-header">
          <h2>
            {existingProject
              ? "Configure Project"
              : step === "configure"
              ? "Add Project (Step 2 / 2)"
              : "Add Project (Step 1 / 2)"}
          </h2>
        </div>

        {step === "sources" && (
          <div className="dialog-body">
            <p className="dialog-description">
              Select your agent source(s):
            </p>

            {/* Agent source toggle buttons */}
            <div className="source-toggles">
              <button
                className={`source-toggle ${activeTab === "vscode" ? "source-toggle-active" : ""} ${selectedVSCode ? "source-toggle-selected" : ""}`}
                onClick={() => handleTabClick("vscode")}
              >
                {selectedVSCode && <Check size={14} className="source-toggle-check" />}
                <img src={copilotLogo} alt="VSCode" style={{ filter: 'invert(1)', height: '38px', width: '38px', objectFit: 'contain' }} className="source-toggle-icon" />
                <span className="source-toggle-label">GitHub Copilot</span>
              </button>

              <button
                className={`source-toggle ${activeTab === "claudecode" ? "source-toggle-active" : ""} ${selectedClaude ? "source-toggle-selected" : ""}`}
                onClick={() => handleTabClick("claudecode")}
              >
                {selectedClaude && <Check size={14} className="source-toggle-check" />}
                <img src={claudeLogo} alt="Claude Code" style={{ height: '38px', width: '50px', objectFit: 'contain' }} className="source-toggle-icon" />
                <span className="source-toggle-label">Claude Code</span>
              </button>

              <button
                className={`source-toggle ${activeTab === "cursor" ? "source-toggle-active" : ""} ${selectedCursor ? "source-toggle-selected" : ""}`}
                onClick={() => handleTabClick("cursor")}
              >
                {selectedCursor && <Check size={14} className="source-toggle-check" />}
                <img src={cursorLogo} alt="Cursor" style={{ height: '34px', width: '34px', objectFit: 'contain' }} className="source-toggle-icon" />
                <span className="source-toggle-label">Cursor</span>
              </button>
            </div>

            {/* Selection summary */}
            {(selectedVSCode || selectedClaude || selectedCursor) && (
              <div className="source-selections">
                {selectedVSCode && (
                  <div className="source-selection-item">
                    <CheckCircle2 size={12} className="configured-check" />
                    <span>GitHub Copilot: {selectedVSCode.name}</span>
                  </div>
                )}
                {selectedClaude && (
                  <div className="source-selection-item">
                    <CheckCircle2 size={12} className="configured-check" />
                    <span>Claude Code: {selectedClaude.displayName}</span>
                  </div>
                )}
                {selectedCursor && (
                  <div className="source-selection-item">
                    <CheckCircle2 size={12} className="configured-check" />
                    <span>Cursor: {selectedCursor.displayName}</span>
                  </div>
                )}
              </div>
            )}

            {/* VSCode workspace list */}
            {activeTab === "vscode" && (
              <>
                <div className="search-box">
                  <Search size={14} />
                  <input
                    placeholder="Search workspaces…"
                    value={vscodeSearch}
                    onChange={(e) => setVscodeSearch(e.target.value)}
                    autoFocus
                  />
                </div>

                <div className="workspace-list">
                  {vscodeLoading && (
                    <div className="workspace-loading">Scanning workspaces…</div>
                  )}

                  {vscodeNoResults && (
                    <div className="workspace-empty">
                      <p>No workspaces found</p>
                      {storagePath && (
                        <p className="workspace-empty-hint">
                          Looking in <span className="mono">{shortenPath(storagePath)}</span>.
                          Make sure VS Code has been used with GitHub Copilot.
                        </p>
                      )}
                    </div>
                  )}

                  {vscodeAvailable.map((ws) => (
                    <button
                      key={ws.id}
                      className={`workspace-item ${selectedVSCode?.id === ws.id ? "workspace-item-active" : ""}`}
                      onClick={() => selectVSCodeWorkspace(ws)}
                    >
                      <div className="workspace-item-info">
                        <span className="workspace-name">
                          {ws.name}
                          {ws.name.endsWith(".code-workspace") && (
                            <span className="badge badge-workspace badge-inline">
                              <Layers size={10} /> Workspace
                            </span>
                          )}
                        </span>
                        <span className="workspace-path" title={ws.chatSessionsDir}>{shortenPath(ws.chatSessionsDir)}</span>
                      </div>
                      {selectedVSCode?.id === ws.id ? (
                        <CheckCircle2 size={14} className="configured-check" />
                      ) : (
                        <ChevronRight size={16} />
                      )}
                    </button>
                  ))}

                  {vscodeConfigured.length > 0 && (
                    <>
                      <div className="workspace-section-label">
                        <CheckCircle2 size={12} />
                        Already Configured
                      </div>
                      {vscodeConfigured.map((ws) => {
                        const isCurrentConfigured = existingProject?.sources?.some(s => s.type === "vscode" && s.watchDir === ws.chatSessionsDir) || existingProject?.watchDir === ws.chatSessionsDir;
                        const isSelected = selectedVSCode?.chatSessionsDir === ws.chatSessionsDir;
                        
                        return (
                        <div
                          key={ws.id}
                          className={`workspace-item ${isCurrentConfigured ? (isSelected ? "workspace-item-active" : "") : "workspace-item-disabled"}`}
                          style={isCurrentConfigured ? { cursor: "pointer", opacity: 1 } : {}}
                          onClick={() => isCurrentConfigured && selectVSCodeWorkspace(ws)}
                        >
                          <div className="workspace-item-info">
                            <span className="workspace-name">
                              {ws.name}
                              {ws.name.endsWith(".code-workspace") && (
                                <span className="badge badge-workspace badge-inline">
                                  <Layers size={10} /> Workspace
                                </span>
                              )}
                              {isCurrentConfigured && <span className="badge badge-inline" style={{ marginLeft: 6 }}>Current</span>}
                            </span>
                            <span className="workspace-path" title={ws.chatSessionsDir}>{shortenPath(ws.chatSessionsDir)}</span>
                          </div>
                          {isSelected ? (
                            <CheckCircle2 size={14} className="configured-check" />
                          ) : isCurrentConfigured ? (
                            <ChevronRight size={16} />
                          ) : (
                            <CheckCircle2 size={14} className="configured-check" />
                          )}
                        </div>
                      )})}
                    </>
                  )}
                </div>
              </>
            )}

            {/* Claude Code project list */}
            {activeTab === "claudecode" && (
              <>
                <div className="search-box">
                  <Search size={14} />
                  <input
                    placeholder="Search projects…"
                    value={claudeSearch}
                    onChange={(e) => setClaudeSearch(e.target.value)}
                    autoFocus
                  />
                </div>

                <div className="workspace-list">
                  {claudeLoading && (
                    <div className="workspace-loading">Scanning Claude Code projects…</div>
                  )}

                  {claudeNoResults && !claudeLoading && (
                    <div className="workspace-empty">
                      <p>No Claude Code projects found</p>
                      <p className="workspace-empty-hint">
                        Looking in <span className="mono">~/.claude/projects/</span>.
                        Make sure Claude Code has been used in at least one project.
                      </p>
                    </div>
                  )}

                  {claudeAvailable.map((project) => (
                    <button
                      key={project.encodedName}
                      className={`workspace-item ${selectedClaude?.encodedName === project.encodedName ? "workspace-item-active" : ""}`}
                      onClick={() => selectClaudeProject(project)}
                    >
                      <div className="workspace-item-info">
                        <span className="workspace-name">{project.displayName}</span>
                        <span className="workspace-path" title={project.projectPath}>
                          {project.sessionCount} session{project.sessionCount !== 1 ? "s" : ""}
                          {" · "}
                          {shortenPath(project.transcriptDirectory)}
                        </span>
                      </div>
                      {selectedClaude?.encodedName === project.encodedName ? (
                        <CheckCircle2 size={14} className="configured-check" />
                      ) : (
                        <ChevronRight size={16} />
                      )}
                    </button>
                  ))}

                  {claudeConfigured.length > 0 && (
                    <>
                      <div className="workspace-section-label">
                        <CheckCircle2 size={12} />
                        Already Configured
                      </div>
                      {claudeConfigured.map((project) => {
                        const isCurrentConfigured = existingProject?.sources?.some(s => s.type === "claudecode" && s.watchDir === project.transcriptDirectory);
                        const isSelected = selectedClaude?.transcriptDirectory === project.transcriptDirectory;

                        return (
                        <div
                          key={project.encodedName}
                          className={`workspace-item ${isCurrentConfigured ? (isSelected ? "workspace-item-active" : "") : "workspace-item-disabled"}`}
                          style={isCurrentConfigured ? { cursor: "pointer", opacity: 1 } : {}}
                          onClick={() => isCurrentConfigured && selectClaudeProject(project)}
                        >
                          <div className="workspace-item-info">
                            <span className="workspace-name">
                              {project.displayName}
                              {isCurrentConfigured && <span className="badge badge-inline" style={{ marginLeft: 6 }}>Current</span>}
                            </span>
                            <span className="workspace-path" title={project.projectPath}>
                              {project.sessionCount} session{project.sessionCount !== 1 ? "s" : ""}
                              {" · "}
                              {shortenPath(project.transcriptDirectory)}
                            </span>
                          </div>
                          {isSelected ? (
                            <CheckCircle2 size={14} className="configured-check" />
                          ) : isCurrentConfigured ? (
                            <ChevronRight size={16} />
                          ) : (
                            <CheckCircle2 size={14} className="configured-check" />
                          )}
                        </div>
                      )})}
                    </>
                  )}
                </div>
              </>
            )}

            {/* Cursor project list */}
            {activeTab === "cursor" && (
              <>
                <div className="search-box">
                  <Search size={14} />
                  <input
                    placeholder="Search projects…"
                    value={cursorSearch}
                    onChange={(e) => setCursorSearch(e.target.value)}
                    autoFocus
                  />
                </div>

                <div className="workspace-list">
                  {cursorLoading && (
                    <div className="workspace-loading">Scanning Cursor projects…</div>
                  )}

                  {cursorNoResults && !cursorLoading && (
                    <div className="workspace-empty">
                      <p>No Cursor projects found</p>
                      <p className="workspace-empty-hint">
                        Make sure Cursor has been used with at least one project.
                      </p>
                    </div>
                  )}

                  {cursorAvailable.map((project) => (
                    <button
                      key={project.workspacePath}
                      className={`workspace-item ${selectedCursor?.workspacePath === project.workspacePath ? "workspace-item-active" : ""}`}
                      onClick={() => selectCursorProject(project)}
                    >
                      <div className="workspace-item-info">
                        <span className="workspace-name">
                          {project.displayName}
                          {project.displayName.endsWith(".code-workspace") && (
                            <span className="badge badge-workspace badge-inline">
                              <Layers size={10} /> Workspace
                            </span>
                          )}
                        </span>
                        <span className="workspace-path" title={project.workspacePath}>
                          {project.composerCount} session{project.composerCount !== 1 ? "s" : ""}
                          {" · "}
                          {shortenPath(project.workspacePath)}
                        </span>
                      </div>
                      {selectedCursor?.workspacePath === project.workspacePath ? (
                        <CheckCircle2 size={14} className="configured-check" />
                      ) : (
                        <ChevronRight size={16} />
                      )}
                    </button>
                  ))}

                  {cursorConfigured.length > 0 && (
                    <>
                      <div className="workspace-section-label">
                        <CheckCircle2 size={12} />
                        Already Configured
                      </div>
                      {cursorConfigured.map((project) => {
                        const isCurrentConfigured = existingProject?.sources?.some(s => s.type === "cursor" && s.watchDir === project.workspacePath);
                        const isSelected = selectedCursor?.workspacePath === project.workspacePath;

                        return (
                          <div
                            key={project.workspacePath}
                            className={`workspace-item ${isCurrentConfigured ? (isSelected ? "workspace-item-active" : "") : "workspace-item-disabled"}`}
                            style={isCurrentConfigured ? { cursor: "pointer", opacity: 1 } : {}}
                            onClick={() => isCurrentConfigured && selectCursorProject(project)}
                          >
                            <div className="workspace-item-info">
                              <span className="workspace-name">
                                {project.displayName}
                                {project.displayName.endsWith(".code-workspace") && (
                                  <span className="badge badge-workspace badge-inline">
                                    <Layers size={10} /> Workspace
                                  </span>
                                )}
                                {isCurrentConfigured && <span className="badge badge-inline" style={{ marginLeft: 6 }}>Current</span>}
                              </span>
                              <span className="workspace-path" title={project.workspacePath}>
                                {project.composerCount} session{project.composerCount !== 1 ? "s" : ""}
                                {" · "}
                                {shortenPath(project.workspacePath)}
                              </span>
                            </div>
                            {isSelected ? (
                              <CheckCircle2 size={14} className="configured-check" />
                            ) : isCurrentConfigured ? (
                              <ChevronRight size={16} />
                            ) : (
                              <CheckCircle2 size={14} className="configured-check" />
                            )}
                          </div>
                        );
                      })}
                    </>
                  )}
                </div>
              </>
            )}

            {/* Prompt to select a source if none active */}
            {activeTab === null && (
              <div className="workspace-empty">
                <p>Click an agent source above to browse available projects.</p>
              </div>
            )}
          </div>
        )}

        {step === "configure" && (
          <div className="dialog-body">
            <div className="form-field">
              <label>Project Name</label>
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={defaultName || "My Project"}
                autoFocus
              />
            </div>

            <div className="form-field">
              <label>Output Directory</label>
              <div className="input-with-btn">
                <input
                  value={outputDir}
                  onChange={(e) => {
                    setOutputDir(e.target.value);
                    setOutputDirError("");
                  }}
                  className={`mono ${outputDirError ? "input-error" : ""}`}
                  placeholder="contrails"
                />
                <button className="btn btn-sm" onClick={pickOutputDir}>
                  <FolderOpen size={13} />
                </button>
              </div>
              {outputDirError ? (
                <span className="form-error">
                  <AlertCircle size={11} /> {outputDirError}
                </span>
              ) : (
                <span className="form-hint">
                  Where parsed chat history will be saved
                </span>
              )}
            </div>

            {/* Source summary */}
            <div className="source-summary">
              {selectedVSCode && (
                <div className="source-summary-item">
                  <img src={copilotLogo} alt="GitHub Copilot" title="GitHub Copilot" style={{ filter: 'invert(1)', height: '30px', width: '30px', objectFit: 'contain' }} />
                  <span className="mono source-summary-path">{shortenPath(selectedVSCode.chatSessionsDir)}</span>
                </div>
              )}
              {selectedClaude && (
                <div className="source-summary-item">
                  <img src={claudeLogo} alt="Claude Code" title="Claude Code" style={{ height: '24px', width: '36px', objectFit: 'contain' }} />
                  <span className="mono source-summary-path">{shortenPath(selectedClaude.transcriptDirectory)}</span>
                </div>
              )}
              {selectedCursor && (
                <div className="source-summary-item">
                  <img src={cursorLogo} alt="Cursor" title="Cursor" style={{ height: '24px', width: '24px', objectFit: 'contain' }} />
                  <span className="mono source-summary-path">{shortenPath(selectedCursor.workspacePath)}</span>
                </div>
              )}
            </div>

            <div className="configure-info">
              <Info size={13} />
              <p>
                Only <strong>new</strong> chats will be automatically processed.
                To process existing sessions, use the &ldquo;Process All Now&rdquo; button after adding.
              </p>
            </div>
          </div>
        )}

        <div className="dialog-footer">
          {!existingProject && step === "configure" && (
            <button
              className="btn btn-ghost"
              onClick={() => {
                setStep("sources");
                setOutputDirError("");
              }}
            >
              Back
            </button>
          )}
          <div className="spacer" />
          <button className="btn btn-ghost" onClick={onCancel}>
            Cancel
          </button>
          {!existingProject && step === "sources" && (
            <button
              className="btn btn-primary"
              onClick={goToConfigure}
              disabled={!hasAnySelection}
            >
              Next &rarr;
            </button>
          )}
          {!existingProject && step === "configure" && (
            <button
              className="btn btn-primary"
              onClick={submit}
              disabled={(!name.trim() && !defaultName) || !outputDir || validatingDir}
            >
              {validatingDir ? "Validating…" : "Add Project"}
            </button>
          )}
          {existingProject && (
            <button
              className="btn btn-primary"
              onClick={submit}
              disabled={!hasNewSelection || validatingDir}
            >
              {validatingDir ? "Validating…" : "Save"}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
