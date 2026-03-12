import { useState, useEffect } from "react";
import { X, FolderOpen, Download } from "lucide-react";
import { DetectIDEs, GetDirectoryOpener, SetDirectoryOpener, OpenDirectoryWith, GetVersion, ApplyAppUpdate } from "../../wailsjs/go/main/App";
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
}

export function DirectoryOpenerDialog({ dirPath, onClose, updateInfo }: DirectoryOpenerDialogProps) {
  const [ides, setIdes] = useState<IDEChoice[]>([]);
  const [selected, setSelected] = useState("open");
  const [customCommand, setCustomCommand] = useState("");
  const [dontAsk, setDontAsk] = useState(false);
  const [loading, setLoading] = useState(true);
  const [version, setVersion] = useState("");
  const [updating, setUpdating] = useState(false);

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
          {isSettingsMode && (
            <p style={{ fontSize: 12, color: "var(--text-tertiary)", marginBottom: 12 }}>
              Choose which application opens output directories.
            </p>
          )}
          {loading ? (
            <p style={{ color: "var(--text-tertiary)" }}>Detecting installed editors...</p>
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
                  <span style={{ fontSize: 11, color: "var(--text-tertiary)", marginLeft: "auto", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
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
        {isSettingsMode && (
          <div className="settings-version-section">
            <div className="settings-version-row">
              <span className="settings-version-label">
                Version {version || "…"}
              </span>
              {updateInfo ? (
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
                  {updating ? "Updating..." : <><Download size={11} /> Update to v{updateInfo.latestVersion}</>}
                </button>
              ) : (
                <span className="settings-version-uptodate">Up to date</span>
              )}
            </div>
          </div>
        )}
        <div className="error-modal-footer" style={{ justifyContent: "space-between" }}>
          {!isSettingsMode ? (
            <label style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 12, color: "var(--text-tertiary)", cursor: "pointer", whiteSpace: "nowrap" }}>
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
