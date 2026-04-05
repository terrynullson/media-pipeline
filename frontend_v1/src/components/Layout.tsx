import { useEffect, useMemo, useRef, useState } from "react";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import {
  Bell,
  ChevronRight,
  Clock3,
  FolderKanban,
  LayoutGrid,
  Search,
  Settings,
  UploadCloud,
  Waves
} from "lucide-react";
import { api } from "../api/client";

const navItems = [
  { to: "/", label: "Главная", icon: LayoutGrid, end: true },
  { to: "/history", label: "История", icon: FolderKanban },
  { to: "/settings", label: "Настройки", icon: Settings }
];

export function Layout() {
  const [flash, setFlash] = useState("");
  const [uploading, setUploading] = useState(false);
  const [preferredTheme, setPreferredTheme] = useState("new");
  const inputRef = useRef<HTMLInputElement | null>(null);
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    api.uiConfig().then((config) => setPreferredTheme(config.uiTheme)).catch(() => undefined);
  }, []);

  const pageTitle = useMemo(() => {
    const current = navItems.find((item) => (item.end ? location.pathname === item.to : location.pathname.startsWith(item.to)));
    return current?.label ?? "Детали";
  }, [location.pathname]);

  async function handleUpload(file: File | null) {
    if (!file) {
      return;
    }

    try {
      setUploading(true);
      const result = await api.uploadMedia(file);
      setFlash(result.message);
      navigate(`/media/${result.mediaId}`);
    } catch {
      setFlash("Не удалось загрузить файл. Проверьте формат и доступность backend API.");
    } finally {
      setUploading(false);
      window.setTimeout(() => setFlash(""), 4000);
    }
  }

  return (
    <div className="v1-shell">
      <aside className="v1-sidebar">
        <div className="brand-block">
          <div className="brand-mark">
            <Waves size={18} />
          </div>
          <div>
            <div className="brand-title">Media Pipeline</div>
            <div className="brand-subtitle">PORTAL V1</div>
          </div>
        </div>

        <nav className="sidebar-nav">
          {navItems.map(({ to, label, icon: Icon, end }) => (
            <NavLink key={to} to={to} end={end} className={({ isActive }) => `sidebar-link${isActive ? " active" : ""}`}>
              <Icon size={16} />
              <span>{label}</span>
              <ChevronRight size={14} className="sidebar-arrow" />
            </NavLink>
          ))}
        </nav>

        <div className="sidebar-promo">
          <div className="promo-label">Рабочий режим</div>
          <div className="promo-title">Новый shell поверх Go API</div>
          <div className="promo-copy">
            Старый UI сохранён. Предпочтение сейчас: <strong>{preferredTheme === "new" ? "новый" : "старый"}</strong>.
          </div>
        </div>
      </aside>

      <div className="v1-main">
        <header className="v1-topbar">
          <div>
            <div className="topbar-kicker">Workspace</div>
            <h1>{pageTitle}</h1>
          </div>

          <div className="topbar-right">
            <label className="topbar-search">
              <Search size={14} />
              <input readOnly value="Поиск появится на странице История" />
            </label>

            <button type="button" className="topbar-icon" aria-label="Notifications">
              <Bell size={15} />
            </button>

            <button type="button" className="topbar-icon status-icon" aria-label="Single worker mode">
              <Clock3 size={14} />
            </button>

            <button type="button" className="primary-action" onClick={() => inputRef.current?.click()} disabled={uploading}>
              <UploadCloud size={15} />
              <span>{uploading ? "Загрузка..." : "Загрузить файл"}</span>
            </button>
            <input
              ref={inputRef}
              type="file"
              hidden
              accept=".mp4,.mov,.mkv,.avi,.webm,.mp3,.wav,.m4a,.aac,.flac"
              onChange={(event) => handleUpload(event.target.files?.[0] ?? null)}
            />
          </div>
        </header>

        {flash ? <div className="flash-banner">{flash}</div> : null}
        <main className="v1-content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
