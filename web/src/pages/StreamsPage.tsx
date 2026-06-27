import { FormEvent, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, clusterPath, StreamInfo } from "../lib/api";
import { useCluster } from "../lib/cluster";

type StreamListResponse = {
  streams: StreamInfo[];
  total: number;
};

export default function StreamsPage() {
  const { clusterId } = useCluster();
  const [streams, setStreams] = useState<StreamInfo[]>([]);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState("");
  const [subjects, setSubjects] = useState("events.>");
  const [retention, setRetention] = useState("limits");
  const [storage, setStorage] = useState("file");
  const [maxMsgs, setMaxMsgs] = useState("");
  const [maxBytes, setMaxBytes] = useState("");

  async function loadStreams() {
    if (!clusterId) return;
    try {
      const data = await api<StreamListResponse>(clusterPath(clusterId, "/streams"));
      setStreams(data.streams);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load streams");
    }
  }

  useEffect(() => {
    loadStreams();
  }, [clusterId]);

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
      if (maxMsgs) body.max_msgs = Number(maxMsgs);
      if (maxBytes) body.max_bytes = Number(maxBytes);

      await api(clusterPath(clusterId, "/streams"), {
        method: "POST",
        body: JSON.stringify(body),
      });
      setShowForm(false);
      setName("");
      await loadStreams();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create stream");
    }
  }

  async function deleteStream(streamName: string) {
    if (!clusterId || !confirm(`Delete stream "${streamName}"?`)) return;
    try {
      await api(clusterPath(clusterId, `/streams/${encodeURIComponent(streamName)}`), { method: "DELETE" });
      await loadStreams();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete stream");
    }
  }

  return (
    <div>
      <div className="page-header">
        <h1>Streams</h1>
        <button className="btn" onClick={() => setShowForm((v) => !v)}>
          {showForm ? "Cancel" : "Create Stream"}
        </button>
      </div>

      {error && <div className="error">{error}</div>}

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

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Subjects</th>
              <th>Messages</th>
              <th>Consumers</th>
              <th>Storage</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {streams.map((stream) => (
              <tr key={stream.config.name}>
                <td>
                  <Link to={`/streams/${stream.config.name}`}>{stream.config.name}</Link>
                </td>
                <td className="mono">{(stream.config.subjects ?? []).join(", ")}</td>
                <td>{stream.state.messages}</td>
                <td>{stream.state.consumer_count}</td>
                <td>{stream.config.storage}</td>
                <td>
                  <button className="btn danger" onClick={() => deleteStream(stream.config.name)}>
                    Delete
                  </button>
                </td>
              </tr>
            ))}
            {streams.length === 0 && (
              <tr>
                <td colSpan={6} className="text-muted">
                  No streams yet
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
