import { useState, useMemo } from "react";
import { ExternalLink, X, AlertCircle } from "lucide-react";
import { ContrailsIcon } from "./components/ContrailsIcon";
import { ProjectList } from "./components/ProjectList";
import { ProjectDetail } from "./components/ProjectDetail";
import { AddProjectDialog } from "./components/AddProjectDialog";
import { OnboardingTour } from "./components/OnboardingTour";
import { useProjects } from "./hooks/useProjects";
import { Project } from "./types";
import { BrowserOpenURL } from "../wailsjs/runtime/runtime";
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
        <footer className="sidebar-footer">
          <button
            className="footer-link"
            onClick={() => BrowserOpenURL("https://github.com/ThreePalmTrees/Contrails")}
          >
            Documentation <ExternalLink size={11} />
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
            <div>
              <ContrailsIcon style={{ width: '48px' }} />
              <h2>Contrails</h2>
            </div>
            <p>Select a project or add a new one to get started.</p>
            <button
              className="btn btn-primary"
              onClick={() => setShowAddDialog(true)}
            >
              Add Project
            </button>
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
    </div>
  );
}

export default App;
