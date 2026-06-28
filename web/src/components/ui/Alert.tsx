type AlertProps = {
  variant?: "error" | "success" | "info";
  children: string;
};

export default function Alert({ variant = "error", children }: AlertProps) {
  if (!children) return null;
  return (
    <div className={`alert alert--${variant}`} role="alert">
      {children}
    </div>
  );
}
