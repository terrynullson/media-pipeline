import { createBrowserRouter } from "react-router-dom";
import { Shell } from "../components/layout/Shell";
import { HomePage } from "../components/home/HomePage";
import { MediaDetailPage } from "../components/media/MediaDetailPage";
import { SettingsPage } from "../components/settings/SettingsPage";

export const router = createBrowserRouter(
  [
    {
      path: "/",
      element: <Shell />,
      children: [
        { index: true, element: <HomePage /> },
        { path: "media/:mediaId", element: <MediaDetailPage /> },
        // Настройки — отдельная страница, URL: /app-v1/settings
        { path: "settings", element: <SettingsPage /> },
      ],
    },
  ],
  { basename: "/app-v1" }
);
