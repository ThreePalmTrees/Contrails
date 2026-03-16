import { useState, useEffect } from "react";
import { X, FolderOpen, Download, Sun, Moon } from "lucide-react";
import type { Theme } from "../hooks/useTheme";
import { DetectIDEs, GetDirectoryOpener, SetDirectoryOpener, OpenDirectoryWith, GetVersion, ApplyAppUpdate, CheckForAppUpdate } from "../../wailsjs/go/main/App";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { main } from "../../wailsjs/go/models";

interface IDEChoice {
  name: string;
  command: string;
}

const FINDER_OPTION: IDEChoice = { name: "Finder", command: "open" };
const CUSTOM_SENTINEL = "__custom__";

interface UpdateInfo {
  latestVersion: string;
  downloadURL: string;
  releaseURL: string;
}

interface DirectoryOpenerDialogProps {
  /** Directory path to open (null = settings mode, no directory to open) */
  dirPath: string | null;
  onClose: () => void;
  /** Update info from App (if an update has been detected) */
  updateInfo?: UpdateInfo | null;
  /** Whether analytics collection is enabled */
  analyticsEnabled?: boolean;
  /** Callback to toggle analytics */
  onAnalyticsToggle?: (enabled: boolean) => void;
  /** Current theme */
  theme?: Theme;
  /** Callback to change theme */
  onThemeChange?: (theme: Theme) => void;
}

export function DirectoryOpenerDialog({ dirPath, onClose, updateInfo, analyticsEnabled, onAnalyticsToggle, theme, onThemeChange }: DirectoryOpenerDialogProps) {
  const [ides, setIdes] = useState<IDEChoice[]>([]);
  const [selected, setSelected] = useState("open");
  const [customCommand, setCustomCommand] = useState("");
  const [dontAsk, setDontAsk] = useState(false);
  const [loading, setLoading] = useState(true);
  const [version, setVersion] = useState("");
  const [updating, setUpdating] = useState(false);
  const [checkingUpdate, setCheckingUpdate] = useState(false);
  const [checkedUpdate, setCheckedUpdate] = useState<UpdateInfo | null | undefined>(undefined);

  const isSettingsMode = dirPath === null;
  const isCustom = selected === CUSTOM_SENTINEL;
  const effectiveCommand = isCustom ? customCommand.trim() : selected;

  useEffect(() => {
    GetVersion().then(setVersion).catch(() => {});
  }, []);

  useEffect(() => {
    Promise.all([DetectIDEs(), GetDirectoryOpener()]).then(([detected, saved]) => {
      const options: IDEChoice[] = detected.map((d: main.IDEOption) => ({
        name: d.name,
        command: d.command,
      }));
      setIdes(options);

      if (saved) {
        // Check if saved command matches a known option or Finder
        const knownCommands = [...options.map(o => o.command), "open"];
        if (knownCommands.includes(saved)) {
          setSelected(saved);
        } else {
          // Saved command is a custom one
          setSelected(CUSTOM_SENTINEL);
          setCustomCommand(saved);
        }
      } else if (options.length > 0) {
        setSelected(options[0].command);
      }
      setLoading(false);
    });
  }, []);

  const allOptions = [...ides, FINDER_OPTION];

  async function handleConfirm() {
    if (!effectiveCommand) return;
    if (isSettingsMode || dontAsk) {
      await SetDirectoryOpener(effectiveCommand);
    }
    if (dirPath) {
      await OpenDirectoryWith(dirPath, effectiveCommand);
    }
    onClose();
  }

  return (
    <div className="error-modal-overlay" onClick={onClose}>
      <div className="error-modal" style={{ width: 340, border: "1px solid var(--border-subtle)", overflow: "hidden" }} onClick={(e) => e.stopPropagation()}>
        <div className="error-modal-header">
          <FolderOpen size={16} />
          <h3>{isSettingsMode ? 'Default "Open Directory with"' : "Open With"}</h3>
          <button className="icon-btn icon-btn-sm" onClick={onClose}>
            <X size={14} />
          </button>
        </div>
        <div className="error-modal-body" style={{ overflow: "hidden" }}>
          {isSettingsMode && onThemeChange && (
            <div className="settings-theme-section">
              <div className="settings-telemetry-row">
                <div className="settings-telemetry-info" style={{ flex: 1 }}>
                  <span className="settings-telemetry-label" style={{ cursor: "default" }}>
                    Appearance
                  </span>
                </div>
                <div className="theme-toggle">
                  <button
                    className={`theme-toggle-btn${theme === "dark" ? " active" : ""}`}
                    onClick={() => onThemeChange("dark")}
                    title="Dark mode"
                  >
                    <Moon size={13} />
                  </button>
                  <button
                    className={`theme-toggle-btn${theme === "light" ? " active" : ""}`}
                    onClick={() => onThemeChange("light")}
                    title="Light mode"
                  >
                    <Sun size={13} />
                  </button>
                </div>
              </div>
            </div>
          )}
          {isSettingsMode && (
            <p style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 12 }}>
              Choose which application opens output directories.
            </p>
          )}
          {loading ? (
            <p style={{ color: "var(--text-muted)" }}>Detecting installed editors...</p>
          ) : (
            <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
              {allOptions.map((opt) => (
                <label
                  key={opt.command}
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 8,
                    padding: "5px 8px",
                    borderRadius: "var(--radius-sm)",
                    cursor: "pointer",
                    background: selected === opt.command ? "var(--bg-hover)" : "transparent",
                    minWidth: 0,
                  }}
                >
                  <input
                    type="radio"
                    name="ide"
                    value={opt.command}
                    checked={selected === opt.command}
                    onChange={() => setSelected(opt.command)}
                    style={{ flexShrink: 0 }}
                  />
                  <span style={{ fontSize: 13, whiteSpace: "nowrap" }}>{opt.name}</span>
                  <span style={{ fontSize: 11, color: "var(--text-muted)", marginLeft: "auto", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {opt.command}
                  </span>
                </label>
              ))}

              {/* Custom command option */}
              <label
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 8,
                  padding: "5px 8px",
                  borderRadius: "var(--radius-sm)",
                  cursor: "pointer",
                  background: isCustom ? "var(--bg-hover)" : "transparent",
                }}
              >
                <input
                  type="radio"
                  name="ide"
                  value={CUSTOM_SENTINEL}
                  checked={isCustom}
                  onChange={() => setSelected(CUSTOM_SENTINEL)}
                  style={{ flexShrink: 0 }}
                />
                <span style={{ fontSize: 13 }}>Custom</span>
              </label>
              {isCustom && (
                <input
                  type="text"
                  value={customCommand}
                  onChange={(e) => setCustomCommand(e.target.value)}
                  placeholder="e.g. ide, nano, open -a MyEditor"
                  autoFocus
                  style={{ marginLeft: 30, marginTop: 2, fontSize: 12, width: "calc(100% - 38px)" }}
                  onKeyDown={(e) => { if (e.key === "Enter") handleConfirm(); }}
                />
              )}
            </div>
          )}
        </div>
        {isSettingsMode && onAnalyticsToggle && (
          <div className="settings-telemetry-section">
            <div className="settings-telemetry-row">
              <div className="settings-telemetry-info">
                <label className="settings-telemetry-label">
                  <input
                    type="checkbox"
                    checked={analyticsEnabled ?? true}
                    onChange={(e) => onAnalyticsToggle(e.target.checked)}
                  />
                  Anonymous telemetry
                </label>
                <span className="settings-telemetry-hint">
                  {analyticsEnabled
                    ? "Usage data is collected anonymously to help improve Contrails."
                    : "Telemetry is off. Only basic, non-identifiable signals (app version, OS) are sent to help track adoption."}
                </span>
              </div>
            </div>
          </div>
        )}
        {isSettingsMode && (
          <div className="settings-version-section">
            <div className="settings-version-row">
              <span className="settings-version-label">
                Version {version || "…"}
              </span>
              {(updateInfo || checkedUpdate) ? (
                <button
                  className="btn btn-primary btn-xs"
                  disabled={updating}
                  onClick={() => {
                    const update = (updateInfo || checkedUpdate)!;
                    if (update.downloadURL) {
                      setUpdating(true);
                      ApplyAppUpdate(update.downloadURL).catch(() => setUpdating(false));
                    } else {
                      BrowserOpenURL(update.releaseURL);
                    }
                  }}
                >
                  {updating ? "Updating..." : <><Download size={11} /> Update to v{(updateInfo || checkedUpdate)!.latestVersion}</>}
                </button>
              ) : checkedUpdate === null ? (
                <span className="settings-version-uptodate">Up to date</span>
              ) : (
                <button
                  className="btn btn-secondary btn-xs"
                  disabled={checkingUpdate}
                  onClick={() => {
                    setCheckingUpdate(true);
                    CheckForAppUpdate()
                      .then((info) => {
                        if (info && info.latestVersion) {
                          setCheckedUpdate(info);
                        } else {
                          setCheckedUpdate(null);
                        }
                      })
                      .catch(() => {
                        setCheckedUpdate(null);
                      })
                      .finally(() => setCheckingUpdate(false));
                  }}
                >
                  {checkingUpdate ? "Checking..." : "Check for updates"}
                </button>
              )}
            </div>
          </div>
        )}
        <div className="error-modal-footer" style={{ justifyContent: "space-between" }}>
          {!isSettingsMode ? (
            <label style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 12, color: "var(--text-muted)", cursor: "pointer", whiteSpace: "nowrap" }}>
              <input
                type="checkbox"
                checked={dontAsk}
                onChange={(e) => setDontAsk(e.target.checked)}
              />
              Don't ask again
            </label>
          ) : (
            <div />
          )}
          <div style={{ display: "flex", gap: 8 }}>
            <button className="btn btn-ghost" onClick={onClose}>Cancel</button>
            <button className="btn btn-primary" onClick={handleConfirm} disabled={loading || (isCustom && !customCommand.trim())}>
              {isSettingsMode ? "Save" : "Open"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
