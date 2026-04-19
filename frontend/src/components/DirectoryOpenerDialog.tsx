import { useState, useEffect, useRef } from "react";
import { X, FolderOpen, Download, Sun, Moon, ChevronDown } from "lucide-react";
import type { Theme } from "../hooks/useTheme";
import { DetectIDEs, GetFileManagerInfo, GetDirectoryOpener, SetDirectoryOpener, OpenDirectoryWith, GetVersion, ApplyAppUpdate, CheckForAppUpdate } from "../../wailsjs/go/main/App";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { main } from "../../wailsjs/go/models";

interface IDEChoice {
  name: string;
  command: string;
}

const CUSTOM_SENTINEL = "__custom__";

interface UpdateInfo {
  latestVersion: string;
  downloadURL: string;
  releaseURL: string;
}

interface ContrailFilters {
  saveThinking: boolean;
  saveToolCalls: boolean;
  saveSubagentContent: boolean;
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
  /** Whether debug file saving is enabled */
  saveDebugFiles?: boolean;
  /** Callback to toggle debug file saving */
  onSaveDebugFilesToggle?: (enabled: boolean) => void;
  /** Contrail content filter toggles (default true when omitted) */
  saveThinking?: boolean;
  saveToolCalls?: boolean;
  saveSubagentContent?: boolean;
  /** Persist all three filter toggles at once */
  onContrailFiltersChange?: (filters: ContrailFilters) => void;
  /** Current theme */
  theme?: Theme;
  /** Callback to change theme */
  onThemeChange?: (theme: Theme) => void;
}

export function DirectoryOpenerDialog({ dirPath, onClose, updateInfo, analyticsEnabled, onAnalyticsToggle, saveDebugFiles, onSaveDebugFilesToggle, saveThinking, saveToolCalls, saveSubagentContent, onContrailFiltersChange, theme, onThemeChange }: DirectoryOpenerDialogProps) {
  const [ides, setIdes] = useState<IDEChoice[]>([]);
  const [selected, setSelected] = useState("open");
  const [customCommand, setCustomCommand] = useState("");
  const [dontAsk, setDontAsk] = useState(false);
  const [fileManager, setFileManager] = useState<IDEChoice>({ name: "Finder", command: "open" });
  const [loading, setLoading] = useState(true);
  const [version, setVersion] = useState("");
  const [updating, setUpdating] = useState(false);
  const [checkingUpdate, setCheckingUpdate] = useState(false);
  const [checkedUpdate, setCheckedUpdate] = useState<UpdateInfo | null | undefined>(undefined);

  // Local draft state for settings that should only apply on Save
  const [draftTheme, setDraftTheme] = useState(theme);
  const [draftAnalytics, setDraftAnalytics] = useState(analyticsEnabled ?? true);
  const [draftSaveDebugFiles, setDraftSaveDebugFiles] = useState(saveDebugFiles ?? false);
  const [draftSaveThinking, setDraftSaveThinking] = useState(saveThinking ?? true);
  const [draftSaveToolCalls, setDraftSaveToolCalls] = useState(saveToolCalls ?? true);
  const [draftSaveSubagentContent, setDraftSaveSubagentContent] = useState(saveSubagentContent ?? true);

  const [dropdownOpen, setDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const isSettingsMode = dirPath === null;
  const isCustom = selected === CUSTOM_SENTINEL;
  const effectiveCommand = isCustom ? customCommand.trim() : selected;

  // Close dropdown on outside click
  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setDropdownOpen(false);
      }
    }
    if (dropdownOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      return () => document.removeEventListener("mousedown", handleClickOutside);
    }
  }, [dropdownOpen]);

  useEffect(() => {
    GetVersion().then(setVersion).catch(() => {});
  }, []);

  useEffect(() => {
    Promise.all([DetectIDEs(), GetDirectoryOpener(), GetFileManagerInfo()]).then(([detected, saved, fm]) => {
      const options: IDEChoice[] = detected.map((d: main.IDEOption) => ({
        name: d.name,
        command: d.command,
      }));
      setIdes(options);
      if (fm && fm.name) {
        setFileManager({ name: fm.name, command: fm.command });
      }

      if (saved) {
        // Check if saved command matches a known option or the file manager
        const knownCommands = [...options.map(o => o.command), fm?.command || "open"];
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

  const allOptions = [...ides, fileManager];

  function handleCancel() {
    // Revert theme preview if it was changed
    if (draftTheme !== theme && theme) {
      document.documentElement.setAttribute("data-theme", theme);
    }
    onClose();
  }

  async function handleConfirm() {
    if (!effectiveCommand) return;
    if (isSettingsMode || dontAsk) {
      await SetDirectoryOpener(effectiveCommand);
    }
    if (dirPath) {
      await OpenDirectoryWith(dirPath, effectiveCommand);
    }
    // Apply theme and analytics changes only on Save
    if (isSettingsMode) {
      if (onThemeChange && draftTheme !== theme) {
        onThemeChange(draftTheme!);
      }
      if (onAnalyticsToggle && draftAnalytics !== (analyticsEnabled ?? true)) {
        onAnalyticsToggle(draftAnalytics);
      }
      if (onSaveDebugFilesToggle && draftSaveDebugFiles !== (saveDebugFiles ?? false)) {
        onSaveDebugFilesToggle(draftSaveDebugFiles);
      }
      if (onContrailFiltersChange) {
        const changed =
          draftSaveThinking !== (saveThinking ?? true) ||
          draftSaveToolCalls !== (saveToolCalls ?? true) ||
          draftSaveSubagentContent !== (saveSubagentContent ?? true);
        if (changed) {
          onContrailFiltersChange({
            saveThinking: draftSaveThinking,
            saveToolCalls: draftSaveToolCalls,
            saveSubagentContent: draftSaveSubagentContent,
          });
        }
      }
    }
    onClose();
  }

  return (
    <div className="error-modal-overlay" onClick={handleCancel}>
      <div className="error-modal" style={{ width: 340, border: "1px solid var(--border-subtle)" }} onClick={(e) => e.stopPropagation()}>
        <div className="error-modal-header">
          <FolderOpen size={16} />
          <h3>Settings</h3>
          <button className="icon-btn icon-btn-sm" onClick={handleCancel}>
            <X size={14} />
          </button>
        </div>
        <div className="error-modal-body" style={{ overflow: "visible" }}>
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
                    className={`theme-toggle-btn${draftTheme === "dark" ? " active" : ""}`}
                    onClick={() => {
                      setDraftTheme("dark");
                      document.documentElement.setAttribute("data-theme", "dark");
                    }}
                    title="Dark mode"
                  >
                    <Moon size={13} />
                  </button>
                  <button
                    className={`theme-toggle-btn${draftTheme === "light" ? " active" : ""}`}
                    onClick={() => {
                      setDraftTheme("light");
                      document.documentElement.setAttribute("data-theme", "light");
                    }}
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
            <div className="settings-opener-group">
              <div className="settings-opener-trigger-wrap" ref={dropdownRef}>
                <button
                  type="button"
                  className={`settings-opener-trigger${isCustom ? " has-custom" : ""}`}
                  onClick={() => setDropdownOpen(!dropdownOpen)}
                >
                  <span className="settings-opener-trigger-label">
                    {isCustom
                      ? "Custom command"
                      : (() => { const match = allOptions.find(o => o.command === selected); return match ? match.name : selected; })()
                    }
                  </span>
                  <ChevronDown size={14} className={`settings-opener-chevron${dropdownOpen ? " open" : ""}`} />
                </button>
                {dropdownOpen && (
                  <div className="settings-opener-dropdown">
                    {allOptions.map((opt) => (
                      <button
                        type="button"
                        key={opt.command}
                        className={`settings-opener-option${selected === opt.command ? " active" : ""}`}
                        onClick={() => { setSelected(opt.command); setDropdownOpen(false); }}
                      >
                        <span>{opt.name}</span>
                        <span className="settings-opener-option-cmd">{opt.command}</span>
                      </button>
                    ))}
                    <button
                      type="button"
                      className={`settings-opener-option${isCustom ? " active" : ""}`}
                      onClick={() => { setSelected(CUSTOM_SENTINEL); setDropdownOpen(false); }}
                    >
                      Custom command
                    </button>
                  </div>
                )}
              </div>
              {isCustom && (
                <input
                  type="text"
                  className="settings-opener-input"
                  value={customCommand}
                  onChange={(e) => setCustomCommand(e.target.value)}
                  placeholder="e.g. ide, nano, open -a MyEditor"
                  autoFocus
                  onKeyDown={(e) => { if (e.key === "Enter") handleConfirm(); }}
                />
              )}
            </div>
          )}
        </div>
        {isSettingsMode && onContrailFiltersChange && (
          <div className="settings-telemetry-section settings-contrail-filters">
            <div className="settings-contrail-filters-header">
              Contrail content
            </div>
            <div className="settings-telemetry-row">
              <div className="settings-telemetry-info">
                <span className="settings-telemetry-label">Thinking</span>
                <span className="settings-telemetry-hint">
                  Include the thinking blocks in saved contrails.
                </span>
              </div>
              <label className="settings-toggle">
                <input
                  type="checkbox"
                  checked={draftSaveThinking}
                  onChange={(e) => setDraftSaveThinking(e.target.checked)}
                />
                <span className="settings-toggle-slider" />
              </label>
            </div>
            <div className="settings-telemetry-row" style={{ marginTop: 8 }}>
              <div className="settings-telemetry-info">
                <span className="settings-telemetry-label">Tool calls</span>
                <span className="settings-telemetry-hint">
                  Include tool calls in saved contrails.
                </span>
              </div>
              <label className="settings-toggle">
                <input
                  type="checkbox"
                  checked={draftSaveToolCalls}
                  onChange={(e) => setDraftSaveToolCalls(e.target.checked)}
                />
                <span className="settings-toggle-slider" />
              </label>
            </div>
            <div className="settings-telemetry-row" style={{ marginTop: 8 }}>
              <div className="settings-telemetry-info">
                <span className="settings-telemetry-label">Sub-agent content</span>
                <span className="settings-telemetry-hint">
                  {draftSaveToolCalls
                    ? "Include activity of sub-agents in saved contrails."
                    : "Disabled because sub-agents are tool calls - enable “Tool calls” to configure this."}
                </span>
              </div>
              <label className={`settings-toggle${draftSaveToolCalls ? "" : " is-disabled"}`}>
                <input
                  type="checkbox"
                  checked={draftSaveToolCalls && draftSaveSubagentContent}
                  disabled={!draftSaveToolCalls}
                  onChange={(e) => setDraftSaveSubagentContent(e.target.checked)}
                />
                <span className="settings-toggle-slider" />
              </label>
            </div>
            {onSaveDebugFilesToggle && (
              <div className="settings-telemetry-row" style={{ marginTop: 8 }}>
                <div className="settings-telemetry-info">
                  <span className="settings-telemetry-label">Save Claude debug files</span>
                  <span className="settings-telemetry-hint">
                    Save raw transcript and signal files alongside contrails for debugging.
                  </span>
                </div>
                <label className="settings-toggle">
                  <input
                    type="checkbox"
                    checked={draftSaveDebugFiles}
                    onChange={(e) => setDraftSaveDebugFiles(e.target.checked)}
                  />
                  <span className="settings-toggle-slider" />
                </label>
              </div>
            )}
          </div>
        )}
        {isSettingsMode && onAnalyticsToggle && (
          <div className="settings-telemetry-section">
            <div className="settings-telemetry-row">
              <div className="settings-telemetry-info">
                <span className="settings-telemetry-label">Anonymous telemetry</span>
                <span className="settings-telemetry-hint">
                  {draftAnalytics
                    ? "Usage data is collected anonymously to help improve Contrails."
                    : "Telemetry is off. Only basic, non-identifiable signals (app version, OS) are sent to help track adoption."}
                </span>
              </div>
              <label className="settings-toggle">
                <input
                  type="checkbox"
                  checked={draftAnalytics}
                  onChange={(e) => setDraftAnalytics(e.target.checked)}
                />
                <span className="settings-toggle-slider" />
              </label>
            </div>
          </div>
        )}
        {isSettingsMode && (
          <div className="settings-version-section">
            <div className="settings-version-row">
              <span className="settings-version-label">
                Version: {version || "..."}
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
                  className="btn btn-primary btn-s"
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
            <button className="btn btn-ghost" onClick={handleCancel}>Cancel</button>
            <button className="btn btn-primary" onClick={handleConfirm} disabled={loading || (isCustom && !customCommand.trim())}>
              {isSettingsMode ? "Save" : "Open"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
