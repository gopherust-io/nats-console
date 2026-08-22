type ValidationHintProps = {
  id?: string;
  message: string;
};

export default function ValidationHint({ id, message }: ValidationHintProps) {
  if (!message) return null;
  return (
    <p id={id} className="validation-hint" role="alert">
      {message}
    </p>
  );
}
