type PagerProps = {
  total: number;
  offset: number;
  limit: number;
  onPageChange: (offset: number) => void;
};

export default function Pager({ total, offset, limit, onPageChange }: PagerProps) {
  if (total <= limit) {
    return null;
  }

  const page = Math.floor(offset / limit) + 1;
  const pages = Math.max(1, Math.ceil(total / limit));

  return (
    <div className="pager">
      <button
        type="button"
        className="btn btn--secondary btn--small"
        disabled={offset <= 0}
        onClick={() => onPageChange(Math.max(0, offset - limit))}
      >
        Previous
      </button>
      <span className="muted">
        Page {page} of {pages} · {total} total
      </span>
      <button
        type="button"
        className="btn btn--secondary btn--small"
        disabled={offset + limit >= total}
        onClick={() => onPageChange(offset + limit)}
      >
        Next
      </button>
    </div>
  );
}

export { DEFAULT_PAGE_SIZE, pageQuery } from "../lib/constants";
