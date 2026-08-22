import { api, clusterPath, jetStreamUIBase, type ConsumerInfo, type StreamInfo } from "./api";
import { consumerLag, isSlowConsumer } from "./consumerMetrics";
import { TOPOLOGY_PAGE_SIZE } from "./constants";
import { fetchAllStreams } from "./streams";

export type TopologyNodeKind = "cluster" | "stream" | "subject" | "consumer";

export type TopologyNodeStatus = "healthy" | "warning" | "idle" | "unhealthy";

export type TopologyNodeRole = "leader" | "replica" | "standalone";

export type TopologyPeer = {
  name: string;
  current: boolean;
  offline?: boolean;
  lag?: number;
  active?: number;
};

export type TopologyRaft = {
  group?: string;
  leader?: string;
  clusterSize?: number;
  peers?: TopologyPeer[];
};

export type TopologyNode = {
  id: string;
  kind: TopologyNodeKind;
  name: string;
  meta?: string[];
  href?: string;
  status?: TopologyNodeStatus;
  role?: TopologyNodeRole;
  raft?: TopologyRaft;
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

type ConsumerListResponse = ConsumerInfo[];

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

function consumerHealthStatus(
  pending: number,
  ackPending: number,
  lag = 0,
  maxAckPending = 0,
): TopologyNode["status"] {
  if (
    isSlowConsumer({
      pending,
      lag,
      ackPending,
      maxAckPending,
    })
  ) {
    return "warning";
  }
  return "healthy";
}

function createStreamTopologyNode(
  stream: {
    name: string;
    subjects: string[];
    messages: number;
    consumerCount: number;
    lastSeq?: number;
    storage?: string;
    retention?: string;
  },
  consumers: Array<{
    name: string;
    filterSubject?: string;
    pending: number;
    ackPending: number;
    waiting?: number;
    redelivered?: number;
    deliveredStreamSeq?: number;
    deliverPolicy?: string;
    maxAckPending?: number;
  }>,
): TopologyNode {
  const subjectNodes: TopologyNode[] = (stream.subjects ?? []).map((subject) => ({
    id: `subject:${stream.name}:${subject}`,
    kind: "subject",
    name: subject,
    children: [],
  }));

  const consumerNodes: TopologyNode[] = consumers.map((consumer) => {
    const waiting = consumer.waiting ?? 0;
    const redelivered = consumer.redelivered ?? 0;
    const lag = consumerLag(stream.lastSeq ?? 0, consumer.deliveredStreamSeq);
    const maxAckPending = consumer.maxAckPending ?? 0;
    const slow = isSlowConsumer({
      pending: consumer.pending,
      lag,
      ackPending: consumer.ackPending,
      maxAckPending,
    });
    const meta: string[] = [];
    if (consumer.filterSubject) meta.push(`filter ${consumer.filterSubject}`);
    if (consumer.deliverPolicy) meta.push(consumer.deliverPolicy);
    if (slow) meta.push("slow");
    if (consumer.pending >= 1000) meta.push("pending");
    if (maxAckPending > 0 && consumer.ackPending >= Math.max(1, Math.floor(maxAckPending * 0.9))) {
      meta.push("ack pending");
    }
    if (waiting > 0) meta.push("waiting");
    if (redelivered > 0) meta.push("redelivered");
    if (lag >= 1000) meta.push(`lag ${lag}`);
    return {
      id: `consumer:${stream.name}:${consumer.name}`,
      kind: "consumer" as const,
      name: consumer.name,
      meta,
      status: consumerHealthStatus(consumer.pending, consumer.ackPending, lag, maxAckPending),
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

async function fetchAllStreamsForTopology(clusterId: string): Promise<StreamInfo[]> {
  return fetchAllStreams(clusterId, PAGE_SIZE);
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
    const consumers = page.data ?? [];
    all.push(...consumers);
    if (offset + consumers.length >= (page.meta?.total ?? 0) || consumers.length === 0) {
      break;
    }
    offset += consumers.length;
  }

  return all;
}

async function buildTopologyFromAPI(clusterId: string, clusterName: string): Promise<TopologyNode> {
  const streams = await fetchAllStreamsForTopology(clusterId);
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
        lastSeq: stream.state.lastSeq,
        storage: stream.config.storage,
        retention: stream.config.retention,
      },
      consumers.map((consumer) => ({
        name: consumer.name,
        filterSubject: consumer.config.filterSubject,
        pending: consumer.numPending,
        ackPending: consumer.numAckPending,
        waiting: consumer.numWaiting,
        redelivered: consumer.numRedelivered,
        deliveredStreamSeq: consumer.delivered?.streamSeq,
        deliverPolicy: consumer.config.deliverPolicy,
        maxAckPending: consumer.config.maxAckPending,
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
    if (tree.data?.kind === "cluster") {
      return normalizeTopologyNode(tree.data);
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
      // Self-match expands full children so filtering by a stream name still
      // reveals its subjects/consumers (including slow-consumer nodes).
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
