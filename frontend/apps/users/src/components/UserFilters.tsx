import { Button, Checkbox, Dropdown, Stack } from "@carbon/react";

import "./user-components.scss";

interface FilterOption {
  id: string;
  label: string;
}

const STATUS_OPTIONS: FilterOption[] = [
  { id: "all", label: "All" },
  { id: "active", label: "Active" },
  { id: "disabled", label: "Disabled" },
];

interface UserFiltersProps {
  onReset: () => void;
  status: string;
  roles: string[];
  selectedRole: string;
  neverLoggedIn: boolean;
  hasActiveFilters?: boolean;
  onStatusChange: (v: string) => void;
  onRoleChange: (v: string) => void;
  onToggleNeverLoggedIn: () => void;
}

export function UserFilters({
  onReset,
  status,
  roles,
  selectedRole,
  neverLoggedIn,
  hasActiveFilters = true,
  onStatusChange,
  onRoleChange,
  onToggleNeverLoggedIn,
}: UserFiltersProps) {
  // Role names are identifiers and are shown as-is; only the "all" sentinel gets a label.
  const roleOptions: FilterOption[] = roles.map((role) => ({
    id: role,
    label: role === "all" ? "All" : role,
  }));

  return (
    <Stack orientation="horizontal" gap={5} className="user-filters">
      {/* Status */}
      <Dropdown
        id="user-status-filter"
        label="Status"
        titleText="Status"
        items={STATUS_OPTIONS}
        selectedItem={STATUS_OPTIONS.find((option) => option.id === status) ?? null}
        itemToString={(item) => item?.label ?? ""}
        className="user-filters__field--status"
        onChange={({ selectedItem }) => {
          if (selectedItem) onStatusChange(selectedItem.id);
        }}
      />

      {/* Role */}
      <Dropdown
        id="user-role-filter"
        label="Role"
        titleText="Role"
        items={roleOptions}
        selectedItem={roleOptions.find((option) => option.id === selectedRole) ?? null}
        itemToString={(item) => item?.label ?? ""}
        className="user-filters__field--role"
        onChange={({ selectedItem }) => {
          if (selectedItem) onRoleChange(selectedItem.id);
        }}
      />

      {/* Never logged in */}
      <div className="user-filters__field--checkbox">
        <Checkbox
          id="user-never-logged-in-filter"
          labelText="Never logged in"
          checked={neverLoggedIn}
          onChange={() => onToggleNeverLoggedIn()}
        />
      </div>

      <Button kind="ghost" size="md" disabled={!hasActiveFilters} onClick={onReset}>
        Clear filters
      </Button>
    </Stack>
  );
}
