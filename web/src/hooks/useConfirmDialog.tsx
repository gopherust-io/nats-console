import { useCallback, useState, type ReactNode } from "react";
import ConfirmDialog from "../components/ui/ConfirmDialog";

export type ConfirmRequest = {
  title: string;
  description: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  tone?: "danger" | "default";
  action: () => void | Promise<void>;
};

/**
 * Imperative confirm helper that renders the shared themed ConfirmDialog.
 * Prefer this over window.confirm / confirm().
 */
export function useConfirmDialog() {
  const [request, setRequest] = useState<ConfirmRequest | null>(null);
  const [busy, setBusy] = useState(false);

  const askConfirm = useCallback((req: ConfirmRequest) => {
    setRequest(req);
  }, []);

  const onCancel = useCallback(() => {
    if (!busy) setRequest(null);
  }, [busy]);

  const onConfirm = useCallback(async () => {
    if (!request) return;
    setBusy(true);
    try {
      await request.action();
    } finally {
      setBusy(false);
      setRequest(null);
    }
  }, [request]);

  const confirmDialog = (
    <ConfirmDialog
      open={Boolean(request)}
      title={request?.title ?? ""}
      description={request?.description ?? null}
      confirmLabel={request?.confirmLabel}
      cancelLabel={request?.cancelLabel}
      tone={request?.tone ?? "danger"}
      busy={busy}
      onCancel={onCancel}
      onConfirm={() => void onConfirm()}
    />
  );

  return { askConfirm, confirmDialog };
}
