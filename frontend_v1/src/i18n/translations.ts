export type Locale = "ru" | "en";

const translations = {
  // ── App ──
  "app.title": { en: "Media Pipeline", ru: "Медиа Пайплайн" },

  // ── Upload ──
  "upload.dropOrClick": { en: "Drop file or click to select", ru: "Перетащите файл или нажмите для выбора" },
  "upload.dropMultiple": { en: "Drop files or click to select", ru: "Перетащите файлы или нажмите для выбора" },
  "upload.loading": { en: "Loading format info\u2026", ru: "Загрузка информации\u2026" },
  "upload.error.generic": { en: "Upload failed", ru: "Ошибка загрузки" },

  // ── Media list ──
  "filter.all": { en: "All", ru: "Все" },
  "filter.processing": { en: "Processing", ru: "В обработке" },
  "filter.done": { en: "Done", ru: "Готово" },
  "filter.failed": { en: "Failed", ru: "Ошибка" },
  "filter.placeholder": { en: "Filter by filename\u2026", ru: "Поиск по имени файла\u2026" },
  "filter.empty": { en: "No items match the current filter.", ru: "Ничего не найдено по текущему фильтру." },

  // ── Actions ──
  "action.delete": { en: "Delete", ru: "Удалить" },
  "action.deleteMedia": { en: "Delete media", ru: "Удалить медиа" },
  "action.confirmDelete": { en: "Are you sure?", ru: "Вы уверены?" },
  "action.confirmDeleteLong": { en: "Are you sure? This cannot be undone.", ru: "Вы уверены? Это действие нельзя отменить." },
  "action.yesDelete": { en: "Yes, delete", ru: "Да, удалить" },
  "action.confirmBtn": { en: "Confirm delete", ru: "Подтвердить" },
  "action.cancel": { en: "Cancel", ru: "Отмена" },
  "action.select": { en: "Select", ru: "Выбрать" },
  "action.cancelSelect": { en: "Cancel selection", ru: "Отменить выбор" },
  "action.selectAll": { en: "Select all", ru: "Выбрать все" },
  "action.selectedCount": { en: "{n} selected", ru: "Выбрано: {n}" },
  "action.deleteSelected": { en: "Delete selected", ru: "Удалить выбранные" },
  "action.confirmDeleteN": { en: "Delete {n} items? Cannot be undone.", ru: "Удалить {n} файл(ов)? Это нельзя отменить." },
  "action.back": { en: "Back", ru: "Назад" },
  "action.openDetails": { en: "Open details", ru: "Подробнее" },
  "action.processing": { en: "Processing in progress\u2026", ru: "Обработка в процессе\u2026" },

  // ── Player ──
  "player.notAvailable": { en: "Player not available yet.", ru: "Плеер пока недоступен." },
  "player.switchToVideo": { en: "Switch to video", ru: "Переключить на видео" },
  "player.audioFallback": { en: "Use audio fallback", ru: "Аудио версия" },

  // ── Transcript ──
  "transcript.fullText": { en: "Full Text", ru: "Полный текст" },
  "transcript.segments": { en: "Segments", ru: "Сегменты" },
  "transcript.notAvailable": { en: "Transcript not available yet.", ru: "Транскрипт пока недоступен." },
  "transcript.search": { en: "Search transcript\u2026", ru: "Поиск по тексту\u2026" },

  // ── Summary ──
  "summary.notAvailable": { en: "No summary available yet.", ru: "Саммари пока нет." },
  "summary.request": { en: "Request Summary", ru: "Запросить саммари" },

  // ── Triggers ──
  "triggers.title": { en: "Triggers", ru: "Триггеры" },
  "triggers.empty": { en: "No trigger matches found.", ru: "Совпадений с триггерами не найдено." },

  // ── Tech details ──
  "tech.pipelineSteps": { en: "Pipeline Steps", ru: "Этапы обработки" },
  "tech.settings": { en: "Settings Snapshot", ru: "Снимок настроек" },
  "tech.runtime": { en: "Runtime Snapshot", ru: "Снимок среды" },
  "tech.runtimePolicy": { en: "Runtime Policy", ru: "Политика среды" },
  "tech.warnings": { en: "Warnings", ru: "Предупреждения" },

  // ── Settings page ──
  "settings.title": { en: "Settings", ru: "Настройки" },
  "settings.transcription": { en: "Transcription", ru: "Транскрипция" },
  "settings.backend": { en: "Backend", ru: "Бэкенд" },
  "settings.model": { en: "Model", ru: "Модель" },
  "settings.device": { en: "Device", ru: "Устройство" },
  "settings.computeType": { en: "Compute type", ru: "Тип вычислений" },
  "settings.language": { en: "Language", ru: "Язык" },
  "settings.beamSize": { en: "Beam size", ru: "Beam size" },
  "settings.auto": { en: "auto", ru: "авто" },
  "settings.vadFilter": { en: "VAD filter", ru: "VAD фильтр" },
  "settings.save": { en: "Save settings", ru: "Сохранить" },
  "settings.saved": { en: "Saved", ru: "Сохранено" },
  "settings.loading": { en: "Loading settings\u2026", ru: "Загрузка настроек\u2026" },
  "settings.loadError": { en: "Could not load settings. Check server connection.", ru: "Не удалось загрузить настройки. Проверьте соединение с сервером." },

  // ── Trigger rules ──
  "rules.title": { en: "Trigger Rules", ru: "Правила триггеров" },
  "rules.name": { en: "Name", ru: "Название" },
  "rules.category": { en: "Category", ru: "Категория" },
  "rules.pattern": { en: "Pattern", ru: "Паттерн" },
  "rules.contains": { en: "Contains", ru: "Содержит" },
  "rules.exact": { en: "Exact", ru: "Точное" },
  "rules.add": { en: "Add rule", ru: "Добавить" },
  "rules.preview": { en: "Check existing files", ru: "Проверить по файлам" },
  "rules.previewEmpty": { en: "No matches found in existing transcripts.", ru: "Совпадений в существующих транскриптах не найдено." },
  "rules.previewResult": { en: "Found {matches} match(es) in {files} file(s).", ru: "Найдено {matches} совпад. в {files} файл(ах)." },

  // ── Topbar ──
  "topbar.settings": { en: "Settings", ru: "Настройки" },
} as const;

export type TranslationKey = keyof typeof translations;

export function getTranslation(key: TranslationKey, locale: Locale): string {
  return translations[key][locale];
}

export default translations;
