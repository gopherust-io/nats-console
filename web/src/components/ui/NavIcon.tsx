export type NavIconName =
  | "dashboard"
  | "clusters"
  | "topology"
  | "supercluster"
  | "streams"
  | "kv"
  | "objects"
  | "audit"
  | "users"
  | "profiling";

const iconPaths: Record<NavIconName, string> = {
  dashboard:
    "M3 10.5L12 3l9 7.5V20a1 1 0 01-1 1h-5v-6H9v6H4a1 1 0 01-1-1v-9.5z",
  clusters:
    "M4 6a2 2 0 012-2h3v4H4V6zm0 6v-4h5v4H4zm7-8h3a2 2 0 012 2v2h-5V4zm0 6h5v4h-3a2 2 0 01-2-2v-2zm7-4a2 2 0 012 2v2h-5V6h3zM4 18v-2h5v4H6a2 2 0 01-2-2zm13 0a2 2 0 01-2 2h-3v-4h5v2z",
  topology:
    "M6 4h4v4H6V4zm8 0h4v4h-4V4zM6 12h4v4H6v-4zm8 0h4v4h-4v-4zM10 8v8M8 10h8",
  supercluster:
    "M4 12h6M14 12h6M12 4v16M7 7l5 5M17 7l-5 5M7 17l5-5M17 17l-5-5",
  streams: "M4 6h16M4 12h10M4 18h14",
  kv: "M5 5h14v14H5V5zm4 4h6v6H9V9z",
  objects:
    "M12 3l8 4.5v9L12 21l-8-4.5v-9L12 3zm0 2.2L6.5 8.5 12 11.8l5.5-3.3L12 5.2z",
  audit: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2",
  users:
    "M16 18v-1a4 4 0 00-4-4H8a4 4 0 00-4 4v1M12 11a4 4 0 100-8 4 4 0 000 8z",
  profiling:
    "M4 19V5M4 19h16M8 17V9M12 17V7M16 17v-5",
};

type NavIconProps = {
  name: NavIconName;
};

export default function NavIcon({ name }: NavIconProps) {
  return (
    <svg
      className="nav-icon"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.75}
      aria-hidden
    >
      <path d={iconPaths[name]} strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
