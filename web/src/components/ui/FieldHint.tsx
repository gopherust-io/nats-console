import type { ReactNode } from "react";

type FieldHintProps = {
  id?: string;
  children: ReactNode;
};

export default function FieldHint({ id, children }: FieldHintProps) {
  if (children === null || children === undefined || children === false || children === "") {
    return null;
  }
  return (
    <p id={id} className="field-hint text-muted">
      {children}
    </p>
  );
}
