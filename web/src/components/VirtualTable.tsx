import { useMemo, useRef, type ReactNode } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import EmptyState from "./ui/EmptyState";

export type VirtualTableColumn = {
  id: string;
  header: ReactNode;
  /** CSS grid track size, e.g. minmax(140px, 1.5fr) or 100px */
  width?: string;
  align?: "left" | "right" | "center";
  cellClassName?: string;
};

type VirtualTableProps<T> = {
  columns: VirtualTableColumn[];
  items: T[];
  rowHeight?: number;
  maxHeight?: number;
  empty?: ReactNode;
  getKey: (item: T, index: number) => string;
  renderCell: (item: T, columnId: string, index: number) => ReactNode;
};

function buildGridTemplate(columns: VirtualTableColumn[]) {
  return columns.map((column) => column.width ?? "minmax(0, 1fr)").join(" ");
}

export default function VirtualTable<T>({
  columns,
  items,
  rowHeight = 52,
  maxHeight = 560,
  empty,
  getKey,
  renderCell,
}: VirtualTableProps<T>) {
  const parentRef = useRef<HTMLDivElement>(null);
  const gridTemplateColumns = useMemo(() => buildGridTemplate(columns), [columns]);

  const virtualizer = useVirtualizer({
    count: items.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => rowHeight,
    overscan: 8,
  });

  if (items.length === 0) {
    const title = typeof empty === "string" ? empty : "Nothing here yet";
    return <EmptyState title={title} />;
  }

  const contentHeight = items.length * rowHeight;
  const scrollHeight = Math.min(contentHeight, maxHeight);

  return (
    <div className="virtual-table" style={{ ["--vt-columns" as string]: gridTemplateColumns }}>
      <div className="virtual-table__header" role="row">
        {columns.map((column) => (
          <div
            key={column.id}
            className={`virtual-table__th${column.align ? ` virtual-table__cell--${column.align}` : ""}`}
            role="columnheader"
          >
            {column.header}
          </div>
        ))}
      </div>

      <div
        ref={parentRef}
        className="virtual-table__body"
        style={{ maxHeight: scrollHeight, overflowY: contentHeight > maxHeight ? "auto" : "visible" }}
      >
        <div className="virtual-table__viewport" style={{ height: virtualizer.getTotalSize() }}>
          {virtualizer.getVirtualItems().map((virtualRow) => {
            const item = items[virtualRow.index];
            return (
              <div
                key={getKey(item, virtualRow.index)}
                className="virtual-table__row"
                style={{
                  height: `${virtualRow.size}px`,
                  transform: `translateY(${virtualRow.start}px)`,
                }}
                role="row"
              >
                {columns.map((column) => (
                  <div
                    key={column.id}
                    className={[
                      "virtual-table__td",
                      column.align ? `virtual-table__cell--${column.align}` : "",
                      column.cellClassName ?? "",
                    ]
                      .filter(Boolean)
                      .join(" ")}
                    role="cell"
                  >
                    {renderCell(item, column.id, virtualRow.index)}
                  </div>
                ))}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
