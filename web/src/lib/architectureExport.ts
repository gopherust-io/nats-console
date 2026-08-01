import { clusterPath } from "./api";
import { downloadBlob, sanitizeFilenamePart } from "./messageDownload";

export type ArchitectureExportOptions = {
  demo?: boolean;
  ai?: boolean;
  fresh?: boolean;
};

function exportPath(clusterId: string, options?: ArchitectureExportOptions): string {
  const qs = new URLSearchParams();
  if (options?.demo) qs.set("demo", "1");
  if (options?.ai) qs.set("ai", "1");
  if (options?.fresh) qs.set("fresh", "1");
  const q = qs.toString();
  return clusterPath(clusterId, `/architecture-export${q ? `?${q}` : ""}`);
}

function filenameFromDisposition(header: string | null, fallback: string): string {
  if (!header) return fallback;
  const match = /filename\*?=(?:UTF-8''|")?([^";]+)/i.exec(header);
  if (!match?.[1]) return fallback;
  return decodeURIComponent(match[1].replace(/"/g, "").trim()) || fallback;
}

/** One-click download of the architecture zip for a cluster (or demo). */
export async function downloadArchitectureExport(
  clusterId: string | null | undefined,
  options?: ArchitectureExportOptions,
): Promise<void> {
  const demo = Boolean(options?.demo) || !clusterId;
  // Prefer cluster-scoped ?demo=1 when a system is selected (same auth as other cluster APIs).
  const path =
    demo && !clusterId
      ? "/api/v1/architecture-export/demo"
      : exportPath(clusterId!, { ...options, demo: demo || options?.demo });

  // Always GET — POST would require write RBAC; ai=1 is a query flag on the same handler.
  const response = await fetch(path, { method: "GET", credentials: "include" });
  if (!response.ok) {
    let detail: string;
    try {
      const body = await response.json();
      detail = body?.error?.message || body?.message || "";
    } catch {
      detail = (await response.text().catch(() => "")).trim();
    }
    if (response.status === 404) {
      throw new Error(
        detail ||
          "Architecture export endpoint not found (404). Rebuild/restart the Consol API (e.g. make reload-api) so architecture-export routes are loaded.",
      );
    }
    throw new Error(detail || `Architecture export failed (${response.status})`);
  }

  const blob = await response.blob();
  const fallback = `nats-consol-architecture-${sanitizeFilenamePart(demo ? "demo" : clusterId!)}.zip`;
  const name = filenameFromDisposition(response.headers.get("Content-Disposition"), fallback);
  downloadBlob(name, blob);
}

export const ARCHITECTURE_GENERATOR_HREF = "/docs/architecture-generator";

export const ARCHITECTURE_EXPORT_FORMATS = [
  "C4 (PlantUML + Mermaid)",
  "Mermaid",
  "PlantUML",
  "Excalidraw",
  "Draw.io",
  "Markdown docs",
  "ADRs",
] as const;
