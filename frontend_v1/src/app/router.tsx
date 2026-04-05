import { createBrowserRouter } from "react-router-dom";
import { Layout } from "../components/Layout";
import { Home } from "../components/home/Home";
import { History } from "../components/history/History";
import { MediaDetail } from "../components/media/MediaDetail";
import { Settings } from "../components/settings/Settings";

export const router = createBrowserRouter(
  [
    {
      path: "/",
      element: <Layout />,
      children: [
        { index: true, element: <Home /> },
        { path: "history", element: <History /> },
        { path: "media/:mediaId", element: <MediaDetail /> },
        { path: "settings", element: <Settings /> }
      ]
    }
  ],
  { basename: "/app-v1" }
);
