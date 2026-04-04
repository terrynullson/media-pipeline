import React from "react";
import ReactDOM from "react-dom/client";
import { RouterProvider, createBrowserRouter } from "react-router-dom";
import { AppShell } from "./shared/AppShell";
import { DashboardPage } from "./pages/DashboardPage";
import { JobsPage } from "./pages/JobsPage";
import { MediaPage } from "./pages/MediaPage";
import { MediaDetailPage } from "./pages/MediaDetailPage";
import { SettingsPage } from "./pages/SettingsPage";
import "./styles.css";

const router = createBrowserRouter(
  [
    {
      path: "/",
      element: <AppShell />,
      children: [
        { index: true, element: <DashboardPage /> },
        { path: "jobs", element: <JobsPage /> },
        { path: "media", element: <MediaPage /> },
        { path: "media/:mediaId", element: <MediaDetailPage /> },
        { path: "settings", element: <SettingsPage /> }
      ]
    }
  ],
  { basename: "/app" }
);

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>
);
