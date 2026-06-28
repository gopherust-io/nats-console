import { STORAGE_KEYS } from "./constants";

export type AssistantPanelLayout = {
  x: number;
  y: number;
  width: number;
  height: number;
};

const STORAGE_KEY = STORAGE_KEYS.assistantLayout;
const MIN_WIDTH = 320;
const MIN_HEIGHT = 360;
const MARGIN = 16;

export function defaultAssistantPanelLayout(): AssistantPanelLayout {
  const width = Math.min(420, window.innerWidth - MARGIN * 2);
  const height = Math.min(560, window.innerHeight - 120);
  return {
    width,
    height,
    x: window.innerWidth - width - 28,
    y: window.innerHeight - height - 92,
  };
}

export function loadAssistantPanelLayout(): AssistantPanelLayout {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return defaultAssistantPanelLayout();
    const parsed = JSON.parse(raw) as AssistantPanelLayout;
    return clampAssistantPanelLayout(parsed);
  } catch {
    return defaultAssistantPanelLayout();
  }
}

export function saveAssistantPanelLayout(layout: AssistantPanelLayout) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(layout));
}

export function clampAssistantPanelLayout(layout: AssistantPanelLayout): AssistantPanelLayout {
  const maxWidth = window.innerWidth - MARGIN * 2;
  const maxHeight = window.innerHeight - MARGIN * 2;
  const width = Math.min(Math.max(layout.width, MIN_WIDTH), maxWidth);
  const height = Math.min(Math.max(layout.height, MIN_HEIGHT), maxHeight);
  const x = Math.min(Math.max(layout.x, MARGIN), window.innerWidth - width - MARGIN);
  const y = Math.min(Math.max(layout.y, MARGIN), window.innerHeight - height - MARGIN);
  return { x, y, width, height };
}

export function fullscreenAssistantPanelLayout(): AssistantPanelLayout {
  return {
    x: 0,
    y: 0,
    width: window.innerWidth,
    height: window.innerHeight,
  };
}

export const assistantPanelBounds = {
  minWidth: MIN_WIDTH,
  minHeight: MIN_HEIGHT,
  margin: MARGIN,
};
