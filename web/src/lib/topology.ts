import { api, clusterPath, type ConsumerInfo, type StreamInfo } from "./api";
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

type JSZStreamDetail = {
  name: string;
  config?: {
    subjects?: string[];
    retention?: string;
    storage?: string;
  };
  state?: {
    messages?: number;
    consumer_count?: number;
    bytes?: number;
  };
  consumer_detail?: JSZConsumerDetail[];
};

type JSZConsumerDetail = {
  name: string;
  stream_name?: string;
  config?: {
    filter_subject?: string;
    durable_name?: string;
    deliver_policy?: string;
    ack_policy?: string;
  };
  num_pending?: number;
  num_ack_pending?: number;
};

type JSZTopologyResponse = {
  account_details?: Array<{
    name: string;
    stream_detail?: JSZStreamDetail[];
  }>;
  total?: {
    streams?: number;
    consumers?: number;
  };
};

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
  const children: TopologyNode[] = [];

  for (const subject of stream.subjects) {
    children.push({
      id: `stream:${stream.name}:subject:${subject}`,
      kind: "subject",
      name: subject,
      meta: ["Captures published messages"],
      children: [],
    });
  }

  for (const consumer of consumers) {
    const meta: string[] = [];
    if (consumer.filterSubject) meta.push(`Filter ${consumer.filterSubject}`);
    if (consumer.deliverPolicy) meta.push(consumer.deliverPolicy);
    if (consumer.pending > 0) meta.push(`${consumer.pending} pending`);
    if (consumer.ackPending > 0) meta.push(`${consumer.ackPending} ack pending`);

    children.push({
      id: `stream:${stream.name}:consumer:${consumer.name}`,
      kind: "consumer",
      name: consumer.name,
      meta,
      href: `/streams/${encodeURIComponent(stream.name)}/consumers/${encodeURIComponent(consumer.name)}`,
      status: consumerHealthStatus(consumer.pending, consumer.ackPending),
      children: [],
    });
  }

  const streamMeta: string[] = [];
  if (stream.storage) streamMeta.push(stream.storage);
  if (stream.retention) streamMeta.push(stream.retention);
  streamMeta.push(`${stream.messages} msgs`);
  streamMeta.push(`${stream.consumerCount} consumers`);

  return {
    id: `stream:${stream.name}`,
    kind: "stream",
    name: stream.name,
    meta: streamMeta,
    href: `/streams/${encodeURIComponent(stream.name)}`,
    status: stream.messages > 0 || stream.consumerCount > 0 ? "healthy" : "idle",
    children,
  };
}

function buildTopologyFromMonitoring(jsz: JSZTopologyResponse, clusterName: string): TopologyNode | null {
  const streamDetails =
    jsz.account_details?.flatMap((account) => account.stream_detail ?? []) ?? [];

  if (streamDetails.length === 0) {
    return null;
  }

  const streamNodes = streamDetails.map((stream) => {
    const subjects = stream.config?.subjects ?? [];
    const consumers =
      stream.consumer_detail?.map((consumer) => ({
        name: consumer.name,
        filterSubject: consumer.config?.filter_subject,
        pending: consumer.num_pending ?? 0,
        ackPending: consumer.num_ack_pending ?? 0,
        deliverPolicy: consumer.config?.deliver_policy,
      })) ?? [];

    return createStreamTopologyNode(
      {
        name: stream.name,
        subjects,
        messages: stream.state?.messages ?? 0,
        consumerCount: stream.state?.consumer_count ?? consumers.length,
        storage: stream.config?.storage,
        retention: stream.config?.retention,
      },
      consumers,
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
  const streamNodes: TopologyNode[] = [];

  for (const stream of streams) {
    const name = stream.config.name;
    const consumers = await fetchAllConsumers(clusterId, name);
    streamNodes.push(
      createStreamTopologyNode(
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
      ),
    );
  }

  streamNodes.sort((a, b) => a.name.localeCompare(b.name));

  return {
    id: "cluster:root",
    kind: "cluster",
    name: clusterName,
    meta: [`${streamNodes.length} streams`],
    children: streamNodes,
  };
}

export async function fetchTopology(clusterId: string, clusterName: string): Promise<TopologyNode> {
  try {
    const jsz = await api<JSZTopologyResponse>(
      clusterPath(clusterId, "/monitoring/jsz?streams=1&consumers=1&config=1"),
    );
    const fromMonitoring = buildTopologyFromMonitoring(jsz, clusterName);
    if (fromMonitoring) {
      return fromMonitoring;
    }
  } catch {
    // Fall back to REST aggregation below.
  }

  return buildTopologyFromAPI(clusterId, clusterName);
}

export function flattenTopology(node: TopologyNode): TopologyNode[] {
  const flattened: TopologyNode[] = [node];
  for (const child of node.children) {
    flattened.push(...flattenTopology(child));
  }
  return flattened;
}

export function filterTopology(node: TopologyNode, filterQuery: string): TopologyNode | null {
  const normalizedFilter = filterQuery.trim().toLowerCase();
  if (!normalizedFilter) return node;

  const filteredChildren = node.children
    .map((child) => filterTopology(child, normalizedFilter))
    .filter((child): child is TopologyNode => child !== null);

  const selfMatch =
    node.name.toLowerCase().includes(normalizedFilter) ||
    node.kind.toLowerCase().includes(normalizedFilter) ||
    (node.meta ?? []).some((item) => item.toLowerCase().includes(normalizedFilter));

  if (selfMatch || filteredChildren.length > 0) {
    return {
      ...node,
      children: selfMatch ? node.children : filteredChildren,
    };
  }

  return null;
}

export function countTopology(node: TopologyNode) {
  const flattened = flattenTopology(node);
  return {
    streams: flattened.filter((node) => node.kind === "stream").length,
    subjects: flattened.filter((node) => node.kind === "subject").length,
    consumers: flattened.filter((node) => node.kind === "consumer").length,
  };
}

export function getStreamNodes(root: TopologyNode): TopologyNode[] {
  return root.children.filter((child) => child.kind === "stream");
}

export function splitStreamChildren(stream: TopologyNode) {
  return {
    subjects: stream.children.filter((child) => child.kind === "subject"),
    consumers: stream.children.filter((child) => child.kind === "consumer"),
  };
}

export function findStreamById(root: TopologyNode, streamId: string): TopologyNode | null {
  return getStreamNodes(root).find((stream) => stream.id === streamId) ?? null;
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
