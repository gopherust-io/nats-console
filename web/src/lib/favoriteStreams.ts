import { STORAGE_KEYS } from "./constants";

export type FavoriteStream = {
  clusterId: string;
  streamName: string;
};

function favoriteKey(fav: FavoriteStream): string {
  return `${fav.clusterId}\0${fav.streamName}`;
}

export function readFavoriteStreams(): FavoriteStream[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEYS.favoriteStreams);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(
      (item): item is FavoriteStream =>
        Boolean(item) &&
        typeof item === "object" &&
        typeof (item as FavoriteStream).clusterId === "string" &&
        typeof (item as FavoriteStream).streamName === "string" &&
        (item as FavoriteStream).clusterId.length > 0 &&
        (item as FavoriteStream).streamName.length > 0,
    );
  } catch {
    return [];
  }
}

export function writeFavoriteStreams(favorites: FavoriteStream[]): void {
  localStorage.setItem(STORAGE_KEYS.favoriteStreams, JSON.stringify(favorites));
}

export function isFavoriteStream(clusterId: string, streamName: string, favorites = readFavoriteStreams()): boolean {
  const key = favoriteKey({ clusterId, streamName });
  return favorites.some((fav) => favoriteKey(fav) === key);
}

export function toggleFavoriteStream(clusterId: string, streamName: string): FavoriteStream[] {
  const current = readFavoriteStreams();
  const key = favoriteKey({ clusterId, streamName });
  const exists = current.some((fav) => favoriteKey(fav) === key);
  const next = exists
    ? current.filter((fav) => favoriteKey(fav) !== key)
    : [...current, { clusterId, streamName }];
  writeFavoriteStreams(next);
  return next;
}

export function favoritesForCluster(clusterId: string, favorites = readFavoriteStreams()): FavoriteStream[] {
  return favorites.filter((fav) => fav.clusterId === clusterId);
}

/** Favorites first, then by message count, then consumers, then name. */
export function sortStreamsFavoritesFirst<
  T extends { config: { name: string }; state?: { messages?: number; consumerCount?: number } },
>(streams: T[], clusterId: string, favorites = readFavoriteStreams()): T[] {
  if (streams.length === 0) return streams;
  return [...streams].sort((a, b) => {
    const aFav = isFavoriteStream(clusterId, a.config.name, favorites) ? 1 : 0;
    const bFav = isFavoriteStream(clusterId, b.config.name, favorites) ? 1 : 0;
    if (aFav !== bFav) return bFav - aFav;

    const aMsg = a.state?.messages ?? 0;
    const bMsg = b.state?.messages ?? 0;
    if (aMsg !== bMsg) return bMsg - aMsg;

    const aCons = a.state?.consumerCount ?? 0;
    const bCons = b.state?.consumerCount ?? 0;
    if (aCons !== bCons) return bCons - aCons;

    return a.config.name.localeCompare(b.config.name);
  });
}
