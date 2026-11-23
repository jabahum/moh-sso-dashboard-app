import { Button, Dropdown, Stack } from "@carbon/react";

import "./client-components.scss";

type StatusFilter = "all" | "enabled" | "disabled";
type TypeFilter = "all" | "public" | "confidential";

interface Option {
  id: string;
  label: string;
}

interface ClientFiltersProps {
  onReset: () => void;
  hasActiveFilters?: boolean;
  status: StatusFilter;
  type: TypeFilter;
  statusOptions: readonly Option[];
  typeOptions: readonly Option[];
  onStatusChange: (v: StatusFilter) => void;
  onTypeChange: (v: TypeFilter) => void;
}

export function ClientFilters({
  onReset,
  hasActiveFilters = true,
  status,
  type,
  statusOptions,
  typeOptions,
  onStatusChange,
  onTypeChange,
}: ClientFiltersProps) {
  return (
    <Stack orientation="horizontal" gap={5} className="client-filters">
      {/* Status */}
      <Dropdown
        id="client-status-filter"
        titleText="Status"
        label="Status"
        items={[...statusOptions]}
        selectedItem={statusOptions.find((i) => i.id === status)}
        itemToString={(item) => item?.label ?? ""}
        className="client-filters__field--status"
        onChange={({ selectedItem }) => {
          onStatusChange(selectedItem?.id as StatusFilter);
        }}
      />

      {/* Type */}
      <Dropdown
        id="client-type-filter"
        titleText="Type"
        label="Type"
        items={[...typeOptions]}
        selectedItem={typeOptions.find((i) => i.id === type)}
        itemToString={(item) => item?.label ?? ""}
        className="client-filters__field--type"
        onChange={({ selectedItem }) => {
          onTypeChange(selectedItem?.id as TypeFilter);
        }}
      />
      <Button kind="ghost" size="md" disabled={!hasActiveFilters} onClick={onReset}>
        Clear filters
      </Button>
    </Stack>
  );
}
