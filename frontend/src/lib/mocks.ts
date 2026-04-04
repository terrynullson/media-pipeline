import type { DashboardResponse, JobItem, MediaListItem, SettingsResponse, TriggerRule } from "./types";

export const mockDashboard: DashboardResponse = {
  overview: [
    { label: "Media", value: "3", tone: "neutral", help: "Файлы в витрине" },
    { label: "Jobs in progress", value: "1", tone: "running", help: "Обработка ещё идёт" },
    { label: "Failed media", value: "1", tone: "error", help: "Нужна проверка" },
    { label: "Transcript ready", value: "1", tone: "success", help: "Текст уже готов" }
  ],
  queueBreakdown: [
    { label: "Pending", value: "2", tone: "neutral", help: "Ждут worker" },
    { label: "Running", value: "1", tone: "running", help: "Выполняются" },
    { label: "Done", value: "4", tone: "success", help: "Завершены" },
    { label: "Preview ready", value: "1", tone: "success", help: "Есть preview" }
  ],
  workerNotice: {
    label: "Worker",
    tone: "running",
    text: "Mock fallback включён только для layout-среза. Реальный API подключается автоматически, когда доступен."
  },
  recentMedia: [],
  recentJobs: [],
  latestErrors: []
};

export const mockMedia: MediaListItem[] = [];
export const mockJobs: JobItem[] = [];

export const mockSettings: SettingsResponse = {
  profile: {
    backend: "faster-whisper",
    modelName: "tiny",
    device: "cpu",
    computeType: "int8",
    language: "",
    beamSize: 5,
    vadEnabled: true
  },
  warnings: [],
  options: {
    backends: ["faster-whisper"],
    models: ["tiny", "base", "small"],
    devices: ["cpu", "cuda"],
    cpu: ["int8", "float32"],
    cuda: ["float16", "int8_float16", "int8_float32"]
  }
};

export const mockRules: TriggerRule[] = [];
