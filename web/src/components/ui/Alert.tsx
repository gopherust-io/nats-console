import type { ReactNode } from "react";

type AlertProps = {
  variant?: "error" | "success" | "info";
  children: ReactNode;
};

export default function Alert({ variant = "error", children }: AlertProps) {
  if (children === null || children === undefined || children === false || children === "") {
    return null;
  }
  return (
    <div className={`alert alert--${variant}`} role="alert">
      {children}
    </div>
  );
}
