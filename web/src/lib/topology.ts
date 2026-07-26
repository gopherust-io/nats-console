import { api, clusterPath, jetStreamUIBase, type ConsumerInfo, type StreamInfo } from "./api";
import { TOPOLOGY_PAGE_SIZE } from "./constants";

export type TopologyNodeKind = "cluster" | "stream" | "subject" | "consumer";

export type TopologyNode = {
  id: string;
  kind: TopologyNodeKind;
  name: string;
  meta?: string[];
  href?: string;
  status?: "healthy" | "warning" | "idle";
  children: TopologyNode[];
};

/** Location state marker so stream/consumer pages can return to Topology. */
export const TOPOLOGY_LOCATION_STATE = { from: "topology" } as const;

export type TopologyLocationState = { from?: string };

export function isFromTopology(state: unknown): boolean {
  return Boolean(state && typeof state === "object" && (state as TopologyLocationState).from === "topology");
}

/** Rewrite stream/consumer hrefs to account-scoped JetStream UI paths. */
export function withJetStreamHrefs(
  root: TopologyNode,
  clusterId: string,
  accountName: string,
): TopologyNode {
  const jsBase = jetStreamUIBase(clusterId, accountName || "Default");

  function walk(node: TopologyNode, parentStream?: string): TopologyNode {
    let href = node.href;
    if (node.kind === "stream") {
      href = `${jsBase}/streams/${encodeURIComponent(node.name)}`;
    } else if (node.kind === "consumer" && parentStream) {
      href = `${jsBase}/streams/${encodeURIComponent(parentStream)}/consumers/${encodeURIComponent(node.name)}`;
    }
    const streamName = node.kind === "stream" ? node.name : parentStream;
    return {
      ...node,
      href,
      children: (Array.isArray(node.children) ? node.children : []).map((child) => walk(child, streamName)),
    };
  }

  return walk(root);
}

type StreamListResponse = {
  streams: StreamInfo[];
  total: number;
  offset: number;
  limit: number;
};

type ConsumerListResponse = {
  consumers: ConsumerInfo[];
  total: number;
  offset: number;
  limit: number;
};

const PAGE_SIZE = TOPOLOGY_PAGE_SIZE;
const CONSUMER_FETCH_CONCURRENCY = 6;

async function mapConcurrent<T, R>(items: T[], limit: number, fn: (item: T) => Promise<R>): Promise<R[]> {
  if (items.length === 0) return [];
  const results = new Array<R>(items.length);
  let nextIndex = 0;

  async function worker() {
    while (nextIndex < items.length) {
      const index = nextIndex;
      nextIndex += 1;
      results[index] = await fn(items[index]);
    }
  }

  const workers = Array.from({ length: Math.min(limit, items.length) }, () => worker());
  await Promise.all(workers);
  return results;
}

function consumerHealthStatus(pending: number, ackPending: number): TopologyNode["status"] {
  if (pending > 0 || ackPending > 0) return "warning";
  return "healthy";
}

function createStreamTopologyNode(
  stream: {
    name: string;
    subjects: string[];
    messages: number;
    consumerCount: number;
    storage?: string;
    retention?: string;
  },
  consumers: Array<{
    name: string;
    filterSubject?: string;
    pending: number;
    ackPending: number;
    deliverPolicy?: string;
  }>,
): TopologyNode {
  const subjectNodes: TopologyNode[] = (stream.subjects ?? []).map((subject) => ({
    id: `subject:${stream.name}:${subject}`,
    kind: "subject",
    name: subject,
    children: [],
  }));

  const consumerNodes: TopologyNode[] = consumers.map((consumer) => {
    const meta: string[] = [];
    if (consumer.filterSubject) meta.push(`filter ${consumer.filterSubject}`);
    if (consumer.deliverPolicy) meta.push(consumer.deliverPolicy);
    if (consumer.pending > 0) meta.push("pending");
    return {
      id: `consumer:${stream.name}:${consumer.name}`,
      kind: "consumer" as const,
      name: consumer.name,
      meta,
      status: consumerHealthStatus(consumer.pending, consumer.ackPending),
      href: `/streams/${stream.name}/consumers/${consumer.name}`,
      children: [],
    };
  });

  const meta = [`${stream.messages} msgs`, `${Math.max(stream.consumerCount, consumers.length)} consumers`];
  if (stream.storage) meta.push(stream.storage);
  if (stream.retention) meta.push(stream.retention);

  return {
    id: `stream:${stream.name}`,
    kind: "stream",
    name: stream.name,
    meta,
    href: `/streams/${stream.name}`,
    status: "healthy",
    children: [...subjectNodes, ...consumerNodes],
  };
}

async function fetchAllStreams(clusterId: string): Promise<StreamInfo[]> {
  const all: StreamInfo[] = [];
  let offset = 0;

  while (true) {
    const page = await api<StreamListResponse>(
      clusterPath(clusterId, `/streams?offset=${offset}&limit=${PAGE_SIZE}`),
    );
    const streams = page.streams ?? [];
    all.push(...streams);
    if (offset + streams.length >= page.total || streams.length === 0) {
      break;
    }
    offset += PAGE_SIZE;
  }

  return all;
}

async function fetchAllConsumers(clusterId: string, streamName: string): Promise<ConsumerInfo[]> {
  const all: ConsumerInfo[] = [];
  let offset = 0;

  while (true) {
    const page = await api<ConsumerListResponse>(
      clusterPath(
        clusterId,
        `/streams/${encodeURIComponent(streamName)}/consumers?offset=${offset}&limit=${PAGE_SIZE}`,
      ),
    );
    const consumers = page.consumers ?? [];
    all.push(...consumers);
    if (offset + consumers.length >= page.total || consumers.length === 0) {
      break;
    }
    offset += PAGE_SIZE;
  }

  return all;
}

async function buildTopologyFromAPI(clusterId: string, clusterName: string): Promise<TopologyNode> {
  const streams = await fetchAllStreams(clusterId);
  const consumerLists = await mapConcurrent(streams, CONSUMER_FETCH_CONCURRENCY, (stream) =>
    fetchAllConsumers(clusterId, stream.config.name),
  );
  const streamNodes: TopologyNode[] = streams.map((stream, index) => {
    const name = stream.config.name;
    const consumers = consumerLists[index] ?? [];
    return createStreamTopologyNode(
      {
        name,
        subjects: stream.config.subjects ?? [],
        messages: stream.state.messages,
        consumerCount: stream.state.consumerCount,
        storage: stream.config.storage,
        retention: stream.config.retention,
      },
      consumers.map((consumer) => ({
        name: consumer.name,
        filterSubject: consumer.config.filterSubject,
        pending: consumer.numPending,
        ackPending: consumer.numAckPending,
        deliverPolicy: consumer.config.deliverPolicy,
      })),
    );
  });

  streamNodes.sort((a, b) => a.name.localeCompare(b.name));

  return {
    id: "cluster:root",
    kind: "cluster",
    name: clusterName,
    meta: [`${streamNodes.length} streams`],
    children: streamNodes,
  };
}

/** Prefer backend aggregate (hub/jsz); fall back to REST only if that fails.
 * Pass fresh=true after mutations so the ~60s snapshot hub is bypassed. */
export async function fetchTopology(
  clusterId: string,
  clusterName: string,
  options?: { fresh?: boolean },
): Promise<TopologyNode> {
  try {
    const fresh = options?.fresh ? "&fresh=1" : "";
    const tree = await api<TopologyNode>(
      clusterPath(clusterId, `/topology?name=${encodeURIComponent(clusterName)}${fresh}`),
    );
    if (tree?.kind === "cluster") {
      return normalizeTopologyNode(tree);
    }
  } catch {
    // Fall back to REST aggregation below.
  }

  return buildTopologyFromAPI(clusterId, clusterName);
}

/** Ensure every node has a real children array (API may encode nil slices as null). */
export function normalizeTopologyNode(node: TopologyNode): TopologyNode {
  const children = Array.isArray(node.children) ? node.children.map(normalizeTopologyNode) : [];
  return { ...node, children };
}

function nodeChildren(node: TopologyNode): TopologyNode[] {
  return Array.isArray(node.children) ? node.children : [];
}

export function flattenTopology(node: TopologyNode): TopologyNode[] {
  const flattened: TopologyNode[] = [node];
  for (const child of nodeChildren(node)) {
    flattened.push(...flattenTopology(child));
  }
  return flattened;
}

export function filterTopology(node: TopologyNode, filterQuery: string): TopologyNode | null {
  const normalizedFilter = filterQuery.trim().toLowerCase();
  if (!normalizedFilter) return node;

  const filteredChildren = nodeChildren(node)
    .map((child) => filterTopology(child, normalizedFilter))
    .filter((child): child is TopologyNode => child !== null);

  const selfMatch =
    node.name.toLowerCase().includes(normalizedFilter) ||
    node.kind.toLowerCase().includes(normalizedFilter) ||
    (node.meta ?? []).some((item) => item.toLowerCase().includes(normalizedFilter));

  if (selfMatch || filteredChildren.length > 0) {
    return {
      ...node,
      children: selfMatch ? nodeChildren(node) : filteredChildren,
    };
  }

  return null;
}

export function countTopology(node: TopologyNode) {
  let streams = 0;
  let subjects = 0;
  let consumers = 0;

  function walk(current: TopologyNode) {
    switch (current.kind) {
      case "stream":
        streams += 1;
        break;
      case "subject":
        subjects += 1;
        break;
      case "consumer":
        consumers += 1;
        break;
    }
    for (const child of nodeChildren(current)) {
      walk(child);
    }
  }

  walk(node);
  return { streams, subjects, consumers };
}

export function getStreamNodes(root: TopologyNode): TopologyNode[] {
  return nodeChildren(root).filter((child) => child.kind === "stream");
}

export function splitStreamChildren(stream: TopologyNode) {
  const children = nodeChildren(stream);
  return {
    subjects: children.filter((child) => child.kind === "subject"),
    consumers: children.filter((child) => child.kind === "consumer"),
  };
}

export function findStreamById(root: TopologyNode, streamId: string): TopologyNode | null {
  return getStreamNodes(root).find((stream) => stream.id === streamId) ?? null;
}

export function findNodeById(root: TopologyNode, id: string): TopologyNode | null {
  if (root.id === id) return root;
  for (const child of nodeChildren(root)) {
    const found = findNodeById(child, id);
    if (found) return found;
  }
  return null;
}

/** Stream that owns `nodeId`, or the node itself when it is a stream. */
export function findParentStream(root: TopologyNode, nodeId: string): TopologyNode | null {
  for (const stream of getStreamNodes(root)) {
    if (stream.id === nodeId) return stream;
    if (nodeChildren(stream).some((child) => child.id === nodeId)) return stream;
  }
  return null;
}

export type StreamOverviewSort = "name" | "messages" | "consumers" | "subjects";

export function streamMessageCount(stream: TopologyNode): number {
  const match = stream.meta?.find((item) => item.endsWith(" msgs"));
  if (!match) return 0;
  const value = Number.parseInt(match, 10);
  return Number.isFinite(value) ? value : 0;
}

export function sortStreamNodes(streams: TopologyNode[], sortBy: StreamOverviewSort): TopologyNode[] {
  const sorted = [...streams];
  sorted.sort((a, b) => {
    const { subjects: aSubjects, consumers: aConsumers } = splitStreamChildren(a);
    const { subjects: bSubjects, consumers: bConsumers } = splitStreamChildren(b);

    switch (sortBy) {
      case "messages":
        return streamMessageCount(b) - streamMessageCount(a) || a.name.localeCompare(b.name);
      case "consumers":
        return bConsumers.length - aConsumers.length || a.name.localeCompare(b.name);
      case "subjects":
        return bSubjects.length - aSubjects.length || a.name.localeCompare(b.name);
      default:
        return a.name.localeCompare(b.name);
    }
  });
  return sorted;
}
