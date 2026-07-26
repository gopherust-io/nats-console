import { useEffect } from "react";
import { useToast } from "../../lib/toast";

/** Sticky panel error that also pops a toast so users see failures without scrolling. */
export default function PanelError({ message }: { message?: string }) {
  const toast = useToast();

  useEffect(() => {
    if (message) toast.error(message);
    // toast.error is stable from provider memo
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only re-toast when message changes
  }, [message]);

  if (!message) return null;

  return (
    <div className="stream-config-panel__error" role="alert">
      {message}
    </div>
  );
}
