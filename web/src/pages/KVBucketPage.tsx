import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import Pager, { DEFAULT_PAGE_SIZE, pageQuery } from "../components/Pager";
import VirtualTable from "../components/VirtualTable";
import { api, clusterPath } from "../lib/api";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey } from "../lib/query";

type KeyListResponse = {
  keys: string[];
  total: number;
  offset: number;
  limit: number;
};

export default function KVBucketPage() {
  const { bucket = "" } = useParams();
  const { clusterId } = useCluster();
  const [offset, setOffset] = useState(0);
  const limit = DEFAULT_PAGE_SIZE;

  const keysQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, `kv-keys:${bucket}:${offset}`),
    queryFn: () =>
      api<KeyListResponse>(
        clusterPath(clusterId!, `/kv/buckets/${encodeURIComponent(bucket)}/keys${pageQuery(offset, limit)}`),
      ),
    enabled: Boolean(clusterId && bucket),
  });

  const keys = keysQuery.data?.keys ?? [];
  const total = keysQuery.data?.total ?? 0;
  const error = keysQuery.error instanceof Error ? keysQuery.error.message : "";

  return (
    <div>
      <div className="page-header">
        <div>
          <Link to="/kv" className="link-back">
            ← Back to KV Stores
          </Link>
          <h1>{bucket}</h1>
        </div>
      </div>

      {error && <div className="error">{error}</div>}

      <div className="table-wrap">
        <VirtualTable
          columns={[{ id: "key", header: "Key", width: "minmax(0, 1fr)" }]}
          items={keys}
          empty="No keys"
          getKey={(key) => key}
          renderCell={(key) => (
            <Link to={`/kv/${bucket}/${encodeURIComponent(key)}`} className="mono virtual-table__truncate">
              {key}
            </Link>
          )}
        />
      </div>

      <Pager total={total} offset={offset} limit={limit} onPageChange={setOffset} />
    </div>
  );
}
