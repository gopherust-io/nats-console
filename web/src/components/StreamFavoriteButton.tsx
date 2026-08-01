import { useCallback, useState, useSyncExternalStore, type MouseEvent } from "react";
import { useTranslation } from "react-i18next";
import {
  isFavoriteStream,
  readFavoriteStreams,
  toggleFavoriteStream,
  type FavoriteStream,
} from "../lib/favoriteStreams";

let favoritesSnapshot = readFavoriteStreams();
const listeners = new Set<() => void>();

function emitFavorites() {
  favoritesSnapshot = readFavoriteStreams();
  for (const listener of listeners) listener();
}

function subscribeFavorites(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function getFavoritesSnapshot() {
  return favoritesSnapshot;
}

export function useFavoriteStreams(): FavoriteStream[] {
  return useSyncExternalStore(subscribeFavorites, getFavoritesSnapshot, () => []);
}

export function useIsFavoriteStream(clusterId: string | null | undefined, streamName: string): boolean {
  const favorites = useFavoriteStreams();
  if (!clusterId || !streamName) return false;
  return isFavoriteStream(clusterId, streamName, favorites);
}

type StreamFavoriteButtonProps = {
  clusterId: string;
  streamName: string;
  className?: string;
};

export default function StreamFavoriteButton({ clusterId, streamName, className = "nc-icon-btn" }: StreamFavoriteButtonProps) {
  const { t } = useTranslation();
  const favorite = useIsFavoriteStream(clusterId, streamName);
  const [, setTick] = useState(0);

  const onToggle = useCallback(
    (event: MouseEvent) => {
      event.preventDefault();
      event.stopPropagation();
      toggleFavoriteStream(clusterId, streamName);
      emitFavorites();
      setTick((n) => n + 1);
    },
    [clusterId, streamName],
  );

  return (
    <button
      type="button"
      className={`${className}${favorite ? " nc-favorite-btn--active" : ""}`}
      aria-pressed={favorite}
      aria-label={favorite ? t("streams.unfavoriteStream") : t("streams.favoriteStream")}
      title={favorite ? t("streams.unfavoriteStream") : t("streams.favoriteStream")}
      onClick={onToggle}
    >
      <svg width="16" height="16" viewBox="0 0 24 24" fill={favorite ? "currentColor" : "none"} stroke="currentColor" strokeWidth="1.75" aria-hidden>
        <path
          d="M7 3.75h10a1.25 1.25 0 0 1 1.25 1.25v15.1l-6.25-3.75L5.75 20.1V5A1.25 1.25 0 0 1 7 3.75z"
          strokeLinejoin="round"
        />
      </svg>
    </button>
  );
}

export { emitFavorites };
