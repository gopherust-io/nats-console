import {
  forwardRef,
  useCallback,
  useImperativeHandle,
  useMemo,
  useRef,
  type TextareaHTMLAttributes,
  type UIEvent,
} from "react";

export type LinedTextareaProps = Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, "children"> & {
  /** 1-based line to highlight in the gutter when JSON (or other) validation fails. */
  errorLine?: number | null;
};

function countLines(value: string | number | readonly string[] | undefined): number {
  const text = typeof value === "string" ? value : value == null ? "" : String(value);
  return Math.max(1, text.split("\n").length);
}

const LinedTextarea = forwardRef<HTMLTextAreaElement, LinedTextareaProps>(function LinedTextarea(
  { className, errorLine, onScroll, value, ...rest },
  ref,
) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const gutterRef = useRef<HTMLDivElement>(null);

  useImperativeHandle(ref, () => textareaRef.current as HTMLTextAreaElement);

  const lineCount = useMemo(() => countLines(value), [value]);
  const lines = useMemo(
    () => Array.from({ length: lineCount }, (_, index) => index + 1),
    [lineCount],
  );

  const syncGutter = useCallback(() => {
    const textarea = textareaRef.current;
    const gutter = gutterRef.current;
    if (!textarea || !gutter) return;
    gutter.scrollTop = textarea.scrollTop;
  }, []);

  const handleScroll = useCallback(
    (event: UIEvent<HTMLTextAreaElement>) => {
      syncGutter();
      onScroll?.(event);
    },
    [onScroll, syncGutter],
  );

  const hasInvalidClass = Boolean(className?.split(/\s+/).includes("input-invalid"));
  const invalid = Boolean(rest["aria-invalid"]) || hasInvalidClass;
  const inputClassName = ["lined-textarea__input", className]
    .filter(Boolean)
    .join(" ")
    .replace(/\binput-invalid\b/g, "")
    .replace(/\s+/g, " ")
    .trim();
  const wrapperClass = ["lined-textarea", invalid ? "lined-textarea--invalid input-invalid" : ""]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={wrapperClass}>
      <div ref={gutterRef} className="lined-textarea__gutter" aria-hidden="true">
        {lines.map((line) => (
          <span
            key={line}
            className={
              errorLine === line
                ? "lined-textarea__line lined-textarea__line--error"
                : "lined-textarea__line"
            }
          >
            {line}
          </span>
        ))}
      </div>
      <textarea
        {...rest}
        ref={textareaRef}
        className={inputClassName}
        value={value}
        onScroll={handleScroll}
      />
    </div>
  );
});

export default LinedTextarea;
