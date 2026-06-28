import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import Pager, { DEFAULT_PAGE_SIZE, pageQuery } from "../components/Pager";
import VirtualTable from "../components/VirtualTable";
import Alert from "../components/ui/Alert";
import PageHeader from "../components/ui/PageHeader";
import { api, clusterPath, decodeBase64, ObjectInfo, tryParseJSON } from "../lib/api";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey } from "../lib/query";

type ObjectListResponse = {
  objects: string[];
  total: number;
  offset: number;
  limit: number;
};

const PREVIEW_LIMIT = 8192;

export default function ObjectBucketPage() {
  const { bucket = "" } = useParams();
  const { clusterId } = useCluster();
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<ObjectInfo | null>(null);
  const [showFull, setShowFull] = useState(false);
  const [actionError, setActionError] = useState("");
  const limit = DEFAULT_PAGE_SIZE;

  const objectsQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, `objects:${bucket}:${offset}`),
    queryFn: () =>
      api<ObjectListResponse>(
        clusterPath(clusterId!, `/objects/buckets/${encodeURIComponent(bucket)}/objects${pageQuery(offset, limit)}`),
      ),
    enabled: Boolean(clusterId && bucket),
  });

  const objects = objectsQuery.data?.objects ?? [];
  const total = objectsQuery.data?.total ?? 0;
  const error =
    actionError || (objectsQuery.error instanceof Error ? objectsQuery.error.message : "");

  async function loadObject(name: string) {
    if (!clusterId) return;
    try {
      const info = await api<ObjectInfo>(
        clusterPath(clusterId, `/objects/buckets/${encodeURIComponent(bucket)}/objects/${encodeURIComponent(name)}`),
      );
      setSelected(info);
      setShowFull(false);
      setActionError("");
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to load object");
    }
  }

  const payload = selected ? decodeBase64(selected.data) : "";
  const parsed = tryParseJSON(payload);
  const truncated = !showFull && payload.length > PREVIEW_LIMIT;
  const displayPayload = truncated ? `${payload.slice(0, PREVIEW_LIMIT)}\n…` : payload;

  return (
    <div className="page">
      <PageHeader
        eyebrow="Object store"
        title={bucket}
        subtitle="Browse objects in this bucket and inspect payloads."
        actions={
          <Link to="/objects" className="btn btn--secondary">
            ← All buckets
          </Link>
        }
      />

      <Alert variant="error">{error}</Alert>

      {objectsQuery.isLoading && <div className="skeleton skeleton--table" />}

      {!objectsQuery.isLoading && (
        <div className="split-view">
          <div className="table-wrap">
            <VirtualTable
              columns={[
                { id: "object", header: "Object", width: "minmax(0, 1fr)" },
                { id: "actions", header: "", width: "96px", align: "right" },
              ]}
              items={objects}
              empty="No objects in this bucket"
              getKey={(name) => name}
              renderCell={(name, columnId) => {
                if (columnId === "object") {
                  return <span className="mono virtual-table__truncate">{name}</span>;
                }
                return (
                  <button className="btn btn--secondary btn--small" type="button" onClick={() => loadObject(name)}>
                    View
                  </button>
                );
              }}
            />
            <Pager total={total} offset={offset} limit={limit} onPageChange={setOffset} />
          </div>

          {selected && (
            <div className="card panel">
              <div className="card-label">
                {selected.name} · {selected.size} bytes · {selected.modified}
              </div>
              {truncated && (
                <button className="btn btn--secondary btn--small" type="button" onClick={() => setShowFull(true)}>
                  Show full payload
                </button>
              )}
              <pre className="mono">{parsed.isJSON && !truncated ? JSON.stringify(parsed.parsed, null, 2) : displayPayload}</pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
