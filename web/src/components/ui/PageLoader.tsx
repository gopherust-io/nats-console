export default function PageLoader() {
  return (
    <div className="page-loader" role="status" aria-label="Loading">
      <div className="page-loader__bar" />
      <div className="page-loader__grid">
        <div className="skeleton skeleton--title" />
        <div className="skeleton skeleton--card" />
        <div className="skeleton skeleton--card" />
        <div className="skeleton skeleton--card" />
        <div className="skeleton skeleton--table" />
      </div>
    </div>
  );
}
