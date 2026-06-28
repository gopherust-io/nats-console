import { FormEvent, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import Pager, { DEFAULT_PAGE_SIZE, pageQuery } from "../components/Pager";
import VirtualTable from "../components/VirtualTable";
import Alert from "../components/ui/Alert";
import PageHeader from "../components/ui/PageHeader";
import { api, clusterPath, StreamInfo } from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey } from "../lib/query";

type StreamListResponse = {
  streams: StreamInfo[];
  total: number;
  offset: number;
  limit: number;
};

export default function StreamsPage() {
  const { clusterId } = useCluster();
  const { canWrite } = useAuth();
  const queryClient = useQueryClient();
  const [offset, setOffset] = useState(0);
  const [actionError, setActionError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState("");
  const [subjects, setSubjects] = useState("events.>");
  const [retention, setRetention] = useState("limits");
  const [storage, setStorage] = useState("file");
  const [maxMsgs, setMaxMsgs] = useState("");
  const [maxBytes, setMaxBytes] = useState("");
  const limit = DEFAULT_PAGE_SIZE;

  const streamsQuery = useQuery({
    queryKey: [...clusterQueryKey(clusterId, "streams"), offset],
    queryFn: () =>
      api<StreamListResponse>(clusterPath(clusterId!, `/streams${pageQuery(offset, limit)}`)),
    enabled: Boolean(clusterId),
  });

  const streams = streamsQuery.data?.streams ?? [];
  const total = streamsQuery.data?.total ?? 0;
  const error =
    actionError ||
    (streamsQuery.error instanceof Error ? streamsQuery.error.message : "");

  async function invalidateStreams() {
    await queryClient.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "streams") });
  }

  async function createStream(event: FormEvent) {
    event.preventDefault();
    if (!clusterId) return;
    try {
      const body: Record<string, unknown> = {
        name,
        subjects: subjects.split(",").map((s) => s.trim()).filter(Boolean),
        retention,
        storage,
      };
      if (maxMsgs) body.maxMsgs = Number(maxMsgs);
      if (maxBytes) body.maxBytes = Number(maxBytes);

      await api(clusterPath(clusterId, "/streams"), {
        method: "POST",
        body: JSON.stringify(body),
      });
      setShowForm(false);
      setName("");
      setActionError("");
      await invalidateStreams();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to create stream");
    }
  }

  async function deleteStream(streamName: string) {
    if (!clusterId || !confirm(`Delete stream "${streamName}"?`)) return;
    try {
      await api(clusterPath(clusterId, `/streams/${encodeURIComponent(streamName)}`), { method: "DELETE" });
      setActionError("");
      await invalidateStreams();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to delete stream");
    }
  }

  return (
    <div className="page">
      <PageHeader
        eyebrow="JetStream"
        title="Streams"
        subtitle="Create, inspect, and manage message streams across subjects."
        actions={
          canWrite ? (
            <button className="btn" type="button" onClick={() => setShowForm((v) => !v)}>
              {showForm ? "Cancel" : "Create stream"}
            </button>
          ) : undefined
        }
      />

      <Alert variant="error">{error}</Alert>

      {showForm && (
        <form className="form-grid card mb-24" onSubmit={createStream}>
          <label>
            Name
            <input value={name} onChange={(e) => setName(e.target.value)} required />
          </label>
          <label>
            Subjects (comma separated)
            <input value={subjects} onChange={(e) => setSubjects(e.target.value)} required />
          </label>
          <label>
            Retention
            <select value={retention} onChange={(e) => setRetention(e.target.value)}>
              <option value="limits">limits</option>
              <option value="interest">interest</option>
              <option value="workqueue">workqueue</option>
            </select>
          </label>
          <label>
            Storage
            <select value={storage} onChange={(e) => setStorage(e.target.value)}>
              <option value="file">file</option>
              <option value="memory">memory</option>
            </select>
          </label>
          <label>
            Max Messages (optional)
            <input value={maxMsgs} onChange={(e) => setMaxMsgs(e.target.value)} type="number" />
          </label>
          <label>
            Max Bytes (optional)
            <input value={maxBytes} onChange={(e) => setMaxBytes(e.target.value)} type="number" />
          </label>
          <button className="btn" type="submit">
            Save Stream
          </button>
        </form>
      )}

      {streamsQuery.isLoading && <div className="skeleton skeleton--table" />}

      {!streamsQuery.isLoading && (
        <div className="table-wrap">
          <VirtualTable
            columns={[
              { id: "name", header: "Name", width: "minmax(120px, 1.1fr)" },
              { id: "subjects", header: "Subjects", width: "minmax(180px, 2fr)" },
              { id: "messages", header: "Messages", width: "96px", align: "right" },
              { id: "consumers", header: "Consumers", width: "108px", align: "right" },
              { id: "storage", header: "Storage", width: "96px" },
              { id: "actions", header: "", width: "112px", align: "right" },
            ]}
            items={streams}
            empty="No streams yet"
            getKey={(stream) => stream.config.name}
            renderCell={(stream, columnId) => {
              switch (columnId) {
                case "name":
                  return <Link to={`/streams/${stream.config.name}`}>{stream.config.name}</Link>;
                case "subjects":
                  return (
                    <span className="mono virtual-table__truncate">
                      {(stream.config.subjects ?? []).join(", ")}
                    </span>
                  );
                case "messages":
                  return stream.state.messages ?? 0;
                case "consumers":
                  return stream.state.consumerCount ?? 0;
                case "storage":
                  return stream.config.storage;
                case "actions":
                  return canWrite ? (
                    <button className="btn danger btn--small" type="button" onClick={() => deleteStream(stream.config.name)}>
                      Delete
                    </button>
                  ) : null;
                default:
                  return null;
              }
            }}
          />
        </div>
      )}

      <Pager total={total} offset={offset} limit={limit} onPageChange={setOffset} />
    </div>
  );
}
