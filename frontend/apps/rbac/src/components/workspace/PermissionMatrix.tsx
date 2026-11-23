import { useCallback, useId, useMemo, useState } from "react";
import { Button, Checkbox, InlineLoading, InlineNotification, Search, Tag } from "@carbon/react";
import { Reset, Save } from "@carbon/react/icons";
import { useToast } from "@moh-sso/ui";

import type { RbacPermission } from "../../types";
import { ActionError, useUnsavedChanges } from "./feedback";
import {
  getApiErrorMessage,
  permissionLabel,
  pluralize,
  type PermissionCategory,
} from "./workspace-helpers";

type Failure = { key: string; change: "add" | "remove"; message: string };

export type StagedPermissionEditorProps = {
  categories: PermissionCategory[];
  /** Permission keys currently saved for the subject. */
  savedKeys: ReadonlySet<string>;
  /** Names the subject in summaries and warnings, e.g. `role "Viewer"`. */
  subjectLabel: string;
  /** True when the person may change permissions. */
  canEdit: boolean;
  /** Shown when `canEdit` is false. */
  readOnlyReason?: string;
  onAssign: (permissionKey: string) => Promise<unknown>;
  onRemove: (permissionKey: string) => Promise<unknown>;
};

function changeSummary(added: number, removed: number) {
  const parts: string[] = [];
  if (added) parts.push(`${added} added`);
  if (removed) parts.push(`${removed} removed`);
  return parts.join(", ");
}

/**
 * Permission checkboxes whose changes are staged locally, reviewed, and then
 * saved with one assign or remove call per staged difference. Mount it with a
 * `key` per subject so switching subjects starts from a clean slate.
 */
export function StagedPermissionEditor({
  categories,
  savedKeys,
  subjectLabel,
  canEdit,
  readOnlyReason,
  onAssign,
  onRemove,
}: StagedPermissionEditorProps) {
  const toast = useToast();
  const matrixId = useId();
  const [search, setSearch] = useState("");
  const [stagedAdds, setStagedAdds] = useState<Set<string>>(() => new Set());
  const [stagedRemovals, setStagedRemovals] = useState<Set<string>>(() => new Set());
  const [progress, setProgress] = useState<{ done: number; total: number } | null>(null);
  const [failures, setFailures] = useState<Failure[]>([]);
  const saving = progress !== null;

  // Only differences from what is saved count, even after a refetch.
  const adds = useMemo(
    () => [...stagedAdds].filter((key) => !savedKeys.has(key)).sort(),
    [savedKeys, stagedAdds],
  );
  const removals = useMemo(
    () => [...stagedRemovals].filter((key) => savedKeys.has(key)).sort(),
    [savedKeys, stagedRemovals],
  );
  const dirty = adds.length + removals.length > 0;

  const isChecked = (key: string) =>
    (savedKeys.has(key) && !stagedRemovals.has(key)) || stagedAdds.has(key);

  const stage = (keys: string[], checked: boolean) => {
    setStagedAdds((current) => {
      const next = new Set(current);
      for (const key of keys) {
        if (checked && !savedKeys.has(key)) next.add(key);
        else next.delete(key);
      }
      return next;
    });
    setStagedRemovals((current) => {
      const next = new Set(current);
      for (const key of keys) {
        if (!checked && savedKeys.has(key)) next.add(key);
        else next.delete(key);
      }
      return next;
    });
  };

  const discard = useCallback(() => {
    setStagedAdds(new Set());
    setStagedRemovals(new Set());
    setFailures([]);
  }, []);

  useUnsavedChanges(dirty, `Permission changes for ${subjectLabel}`, discard);

  const save = async () => {
    const total = adds.length + removals.length;
    if (!total || saving) return;
    const failedAdds = new Set<string>();
    const failedRemovals = new Set<string>();
    const failed: Failure[] = [];
    let done = 0;
    setFailures([]);
    setProgress({ done, total });
    for (const key of adds) {
      try {
        await onAssign(key);
      } catch (error) {
        failedAdds.add(key);
        failed.push({ key, change: "add", message: getApiErrorMessage(error, "Not saved.") });
      }
      setProgress({ done: ++done, total });
    }
    for (const key of removals) {
      try {
        await onRemove(key);
      } catch (error) {
        failedRemovals.add(key);
        failed.push({ key, change: "remove", message: getApiErrorMessage(error, "Not saved.") });
      }
      setProgress({ done: ++done, total });
    }
    // Failed changes stay staged so they can be retried or discarded.
    setStagedAdds(failedAdds);
    setStagedRemovals(failedRemovals);
    setFailures(failed);
    setProgress(null);
    const savedAdds = adds.length - failedAdds.size;
    const savedRemovals = removals.length - failedRemovals.size;
    if (!failed.length) {
      toast.success(
        "Permissions saved",
        `${subjectLabel}: ${changeSummary(savedAdds, savedRemovals)}.`,
      );
    } else if (savedAdds + savedRemovals > 0) {
      toast.warning(
        "Some permission changes were not saved",
        `${failed.length} of ${total} changes failed.`,
      );
    }
  };

  const query = search.trim().toLowerCase();
  const visibleCategories = categories
    .map((category) => ({
      ...category,
      permissions: category.permissions.filter((permission) =>
        `${category.label} ${category.id} ${permission.key} ${permissionLabel(permission)}`
          .toLowerCase()
          .includes(query),
      ),
    }))
    .filter((category) => category.permissions.length > 0);

  const renderPermission = (permission: RbacPermission, categoryId: string) => {
    const checked = isChecked(permission.key);
    const added = checked && !savedKeys.has(permission.key);
    const removed = !checked && savedKeys.has(permission.key);
    return (
      <div
        className={`rbac-permission${added ? " rbac-permission--added" : ""}${removed ? " rbac-permission--removed" : ""}`}
        key={permission.key}
      >
        <Checkbox
          id={`${matrixId}-${categoryId}-${permission.key}`}
          labelText={
            <span className="rbac-permission__text">
              <span className="rbac-permission__label">{permissionLabel(permission)}</span>
              <code className="rbac-permission__key">{permission.key}</code>
            </span>
          }
          title={permission.description || undefined}
          checked={checked}
          disabled={!canEdit || saving}
          onChange={(_, { checked: next }) => stage([permission.key], next)}
        />
        {added && (
          <Tag size="sm" type="green">
            Added
          </Tag>
        )}
        {removed && (
          <Tag size="sm" type="red">
            Removed
          </Tag>
        )}
      </div>
    );
  };

  return (
    <div
      className={`rbac-permission-editor${dirty || saving || failures.length ? " rbac-permission-editor--staged" : ""}`}
    >
      {!canEdit && readOnlyReason && (
        <InlineNotification
          kind="info"
          lowContrast
          hideCloseButton
          title="Read only"
          subtitle={readOnlyReason}
        />
      )}
      <div className="rbac-permission-search">
        <Search
          size="md"
          labelText="Find permissions"
          placeholder="Find a permission or category"
          closeButtonLabelText="Clear permission search"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>
      {visibleCategories.length === 0 && (
        <p className="rbac-empty">No permissions match “{search.trim()}”.</p>
      )}
      <div className="rbac-permission-matrix">
        {visibleCategories.map((category) => {
          const keys = category.permissions.map((permission) => permission.key);
          const selected = keys.filter(isChecked).length;
          return (
            <fieldset className="rbac-permission-group" key={category.id}>
              <legend className="rbac-permission-group__legend">
                <Checkbox
                  id={`${matrixId}-${category.id}-all`}
                  labelText={
                    <span className="rbac-permission-group__title">
                      <span className="cds--visually-hidden">Select all in </span>
                      <span>{category.label}</span>
                      <small>
                        {selected} of {keys.length}
                        <span className="cds--visually-hidden"> selected</span>
                      </small>
                    </span>
                  }
                  checked={selected === keys.length}
                  indeterminate={selected > 0 && selected < keys.length}
                  disabled={!canEdit || saving}
                  onChange={() => stage(keys, selected !== keys.length)}
                />
              </legend>
              {category.permissions.map((permission) => renderPermission(permission, category.id))}
            </fieldset>
          );
        })}
      </div>

      {(dirty || saving || failures.length > 0) && (
        <div className="rbac-staged-bar" role="region" aria-label="Unsaved permission changes">
          <div className="rbac-staged-bar__summary" aria-live="polite">
            {saving ? (
              <InlineLoading
                description={`Saving ${progress.done} of ${pluralize(progress.total, "change")}…`}
              />
            ) : dirty ? (
              <>
                <strong>{changeSummary(adds.length, removals.length)}</strong>
                <span>Not saved yet. Review the highlighted permissions, then save.</span>
              </>
            ) : (
              <strong>Nothing left to save.</strong>
            )}
          </div>
          <div className="rbac-staged-bar__actions">
            <Button
              kind="secondary"
              size="md"
              renderIcon={Reset}
              disabled={saving}
              onClick={discard}
            >
              Discard
            </Button>
            <Button
              kind="primary"
              size="md"
              renderIcon={Save}
              disabled={!dirty || saving || !canEdit}
              onClick={() => void save()}
            >
              Save changes
            </Button>
          </div>
          <ActionError
            title={`${pluralize(failures.length, "change")} could not be saved`}
            message={
              failures.length
                ? "They are still staged below, so you can retry or discard them."
                : null
            }
            onClose={() => setFailures([])}
          >
            <ul className="rbac-staged-bar__failures">
              {failures.map((failure) => (
                <li key={`${failure.change}:${failure.key}`}>
                  {failure.change === "add" ? "Add" : "Remove"} <code>{failure.key}</code>:{" "}
                  {failure.message}
                </li>
              ))}
            </ul>
          </ActionError>
        </div>
      )}
    </div>
  );
}
