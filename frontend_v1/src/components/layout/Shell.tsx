import { useState } from "react";
import { Outlet } from "react-router-dom";
import { Topbar } from "./Topbar";
import { SettingsDrawer } from "../settings/SettingsDrawer";

export function Shell() {
  const [settingsOpen, setSettingsOpen] = useState(false);

  return (
    <div style={{ minHeight: "100vh", display: "flex", flexDirection: "column" }}>
      <Topbar onSettingsClick={() => setSettingsOpen(true)} />
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
      <SettingsDrawer open={settingsOpen} onClose={() => setSettingsOpen(false)} />
    </div>
  );
}
