import { useRef, type ReactNode } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import EmptyState from "./ui/EmptyState";

const DEFAULT_ROW_HEIGHT = 72;
const DEFAULT_MAX_HEIGHT = 560;

type VirtualCatalogListProps<T> = {
  items: T[];
  getKey: (item: T) => string;
  empty: string;
  rowHeight?: number;
  maxHeight?: number;
  className?: string;
  renderItem: (item: T, active: boolean) => ReactNode;
  isActive: (item: T) => boolean;
};

/** Virtualized sidebar list for Event Catalog / Wikipedia entry panes. */
export default function VirtualCatalogList<T>({
  items,
  getKey,
  empty,
  rowHeight = DEFAULT_ROW_HEIGHT,
  maxHeight = DEFAULT_MAX_HEIGHT,
  className = "nc-catalog-entries nc-catalog-entries--virtual",
  renderItem,
  isActive,
}: VirtualCatalogListProps<T>) {
  const parentRef = useRef<HTMLDivElement>(null);
  const virtualizer = useVirtualizer({
    count: items.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => rowHeight,
    overscan: 10,
  });

  if (items.length === 0) {
    return <EmptyState title={empty} />;
  }

  return (
    <div
      ref={parentRef}
      className={className}
      role="list"
      style={{ maxHeight, overflowY: "auto", position: "relative" }}
    >
      <div style={{ height: virtualizer.getTotalSize(), width: "100%", position: "relative" }}>
        {virtualizer.getVirtualItems().map((row) => {
          const item = items[row.index];
          return (
            <div
              key={getKey(item)}
              role="listitem"
              style={{
                position: "absolute",
                top: 0,
                left: 0,
                width: "100%",
                height: `${row.size}px`,
                transform: `translateY(${row.start}px)`,
              }}
            >
              {renderItem(item, isActive(item))}
            </div>
          );
        })}
      </div>
    </div>
  );
}
