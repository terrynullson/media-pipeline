export interface PipelineStep {
  label: string;
  statusLabel: string;
  tone: string;
  isCurrent?: boolean;
  isFailed?: boolean;
  timingText: string;
  startedAtLabel?: string;
  finishedAtLabel?: string;
  durationLabel?: string;
  etaLabel?: string;
  progressLabel?: string;
  progressPercent?: number;
  progressVisible?: boolean;
}

export interface MediaListItem {
  id: number;
  name: string;
  extension: string;
  sizeHuman: string;
  createdAtUtc: string;
  completedAtUtc?: string;
  status: string;
  statusLabel: string;
  statusTone: string;
  stageLabel: string;
  stageValue: number;
  stageTotal: number;
  stagePercent: number;
  currentStage: string;
  currentTimingText: string;
  currentEtaLabel?: string;
  errorSummary?: string;
  hasTranscript: boolean;
  isAudioOnly: boolean;
  previewReady: boolean;
  transcriptUrl: string;
  deleteUrl: string;
  pipelineSteps: PipelineStep[];
  previewStatusLabel?: string;
  previewStatusTone?: string;
}

export interface JobItem {
  id: number;
  mediaId: number;
  mediaName: string;
  type: string;
  typeLabel: string;
  status: string;
  statusLabel: string;
  statusTone: string;
  createdAtUtc: string;
  startedAtUtc?: string;
  finishedAtUtc?: string;
  durationLabel?: string;
  attempts: number;
  progressPercent?: number;
  progressLabel?: string;
  errorMessage?: string;
  detailUrl: string;
  mediaStatusLabel: string;
  mediaStageLabel: string;
  mediaCurrentStage: string;
}

export interface DashboardMetric {
  label: string;
  value: string;
  tone: string;
  help: string;
}

export interface DashboardResponse {
  overview: DashboardMetric[];
  queueBreakdown: DashboardMetric[];
}

export interface TriggerItem {
  category: string;
  ruleName: string;
  matchedPhrase: string;
  timestamp: string;
  segmentText: string;
  contextText: string;
  hasScreenshot: boolean;
  screenshotURL?: string;
  placeholder?: string;
}

export interface TranscriptSegment {
  index: number;
  startLabel: string;
  endLabel: string;
  text: string;
  confidence?: string;
  hasConfidence?: boolean;
}

export interface MediaDetailResponse {
  media: {
    id: number;
    name: string;
    extension: string;
    mimeType: string;
    sizeHuman: string;
    createdAtUtc: string;
    completedAtUtc?: string;
    isAudioOnly: boolean;
  };
  pipeline: {
    statusLabel: string;
    statusTone: string;
    stageLabel: string;
    stageValue: number;
    stageTotal: number;
    currentStage: string;
    failedStage?: string;
    errorSummary?: string;
    errorLocation?: string;
    steps: PipelineStep[];
  };
  player: {
    hasMediaPlayer: boolean;
    isAudioOnly: boolean;
    hasVideoPlayer: boolean;
    videoSourceURL?: string;
    videoSourceType?: string;
    hasAudioPlayer: boolean;
    audioPlayerURL?: string;
    audioPlayerType?: string;
    hasSecondaryAudioFallback: boolean;
    audioFallbackURL?: string;
    previewStatusLabel?: string;
    previewStatusTone?: string;
    previewNotice?: string;
    playerFallbackText?: string;
  };
  transcript: {
    hasTranscript: boolean;
    fullTextParagraphs: string[];
    segments: TranscriptSegment[];
  };
  triggers: {
    statusLabel: string;
    statusTone: string;
    notice: string;
    noticeTone: string;
    items: TriggerItem[];
  };
  summary: {
    hasSummary: boolean;
    text: string;
    highlights: string[];
    provider: string;
    updatedAtUtc: string;
    statusLabel: string;
    statusTone: string;
    notice: string;
    noticeTone: string;
    showAction: boolean;
    actionLabel: string;
    requestSummaryUrl: string;
  };
  settingsSnapshot: {
    settings: { label: string; value: string }[];
    settingsWarnings: string[];
    settingsUnavailable: boolean;
    runtimePolicy?: {
      visible: boolean;
      title: string;
      tone: string;
      summary: string;
      durationLabel?: string;
      durationClass?: string;
      effectiveTimeout?: string;
      warnings?: string[];
    };
    runtimeSnapshot: { label: string; value: string }[];
  };
  actions: {
    deleteUrl: string;
    legacyTranscript: string;
  };
}

export interface TriggerRule {
  id: number;
  name: string;
  category: string;
  pattern: string;
  matchMode: string;
  enabled: boolean;
  toggleLabel: string;
  enabledLabel: string;
  enabledTone: string;
}

export interface SettingsResponse {
  profile: {
    backend: string;
    modelName: string;
    device: string;
    computeType: string;
    language: string;
    beamSize: number;
    vadEnabled: boolean;
    uiTheme: string;
  };
  runtime: {
    autoUploadMinAgeSec: number;
    previewTimeoutSec: number;
    maxUploadSizeMB: number;
  };
  warnings: string[];
  ui: {
    theme: string;
    legacyAppURL: string;
    modernAppURL: string;
    preferredAppURL: string;
    workspaceURL: string;
  };
  options: {
    backends: string[];
    models: string[];
    devices: string[];
    cpu: string[];
    cuda: string[];
    themes: string[];
  };
}

export interface RuntimeSettingsResponse {
  runtime: {
    autoUploadMinAgeSec: number;
    previewTimeoutSec: number;
    maxUploadSizeMB: number;
  };
}

export interface UIConfigResponse {
  maxUploadBytes: number;
  maxUploadHuman: string;
  acceptedFormats: string[];
  uiTheme: string;
  legacyAppURL: string;
  modernAppURL: string;
  preferredAppURL: string;
  workspaceURL: string;
}

export interface UploadProgress {
  loaded: number;
  total: number;
  percent: number;
}

export interface WorkerStatusResponse {
  workerHeartbeatAge: number;
  likelyAlive: boolean;
  currentJob: {
    id: number;
    mediaId: number;
    type: string;
    startedAt: string;
    progressPercent?: number;
    progressLabel?: string;
  } | null;
  queue: {
    pending: number;
    byType: Record<string, number>;
  };
}
