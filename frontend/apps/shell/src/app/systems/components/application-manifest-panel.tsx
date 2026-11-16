import { Button, Checkbox, InlineNotification, Stack, TextArea } from "@carbon/react";
import { useState } from "react";
import {
  useImportApplicationManifestMutation,
  usePreviewApplicationManifestMutation,
  type ApplicationManifestPreview,
} from "../api/application-manifests.api";
import "./application-manifest-panel.scss";

function errorMessage(error: unknown): string {
  if (typeof error === "object" && error !== null && "data" in error) {
    const data = error.data;
    if (typeof data === "object" && data !== null && "error" in data) {
      const detail = data.error;
      if (
        typeof detail === "object" &&
        detail !== null &&
        "message" in detail &&
        typeof detail.message === "string"
      ) {
        return detail.message;
      }
    }
  }
  return error instanceof Error
    ? error.message
    : "The manifest request failed. Check your connection and permissions, then retry.";
}

export function ApplicationManifestPanel() {
  const [document, setDocument] = useState("");
  const [preview, setPreview] = useState<ApplicationManifestPreview | null>(null);
  const [reviewed, setReviewed] = useState(false);
  const [error, setError] = useState("");
  const [imported, setImported] = useState(false);
  const [reading, setReading] = useState(false);
  const [previewManifest, { isLoading: previewing }] = usePreviewApplicationManifestMutation();
  const [importManifest, { isLoading: importing }] = useImportApplicationManifestMutation();
  const busy = previewing || importing || reading;

  const changeDocument = (value: string) => {
    setDocument(value);
    setPreview(null);
    setReviewed(false);
    setImported(false);
    setError("");
  };

  return (
    <Stack gap={5} className="application-manifest-panel">
      <p>Upload or paste one application manifest, then review its registry and access changes.</p>
      <label htmlFor="application-manifest-file">YAML or JSON manifest</label>
      <input
        id="application-manifest-file"
        type="file"
        accept=".yaml,.yml,.json"
        disabled={busy}
        onChange={async (event) => {
          const file = event.target.files?.[0];
          if (!file) return;
          changeDocument("");
          if (file.size > 128 * 1024) {
            changeDocument("");
            setError("The manifest must be at most 128 KiB.");
            return;
          }
          try {
            setReading(true);
            changeDocument(await file.text());
          } catch (failure) {
            setError(errorMessage(failure));
          } finally {
            setReading(false);
          }
        }}
      />
      <TextArea
        id="application-manifest-document"
        labelText="Manifest document"
        rows={14}
        value={document}
        disabled={busy}
        onChange={(event) => changeDocument(event.target.value)}
      />
      {error && (
        <InlineNotification
          kind="error"
          title="Manifest request failed"
          subtitle={error}
          hideCloseButton
        />
      )}
      <Button
        disabled={busy || !document.trim()}
        onClick={async () => {
          setError("");
          setPreview(null);
          setReviewed(false);
          setImported(false);
          try {
            setPreview(await previewManifest(document).unwrap());
          } catch (failure) {
            setError(errorMessage(failure));
          }
        }}
      >
        {previewing ? "Validating…" : "Validate and preview"}
      </Button>
      {preview && (
        <>
          <h3>
            Proposed {preview.operation === "noop" ? "unchanged registration" : preview.operation}
          </h3>
          <p>
            {preview.manifest.metadata.name} · {preview.manifest.metadata.version} ·{" "}
            {preview.manifest.spec.environment}
          </p>
          <p>
            Access requires any of these {preview.manifest.metadata.id} client roles:{" "}
            {preview.accessRoles.join(", ")}.
          </p>
          {preview.rolesToCreate.length > 0 && (
            <p>Local role definitions to create: {preview.rolesToCreate.join(", ")}.</p>
          )}
          {preview.warnings.map((warning) => (
            <p key={warning}>{warning}</p>
          ))}
          <details>
            <summary>Current registration</summary>
            <pre>{JSON.stringify(preview.before ?? null, null, 2)}</pre>
          </details>
          <details open>
            <summary>Proposed registration</summary>
            <pre>{JSON.stringify(preview.after, null, 2)}</pre>
          </details>
          <Checkbox
            id="application-manifest-reviewed"
            labelText="I reviewed the application, environment, URLs and access roles."
            checked={reviewed}
            disabled={busy || imported}
            onChange={(_, { checked }) => setReviewed(checked)}
          />
          <Button
            disabled={busy || !reviewed || imported}
            onClick={async () => {
              setError("");
              try {
                await importManifest({ document, reviewToken: preview.reviewToken }).unwrap();
                setImported(true);
              } catch (failure) {
                setError(errorMessage(failure));
                setPreview(null);
                setReviewed(false);
              }
            }}
          >
            {importing ? "Importing…" : "Import reviewed manifest"}
          </Button>
        </>
      )}
      {imported && (
        <InlineNotification
          kind="success"
          title="Manifest imported"
          subtitle="The registry is up to date. Review the application's lifecycle status before activation."
          hideCloseButton
        />
      )}
    </Stack>
  );
}
