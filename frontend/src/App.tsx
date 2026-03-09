import { useState, useEffect, useMemo } from "react";
import { ExternalLink, X, AlertCircle, Download, Settings } from "lucide-react";
import { ContrailsIcon } from "./components/ContrailsIcon";
import { ProjectList } from "./components/ProjectList";
import { ProjectDetail } from "./components/ProjectDetail";
import { AddProjectDialog } from "./components/AddProjectDialog";
import { OnboardingTour } from "./components/OnboardingTour";
import { useProjects } from "./hooks/useProjects";
import { DirectoryOpenerDialog } from "./components/DirectoryOpenerDialog";
import { Project } from "./types";
import { BrowserOpenURL, EventsOn } from "../wailsjs/runtime/runtime";
import { GetAnalyticsEnabled, SetAnalyticsEnabled, ApplyAppUpdate } from "../wailsjs/go/main/App";
import "./App.css";

function App() {
  const {
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
  } = useProjects();

  const [showAddDialog, setShowAddDialog] = useState(false);
  const [editProject, setEditProject] = useState<Project | null>(null);
  const [editTab, setEditTab] = useState<"vscode" | "claudecode" | "cursor" | "output" | undefined>();
  const [analyticsEnabled, setAnalyticsEnabled] = useState(true);
  const [updateInfo, setUpdateInfo] = useState<{
    latestVersion: string;
    downloadURL: string;
    releaseURL: string;
  } | null>(null);
  const [updating, setUpdating] = useState(false);
  const [showOpenerSettings, setShowOpenerSettings] = useState(false);

  useEffect(() => {
    GetAnalyticsEnabled().then(setAnalyticsEnabled).catch(() => {});
    const cancel = EventsOn("update:available", (info: any) => {
      if (info && info.latestVersion) {
        setUpdateInfo(info);
      }
    });
    return cancel;
  }, []);

  // Collect existing watch dirs for duplicate prevention
  const existingWatchDirs = useMemo(
    () => projects.flatMap((p) => {
      const dirs: string[] = [];
      if (p.watchDir) dirs.push(p.watchDir);
      if (p.sources) {
        for (const s of p.sources) {
          if (s.watchDir) dirs.push(s.watchDir);
        }
      }
      return dirs;
    }),
    [projects]
  );

  const handleRename = async (project: Project, name: string) => {
    await updateProject({ ...project, name });
  };

  const handleToggle = async (project: Project) => {
    await updateProject({ ...project, active: !project.active });
  };

  const handleProcess = async (project: Project) => {
    try {
      await processNow(project);
    } catch (err) {
      console.error("Processing failed:", err);
    }
  };

  if (loading) {
    return (
      <div className="app-loading">
        <ContrailsIcon style={{ width: '34px' }} />
      </div>
    );
  }

  return (
    <div className="app">
      <div className="titlebar" />
      <aside className="sidebar">
        <div className="sidebar-brand">
          <ContrailsIcon style={{ width: '34px' }} />
          <span style={{ marginLeft: '-14px', marginTop: '-1px' }}>Contrails</span>
        </div>
        <ProjectList
          projects={projects}
          selectedId={selectedId}
          onSelect={setSelectedId}
          onAdd={() => setShowAddDialog(true)}
          onRename={handleRename}
          onToggle={handleToggle}
          onRemove={removeProject}
          onProcess={handleProcess}
          processing={processing}
          processingProgress={processingProgress}
          badgeCounts={badgeCounts}
        />
        {updateInfo && (
          <div className="update-banner">
            <div className="update-banner-text">
              v{updateInfo.latestVersion} available
            </div>
            <button
              className="btn btn-primary btn-xs"
              disabled={updating}
              onClick={() => {
                if (updateInfo.downloadURL) {
                  setUpdating(true);
                  ApplyAppUpdate(updateInfo.downloadURL).catch(() => setUpdating(false));
                } else {
                  BrowserOpenURL(updateInfo.releaseURL);
                }
              }}
            >
              {updating ? "Updating..." : <><Download size={11} /> Update</>}
            </button>
            <button
              className="icon-btn icon-btn-sm"
              onClick={() => setUpdateInfo(null)}
              title="Dismiss"
            >
              <X size={12} />
            </button>
          </div>
        )}
        <footer className="sidebar-footer" style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <button
            className="footer-link"
            onClick={() => BrowserOpenURL("https://github.com/ThreePalmTrees/Contrails")}
          >
            Documentation <ExternalLink size={11} />
          </button>
          <button
            className="icon-btn icon-btn-sm"
            onClick={() => setShowOpenerSettings(true)}
            title="Settings"
            style={{ opacity: 0.5 }}
          >
            <Settings size={13} />
          </button>
        </footer>
      </aside>

      <main className="main-content">
        {selectedProject ? (
          <ProjectDetail
            key={selectedProject.id}
            project={selectedProject}
            onToggle={handleToggle}
            onProcess={handleProcess}
            onEdit={(project, tab) => {
              setEditProject(project);
              setEditTab(tab);
            }}
            onUpdateProject={updateProject}
            processing={processing}
            processingProgress={processingProgress}
          />
        ) : (
          <div className="empty-main">
            <div style={{ opacity: 0.2 }}>
              <ContrailsIcon style={{ width: '88px' }} />
              <h2>Contrails</h2>
            </div>
            <p>Select a project or add a new one to get started.</p>
            <button
              className="btn btn-primary"
              onClick={() => setShowAddDialog(true)}
              style={{fontSize: '14px', padding: '8px 28px'}}
            >
              Add Project
            </button>
            <div className="telemetry-toggle">
              <span
                className={`analytics-dot ${analyticsEnabled ? "analytics-on" : "analytics-off"}`}
                role="button"
                tabIndex={0}
                onClick={() => {
                  const next = !analyticsEnabled;
                  SetAnalyticsEnabled(next).then(() => setAnalyticsEnabled(next)).catch(() => {});
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    const next = !analyticsEnabled;
                    SetAnalyticsEnabled(next).then(() => setAnalyticsEnabled(next)).catch(() => {});
                  }
                }}
                title={analyticsEnabled ? "Anonymous telemetry is enabled. Click to disable." : "Anonymous telemetry is disabled. Click to enable."}
              />
              <span style={{ cursor: 'default'}}>Telemetry {analyticsEnabled ? "on" : "off"}</span>
            </div>
          </div>
        )}
      </main>

      {showAddDialog && (
        <AddProjectDialog
          existingWatchDirs={existingWatchDirs}
          onAdd={async (project) => {
            await addProject(project);
            setShowAddDialog(false);
          }}
          onCancel={() => setShowAddDialog(false)}
        />
      )}

      {editProject && (
        <AddProjectDialog
          existingProject={editProject}
          editTargetTab={editTab}
          existingWatchDirs={existingWatchDirs}
          onAdd={async (project) => {
            await updateProject(project);
            setEditProject(null);
          }}
          onCancel={() => {
            setEditProject(null);
            setEditTab(undefined);
          }}
        />
      )}

      {/* Error modal */}
      {appError && (
        <div className="error-modal-overlay">
          <div className="error-modal">
            <div className="error-modal-header">
              <AlertCircle size={16} className="error-icon" />
              <h3>Error</h3>
              <button className="icon-btn icon-btn-sm" onClick={dismissError}>
                <X size={14} />
              </button>
            </div>
            <div className="error-modal-body">
              {appError.projectName && (
                <p className="error-project">Project: {appError.projectName}</p>
              )}
              <p className="error-message">{appError.message}</p>
            </div>
            <div className="error-modal-footer">
              <button className="btn btn-ghost" onClick={dismissError}>
                Dismiss
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Onboarding tour */}
      {!onboardingComplete && projects.length === 0 && (
        <OnboardingTour onComplete={completeOnboarding} />
      )}

      {showOpenerSettings && (
        <DirectoryOpenerDialog
          dirPath={null}
          onClose={() => setShowOpenerSettings(false)}
        />
      )}
    </div>
  );
}

export default App;
