import { useEffect, useState } from "react";
import { api } from "../api";

export default function MatterSelect({
  matterId,
  documentId,
  onChange,
}: {
  matterId?: string;
  documentId: string;
  onChange: (matterId: string) => void;
}) {
  const [matters, setMatters] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .listMatters()
      .then((r) => setMatters(r.matters))
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load matters"));
  }, []);

  async function change(next: string) {
    setError(null);
    try {
      await api.setDocumentMatter(documentId, next);
      onChange(next);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to set matter");
    }
  }

  return (
    <div className="mb-4 text-sm">
      <label className="mr-2 font-medium">
        Matter{" "}
        <select
          className="rounded-md border border-rule bg-card px-2 py-1 font-normal"
          value={matterId ?? ""}
          onChange={(e) => change(e.target.value)}
        >
          <option value="">No matter</option>
          {matters.map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
      </label>
      {error && <span className="text-red-600">{error}</span>}
    </div>
  );
}
