import { Outlet } from "react-router-dom";
import { Topbar } from "./Topbar";

/**
 * Shell — общая обёртка для всех страниц SPA.
 * Рендерит Topbar вверху и дочерний роут через <Outlet />.
 *
 * Настройки больше не открываются как Drawer — у них теперь
 * отдельная страница /app-v1/settings.
 * Кнопка шестерёнки в Topbar делает navigate("/settings").
 */
export function Shell() {
  return (
    <div style={{ minHeight: "100vh", display: "flex", flexDirection: "column" }}>
      <Topbar />
      <main
        style={{
          flex: 1,
          width: "100%",
          maxWidth: 1120,
          margin: "0 auto",
          padding: "var(--sp-6)",
        }}
      >
        <Outlet />
      </main>
    </div>
  );
}
