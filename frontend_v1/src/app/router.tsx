import { createBrowserRouter } from "react-router-dom";
import { Shell } from "../components/layout/Shell";
import { HomePage } from "../components/home/HomePage";
import { MediaDetailPage } from "../components/media/MediaDetailPage";

export const router = createBrowserRouter(
  [
    {
      path: "/",
      element: <Shell />,
      children: [
        { index: true, element: <HomePage /> },
        { path: "media/:mediaId", element: <MediaDetailPage /> },
      ],
    },
  ],
  { basename: "/app-v1" }
);
