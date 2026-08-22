import { api, clusterPath, type StreamInfo } from "./api";
import { DEFAULT_PAGE_SIZE } from "./constants";

/** Fetch every stream page until meta.total is covered (never silently truncate). */
export async function fetchAllStreams(
  clusterId: string,
  pageSize = DEFAULT_PAGE_SIZE,
): Promise<StreamInfo[]> {
  const all: StreamInfo[] = [];
  let offset = 0;

  while (true) {
    const page = await api<StreamInfo[]>(
      clusterPath(clusterId, `/streams?offset=${offset}&limit=${pageSize}`),
    );
    const streams = page.data ?? [];
    all.push(...streams);
    if (offset + streams.length >= (page.meta?.total ?? 0) || streams.length === 0) {
      break;
    }
    offset += streams.length;
  }

  return all;
}
