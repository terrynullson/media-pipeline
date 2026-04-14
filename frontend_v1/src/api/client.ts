import type {
  DashboardResponse,
  JobItem,
  MediaDetailResponse,
  MediaListItem,
  SettingsResponse,
  TriggerRule,
  UIConfigResponse,
  UploadProgress,
  WorkerStatusResponse
} from "../models/types";

const defaultUIConfig: UIConfigResponse = {
  maxUploadBytes: 0,
  maxUploadHuman: "unknown",
  acceptedFormats: [".mp4", ".mov", ".mkv", ".avi", ".webm", ".mp3", ".wav", ".m4a", ".aac", ".flac"],
  uiTheme: "new",
  legacyAppURL: "/app",
  modernAppURL: "/app-v1",
  preferredAppURL: "/app-v1",
  workspaceURL: "/workspace"
};

function normalizeSettingsResponse(raw: SettingsResponse): SettingsResponse {
  return {
    profile: {
      ...raw.profile,
      uiTheme: raw.profile?.uiTheme ?? raw.ui?.theme ?? "new"
    },
    warnings: Array.isArray(raw.warnings) ? raw.warnings : [],
    ui: {
      ...defaultUIConfig,
      theme: raw.ui?.theme ?? raw.profile?.uiTheme ?? "new",
      legacyAppURL: raw.ui?.legacyAppURL ?? defaultUIConfig.legacyAppURL,
      modernAppURL: raw.ui?.modernAppURL ?? defaultUIConfig.modernAppURL,
      preferredAppURL: raw.ui?.preferredAppURL ?? defaultUIConfig.preferredAppURL,
      workspaceURL: raw.ui?.workspaceURL ?? defaultUIConfig.workspaceURL
    },
    options: {
      backends: raw.options?.backends ?? ["faster-whisper"],
      models: raw.options?.models ?? ["tiny", "base", "small"],
      devices: raw.options?.devices ?? ["cpu", "cuda"],
      cpu: raw.options?.cpu ?? ["int8", "float32"],
      cuda: raw.options?.cuda ?? ["float16", "int8_float16", "int8_float32"],
      themes: raw.options?.themes ?? ["old", "new"]
    }
  };
}

async function requestJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    ...init,
    headers: {
      Accept: "application/json",
      ...(init?.headers ?? {})
    }
  });

  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }

  return (await response.json()) as T;
}

export const api = {
  dashboard: () => requestJSON<DashboardResponse>("/api/dashboard"),
  media: async () => (await requestJSON<{ items: MediaListItem[] }>("/api/media")).items,
  jobs: async () => (await requestJSON<{ items: JobItem[] }>("/api/jobs")).items,
  mediaDetail: (mediaId: string) => requestJSON<MediaDetailResponse>(`/api/media/${mediaId}`),
  settings: async () => normalizeSettingsResponse(await requestJSON<SettingsResponse>("/api/settings/transcription")),
  updateSettings: (payload: SettingsResponse["profile"]) =>
    requestJSON<{ status: string; preferredAppURL?: string }>("/api/settings/transcription", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload)
    }),
  uiConfig: async () => {
    try {
      return await requestJSON<UIConfigResponse>("/api/ui-config");
    } catch {
      return defaultUIConfig;
    }
  },
  updateUITheme: (uiTheme: string) =>
    requestJSON<{ status: string; uiTheme: string; preferredAppURL: string }>("/api/ui-preference", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ uiTheme })
    }),
  rules: async () => (await requestJSON<{ items: TriggerRule[] }>("/api/trigger-rules")).items,
  createRule: (payload: { name: string; category: string; pattern: string; matchMode: string }) =>
    requestJSON<{ status: string }>("/api/trigger-rules", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload)
    }),
  toggleRule: (ruleId: number, enabled: boolean) =>
    requestJSON<{ status: string }>(`/api/trigger-rules/${ruleId}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled })
    }),
  deleteRule: (ruleId: number) =>
    requestJSON<{ status: string }>(`/api/trigger-rules/${ruleId}`, {
      method: "DELETE"
    }),
  requestSummary: (url: string) =>
    requestJSON<{ status: string }>(url, {
      method: "POST"
    }),
  uploadMedia: async (file: File) => {
    const form = new FormData();
    form.append("media", file);
    return requestJSON<{ mediaId: number; status: string; message: string }>("/upload", {
      method: "POST",
      body: form
    });
  },
  uploadWithProgress: (
    file: File,
    onProgress: (p: UploadProgress) => void,
    onCancelReady?: (cancel: () => void) => void
  ): Promise<{ mediaId: number; status: string; message: string }> =>
    new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      const form = new FormData();
      form.append("media", file);

      if (onCancelReady) {
        onCancelReady(() => {
          xhr.abort();
          reject(new Error("cancelled"));
        });
      }

      xhr.upload.addEventListener("progress", (e) => {
        if (e.lengthComputable) {
          onProgress({ loaded: e.loaded, total: e.total, percent: Math.round((e.loaded / e.total) * 100) });
        }
      });

      xhr.addEventListener("load", () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          try {
            resolve(JSON.parse(xhr.responseText));
          } catch {
            reject(new Error("Invalid response"));
          }
        } else {
          let msg = `Ошибка загрузки (HTTP ${xhr.status})`;
          try {
            const body = JSON.parse(xhr.responseText);
            if (body.message) msg = body.message;
          } catch { /* ignore parse errors */ }
          reject(new Error(msg));
        }
      });

      xhr.addEventListener("error", () => reject(new Error("Upload failed")));
      xhr.addEventListener("abort", () => reject(new Error("cancelled")));
      xhr.open("POST", "/upload");
      xhr.setRequestHeader("Accept", "application/json");
      xhr.send(form);
    }),
  deleteMedia: (mediaId: number) =>
    requestJSON<{ status: string }>(`/media/${mediaId}/delete`, { method: "POST" }),
  workerStatus: () => requestJSON<WorkerStatusResponse>("/api/worker/status"),
  previewTriggerRule: (payload: { pattern: string; matchMode: string }) =>
    requestJSON<{ totalMatches: number; mediaMatches: { mediaId: number; matchCount: number; firstMatchAt: number }[]; limited: boolean }>("/api/trigger-rules/preview", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    }),
  bulkDeleteMedia: (ids: number[]) =>
    requestJSON<{ deleted: number[]; failed: { id: number; error: string }[] }>("/api/media/bulk-delete", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ids }),
    })
};
