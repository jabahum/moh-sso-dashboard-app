import { useEffect, useMemo, useState } from "react";
import { Button, FilterableMultiSelect } from "@carbon/react";
import { useGetSchedulerRecipientOptionsQuery } from "../api";
import type { RecipientOption } from "../types";
import { useSchedulerQueryToasts } from "../hooks/useSchedulerQueryToasts";

type RecipientSelectProps = {
  kind: "email" | "group" | "portal";
  label: string;
  selected: RecipientOption[];
  onChange: (items: RecipientOption[]) => void;
  disabled?: boolean;
};

export function RecipientSelect({
  kind,
  label,
  selected,
  onChange,
  disabled,
}: RecipientSelectProps) {
  const [input, setInput] = useState("");
  const [search, setSearch] = useState("");
  useEffect(() => {
    const timeout = setTimeout(() => setSearch(input.trim()), 250);
    return () => clearTimeout(timeout);
  }, [input]);
  const directory = useGetSchedulerRecipientOptionsQuery({
    kind,
    search: search || undefined,
    selected: selected.length ? JSON.stringify(selected.map((item) => item.value)) : undefined,
  });
  const [knownOptions, setKnownOptions] = useState<Record<string, RecipientOption>>({});
  useEffect(() => {
    if (!directory.currentData) return;
    setKnownOptions((current) => {
      if (directory.currentData?.every((item) => current[item.id]?.label === item.label))
        return current;
      return {
        ...current,
        ...Object.fromEntries((directory.currentData ?? []).map((item) => [item.id, item])),
      };
    });
  }, [directory.currentData]);
  const items = useMemo(() => {
    // Keep saved recipients and selections visible even when searching another page.
    const merged = new Map(selected.map((item) => [item.id, knownOptions[item.id] ?? item]));
    for (const item of directory.currentData ?? []) merged.set(item.id, item);
    return [...merged.values()];
  }, [selected, directory.currentData, knownOptions]);
  const selectedItems = items.filter((item) =>
    selected.some((selection) => selection.id === item.id),
  );
  useSchedulerQueryToasts([
    {
      id: `recipients-${kind}`,
      active: directory.isError,
      kind: "error",
      title: `Unable to load ${label.toLowerCase()}`,
      subtitle: "Retry the recipient lookup or try again later.",
    },
  ]);
  return (
    <div className="report-scheduler__recipient-select">
      <FilterableMultiSelect<RecipientOption>
        id={`report-recipients-${kind}`}
        titleText={label}
        placeholder={directory.isFetching ? "Loading options…" : "Type to search and select"}
        items={items}
        selectedItems={selectedItems}
        itemToString={(item) => item?.label ?? ""}
        selectionFeedback="fixed"
        autoAlign
        disabled={disabled}
        onInputValueChange={({ inputValue }) => setInput(inputValue ?? "")}
        onChange={({ selectedItems }) => onChange(selectedItems ?? [])}
        helperText={
          kind === "group"
            ? "Group members receive the report by email."
            : kind === "portal"
              ? "Selected users receive the report in their portal inbox."
              : "Select registered users to receive the report by email."
        }
      />
      {directory.isError && (
        <Button
          type="button"
          kind="ghost"
          size="sm"
          disabled={directory.isFetching}
          onClick={() => void directory.refetch()}
        >
          Retry lookup
        </Button>
      )}
      {!directory.isFetching && !directory.isError && directory.currentData?.length === 0 && (
        <p className="report-scheduler__recipient-hint">
          No matching recipients. Try another search.
        </p>
      )}
    </div>
  );
}
