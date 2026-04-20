export type Locale = "ru" | "en";

const translations = {
  "app.title": { en: "Media Pipeline", ru: "Медиа Пайплайн" },

  "upload.dropOrClick": {
    en: "Drop file or click to select",
    ru: "Перетащите файл или нажмите для выбора",
  },
  "upload.dropMultiple": {
    en: "Drop files or click to select",
    ru: "Перетащите файлы или нажмите для выбора",
  },
  "upload.loading": { en: "Loading format info...", ru: "Загрузка информации..." },
  "upload.error.generic": { en: "Upload failed", ru: "Ошибка загрузки" },

  "filter.all": { en: "All", ru: "Все" },
  "filter.processing": { en: "Processing", ru: "В обработке" },
  "filter.done": { en: "Done", ru: "Готово" },
  "filter.failed": { en: "Failed", ru: "Ошибка" },
  "filter.placeholder": {
    en: "Filter by filename...",
    ru: "Поиск по имени файла...",
  },
  "filter.empty": {
    en: "No items match the current filter.",
    ru: "По текущему фильтру ничего не найдено.",
  },

  "action.delete": { en: "Delete", ru: "Удалить" },
  "action.deleteMedia": { en: "Delete media", ru: "Удалить медиа" },
  "action.confirmDelete": { en: "Are you sure?", ru: "Вы уверены?" },
  "action.confirmDeleteLong": {
    en: "Are you sure? This cannot be undone.",
    ru: "Вы уверены? Это действие нельзя отменить.",
  },
  "action.yesDelete": { en: "Yes, delete", ru: "Да, удалить" },
  "action.confirmBtn": { en: "Confirm delete", ru: "Подтвердить" },
  "action.cancel": { en: "Cancel", ru: "Отмена" },
  "action.select": { en: "Select", ru: "Выбрать" },
  "action.cancelSelect": { en: "Cancel selection", ru: "Снять выбор" },
  "action.selectAll": { en: "Select all", ru: "Выбрать все" },
  "action.selectedCount": { en: "{n} selected", ru: "Выбрано: {n}" },
  "action.deleteSelected": { en: "Delete selected", ru: "Удалить выбранные" },
  "action.confirmDeleteN": {
    en: "Delete {n} items? Cannot be undone.",
    ru: "Удалить {n} файл(ов)? Это действие нельзя отменить.",
  },
  "action.back": { en: "Back", ru: "Назад" },
  "action.openDetails": { en: "Open details", ru: "Подробнее" },
  "action.processing": {
    en: "Processing in progress...",
    ru: "Обработка выполняется...",
  },

  "player.notAvailable": {
    en: "Player not available yet.",
    ru: "Плеер пока недоступен.",
  },
  "player.switchToVideo": { en: "Switch to video", ru: "Переключить на видео" },
  "player.audioFallback": { en: "Use audio fallback", ru: "Аудио-версия" },

  "transcript.fullText": { en: "Full Text", ru: "Полный текст" },
  "transcript.segments": { en: "Segments", ru: "Сегменты" },
  "transcript.notAvailable": {
    en: "Transcript not available yet.",
    ru: "Транскрипт пока недоступен.",
  },
  "transcript.search": {
    en: "Search transcript...",
    ru: "Поиск по тексту...",
  },

  "summary.notAvailable": {
    en: "No summary available yet.",
    ru: "Саммари пока недоступно.",
  },
  "summary.request": { en: "Request Summary", ru: "Запросить саммари" },

  "triggers.title": { en: "Triggers", ru: "Триггеры" },
  "triggers.empty": {
    en: "No trigger matches found.",
    ru: "Совпадений по триггерам не найдено.",
  },

  "tech.pipelineSteps": { en: "Pipeline Steps", ru: "Этапы обработки" },
  "tech.settings": { en: "Settings Snapshot", ru: "Снимок настроек" },
  "tech.runtime": { en: "Runtime Snapshot", ru: "Снимок среды" },
  "tech.runtimePolicy": { en: "Runtime Policy", ru: "Политика времени" },
  "tech.warnings": { en: "Warnings", ru: "Предупреждения" },

  "settings.title": { en: "Settings", ru: "Настройки" },
  "settings.transcription": { en: "Transcription", ru: "Транскрипция" },
  "settings.backend": { en: "Backend", ru: "Бэкенд" },
  "settings.model": { en: "Model", ru: "Модель" },
  "settings.device": { en: "Device", ru: "Устройство" },
  "settings.computeType": { en: "Compute type", ru: "Тип вычислений" },
  "settings.language": { en: "Language", ru: "Язык" },
  "settings.beamSize": { en: "Beam size", ru: "Beam size" },
  "settings.auto": { en: "auto", ru: "авто" },
  "settings.vadFilter": { en: "VAD filter", ru: "VAD-фильтр" },
  "settings.save": { en: "Save settings", ru: "Сохранить" },
  "settings.saved": { en: "Saved", ru: "Сохранено" },
  "settings.saveError": { en: "Failed to save", ru: "Не удалось сохранить" },
  "settings.loading": { en: "Loading settings...", ru: "Загрузка настроек..." },
  "settings.loadError": {
    en: "Could not load settings. Check server connection.",
    ru: "Не удалось загрузить настройки. Проверьте соединение с сервером.",
  },

  "settings.transcriptionDesc": {
    en: "Configure the transcription engine: model, device, and quality parameters.",
    ru: "Параметры движка транскрипции: модель, устройство и качество.",
  },
  "settings.rulesDesc": {
    en: "Highlight important moments in transcripts using keyword or exact-phrase rules.",
    ru: "Подсветка важных моментов по ключевым словам и точным фразам.",
  },
  "settings.analytics": { en: "Analytics", ru: "Аналитика" },
  "settings.analyticsDesc": {
    en: "Configure stop-words for the word-frequency report on the Analytics page.",
    ru: "Настройка стоп-слов для отчёта «Топ слов» на странице Аналитика.",
  },

  "settings.groupEngine": { en: "Engine", ru: "Движок" },
  "settings.groupHardware": { en: "Hardware", ru: "Железо" },
  "settings.groupQuality": {
    en: "Language & quality",
    ru: "Язык и качество",
  },

  "settings.backendHint": {
    en: "Transcription engine library.",
    ru: "Библиотека движка транскрипции.",
  },
  "settings.modelHint": {
    en: "Larger models are more accurate but slower and heavier.",
    ru: "Более крупные модели точнее, но медленнее и тяжелее.",
  },
  "settings.deviceHint": {
    en: "CPU works everywhere; CUDA needs an NVIDIA GPU.",
    ru: "CPU работает везде; CUDA требует NVIDIA GPU.",
  },
  "settings.computeTypeHintCPU": {
    en: "int8 is faster; float32 is more precise.",
    ru: "int8 быстрее; float32 точнее.",
  },
  "settings.computeTypeHintGPU": {
    en: "float16 is fastest; float32 is more precise.",
    ru: "float16 быстрее; float32 точнее.",
  },
  "settings.languageHint": {
    en: "ISO code (e.g. en, ru) or leave blank to auto-detect.",
    ru: "ISO-код, например en или ru, либо пусто для автоопределения.",
  },
  "settings.beamSizeHint": {
    en: "Higher values improve accuracy but slow transcription (1-10).",
    ru: "Чем больше значение, тем точнее, но медленнее (1-10).",
  },
  "settings.vadHint": {
    en: "Silences and non-speech segments are filtered out before transcription.",
    ru: "Тишина и неречевые участки будут удалены перед транскрипцией.",
  },

  "rules.title": { en: "Trigger Rules", ru: "Правила триггеров" },
  "rules.name": { en: "Name", ru: "Название" },
  "rules.category": { en: "Category", ru: "Категория" },
  "rules.pattern": { en: "Pattern", ru: "Паттерн" },
  "rules.contains": { en: "Contains", ru: "Содержит" },
  "rules.exact": { en: "Exact", ru: "Точное" },
  "rules.add": { en: "Add rule", ru: "Добавить" },
  "rules.preview": { en: "Check existing files", ru: "Проверить по файлам" },
  "rules.previewEmpty": {
    en: "No matches found in existing transcripts.",
    ru: "Совпадений в существующих транскриптах не найдено.",
  },
  "rules.previewResult": {
    en: "Found {matches} match(es) in {files} file(s).",
    ru: "Найдено {matches} совпадений в {files} файл(ах).",
  },

  "topbar.settings": { en: "Settings", ru: "Настройки" },
} as const;

export type TranslationKey = keyof typeof translations;

export function getTranslation(key: TranslationKey, locale: Locale): string {
  return translations[key][locale];
}

export default translations;
