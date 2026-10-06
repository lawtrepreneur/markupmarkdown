import { useEffect, useState } from "react";

export interface ModelOption {
  id: string;
  provider: string;
  external: boolean;
}

interface Props {
  value: string;
  onChange: (id: string) => void;
  disabled?: boolean;
}

export default function ModelSelector({ value, onChange, disabled }: Props) {
  const [models, setModels] = useState<ModelOption[]>([]);
  const [err, setErr] = useState(false);

  useEffect(() => {
    let live = true;
    fetch("/api/models", { credentials: "include" })
      .then((r) =>
        r.ok ? r.json() : Promise.reject(new Error(String(r.status))),
      )
      .then((b: ModelOption[]) => live && setModels(b))
      .catch(() => live && setErr(true));
    return () => {
      live = false;
    };
  }, []);

  return (
    <>
      <select
        aria-label="Model"
        value={value}
        disabled={disabled || err}
        onChange={(e) => onChange(e.target.value)}
      >
        {err && <option value="">Models unavailable</option>}
        {models.map((m) => (
          <option key={m.id} value={m.id}>
            {m.id}
          </option>
        ))}
      </select>
      {err && <span role="alert">Failed to load models</span>}
    </>
  );
}
