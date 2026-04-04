import { useRef, useState } from "react";
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { Bell, Clapperboard, FileAudio2, LayoutGrid, Search, Settings, Upload, Workflow, Wrench } from "lucide-react";
import { api } from "../lib/api";

const navigation = [
  { to: "/", label: "Dashboard", icon: LayoutGrid, end: true },
  { to: "/jobs", label: "Jobs", icon: Workflow },
  { to: "/media", label: "Media", icon: Clapperboard },
  { to: "/settings", label: "Rules", icon: Wrench },
  { to: "/settings", label: "Settings", icon: Settings }
];

export function AppShell() {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [flash, setFlash] = useState("");
  const [isUploading, setIsUploading] = useState(false);
  const navigate = useNavigate();

  async function handleFile(file: File | null) {
    if (!file) {
      return;
    }

    try {
      setIsUploading(true);
      const result = await api.uploadMedia(file);
      setFlash(result.message);
      navigate(`/media/${result.mediaId}`);
    } catch {
      setFlash("Не удалось загрузить файл через новый UI. Проверьте backend и формат файла.");
    } finally {
      setIsUploading(false);
      window.setTimeout(() => setFlash(""), 3500);
    }
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <div className="brand-mark">
            <FileAudio2 size={18} />
          </div>
          <div>
            <div className="brand-title">Media Pipeline</div>
            <div className="brand-subtitle">Workspace UI</div>
          </div>
        </div>

        <nav className="sidebar-nav">
          {navigation.map(({ to, label, icon: Icon, end }) => (
            <NavLink key={`${to}-${label}`} to={to} end={end} className={({ isActive }) => `nav-item${isActive ? " active" : ""}`}>
              <Icon size={16} />
              <span>{label}</span>
            </NavLink>
          ))}
        </nav>

        <div className="sidebar-footer">
          <div className="sidebar-status-label">System status</div>
          <div className="sidebar-status-card">
            <div className="status-dot success" />
            <div>
              <div className="sidebar-status-title">API / Worker</div>
              <div className="sidebar-status-text">Новый frontend поверх Go backend</div>
            </div>
          </div>
        </div>
      </aside>

      <div className="main-shell">
        <header className="topbar">
          <div className="topbar-search">
            <Search size={14} />
            <input placeholder="Search media, jobs, errors" />
          </div>
          <div className="topbar-actions">
            <button className="icon-button" type="button">
              <Bell size={15} />
            </button>
            <button className="primary-button" type="button" onClick={() => inputRef.current?.click()} disabled={isUploading}>
              <Upload size={14} />
              <span>{isUploading ? "Uploading..." : "Upload"}</span>
            </button>
            <input
              ref={inputRef}
              type="file"
              hidden
              accept=".mp4,.mov,.mkv,.avi,.webm,.mp3,.wav,.m4a,.aac,.flac"
              onChange={(event) => handleFile(event.target.files?.[0] ?? null)}
            />
          </div>
        </header>

        {flash ? <div className="flash-banner">{flash}</div> : null}
        <main className="page-shell">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
