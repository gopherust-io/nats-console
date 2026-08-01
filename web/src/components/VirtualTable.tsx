import { useMemo, useRef, type CSSProperties, type ReactNode } from "react";
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
  /** Extra height reserved under each row when renderDetail returns content. */
  detailHeight?: number;
  maxHeight?: number;
  /** Horizontal overflow. When auto, the whole table (header + body) scrolls together. */
  overflowX?: "auto" | "hidden";
  /** Optional min-width on header/rows so wide grids can scroll instead of crushing. */
  minWidth?: number | string;
  empty?: string;
  getKey: (item: T, index: number) => string;
  renderCell: (item: T, columnId: string, index: number) => ReactNode;
  getRowClassName?: (item: T, index: number) => string | undefined;
  /** Optional detail band under the primary row (e.g. subscription subjects). */
  renderDetail?: (item: T, index: number) => ReactNode | null;
  /** Whether the detail band is open for this row (avoids rendering in estimateSize). */
  isDetailOpen?: (item: T, index: number) => boolean;
};

function buildGridTemplate(columns: VirtualTableColumn[]) {
  return columns.map((column) => column.width ?? "minmax(0, 1fr)").join(" ");
}

export default function VirtualTable<T>({
  columns,
  items,
  rowHeight = 52,
  detailHeight = 0,
  maxHeight = 560,
  overflowX = "hidden",
  minWidth,
  empty = "Nothing here yet",
  getKey,
  renderCell,
  getRowClassName,
  renderDetail,
  isDetailOpen,
}: VirtualTableProps<T>) {
  const parentRef = useRef<HTMLDivElement>(null);
  const gridTemplateColumns = useMemo(() => buildGridTemplate(columns), [columns]);
  const rowMinWidthStyle = useMemo<CSSProperties | undefined>(() => {
    if (minWidth == null) return undefined;
    return { minWidth: typeof minWidth === "number" ? `${minWidth}px` : minWidth };
  }, [minWidth]);

  const estimateSize = (index: number) => {
    if (!renderDetail || detailHeight <= 0) return rowHeight;
    const open = isDetailOpen ? isDetailOpen(items[index], index) : true;
    return open ? rowHeight + detailHeight : rowHeight;
  };

  const virtualizer = useVirtualizer({
    count: items.length,
    getScrollElement: () => parentRef.current,
    estimateSize,
    overscan: 8,
  });

  if (items.length === 0) {
    return <EmptyState title={empty} />;
  }

  const contentHeight = virtualizer.getTotalSize();
  const scrollHeight = Math.min(contentHeight, maxHeight);
  const scrollHorizontally = overflowX === "auto";

  return (
    <div
      className="virtual-table"
      style={{
        ["--vt-columns" as string]: gridTemplateColumns,
        overflowX: scrollHorizontally ? "auto" : undefined,
      }}
    >
      <div className="virtual-table__header" role="row" style={rowMinWidthStyle}>
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
        style={{
          height: scrollHeight,
          maxHeight: scrollHeight,
          overflowX: "hidden",
          overflowY: contentHeight > maxHeight ? "auto" : "hidden",
        }}
      >
        <div
          className="virtual-table__viewport"
          style={{ height: contentHeight, ...rowMinWidthStyle }}
        >
          {virtualizer.getVirtualItems().map((virtualRow) => {
            const item = items[virtualRow.index];
            const rowClass = getRowClassName?.(item, virtualRow.index);
            const open = isDetailOpen ? isDetailOpen(item, virtualRow.index) : Boolean(renderDetail);
            const detail = open && renderDetail ? renderDetail(item, virtualRow.index) : null;
            const hasDetail = detail != null && detailHeight > 0;
            return (
              <div
                key={getKey(item, virtualRow.index)}
                className={["virtual-table__block", rowClass].filter(Boolean).join(" ")}
                style={{
                  height: `${virtualRow.size}px`,
                  transform: `translateY(${virtualRow.start}px)`,
                  ...rowMinWidthStyle,
                }}
                role="rowgroup"
              >
                <div
                  className="virtual-table__row"
                  style={{ height: `${rowHeight}px` }}
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
                {hasDetail ? (
                  <div className="virtual-table__detail" style={{ height: `${detailHeight}px` }}>
                    {detail}
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
