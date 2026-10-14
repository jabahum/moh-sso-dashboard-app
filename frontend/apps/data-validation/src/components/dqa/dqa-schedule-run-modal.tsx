import {
  Button,
  ComposedModal,
  InlineNotification,
  ModalBody,
  ModalFooter,
  ModalHeader,
  MultiSelect,
  Stack,
  TextInput,
} from "@carbon/react";
import { useMemo, useState } from "react";

import type { DQARunFilter, DQARunPeriod, DQARunScope, DQATableMapping } from "../../dqa.types";

type Props = {
  table: DQATableMapping;
  isRunning: boolean;
  isScheduling: boolean;
  onRunNow: (scope: DQARunScope) => Promise<void> | void;
  onRunLater: (scope: DQARunScope, scheduledAt: Date) => Promise<void> | void;
  onClose: () => void;
};

const MONTHS = [
  "January", "February", "March", "April", "May", "June",
  "July", "August", "September", "October", "November", "December",
];

/** Last five years, newest first — periods are always in the past. */
function recentYears(): number[] {
  const thisYear = new Date().getFullYear();
  return Array.from({ length: 5 }, (_, i) => thisYear - i);
}

export function DQAScheduleRunModal({
  table,
  isRunning,
  isScheduling,
  onRunNow,
  onRunLater,
  onClose,
}: Props) {
  const [years, setYears] = useState<number[]>([]);
  const [months, setMonths] = useState<string[]>([]);
  const [filterValues, setFilterValues] = useState<Record<string, string>>({});
  // A native datetime input rather than Carbon's DatePicker: that pulls in
  // flatpickr, which is the only such dependency in this app.
  const [runAt, setRunAt] = useState("");
  const [error, setError] = useState<string | null>(null);

  const filterColumns = table.filter_columns ?? [];
  const canScopePeriod = Boolean(table.period_column);

  const periods = useMemo<DQARunPeriod[]>(() => {
    const result: DQARunPeriod[] = [];
    for (const year of years) {
      for (const month of months) {
        result.push({ year, month: MONTHS.indexOf(month) + 1 });
      }
    }
    return result;
  }, [years, months]);

  const buildScope = (): DQARunScope => {
    const filters: DQARunFilter[] = Object.entries(filterValues)
      .filter(([, value]) => value.trim() !== "")
      .map(([column, value]) => ({ column, value: value.trim() }));
    return { periods, filters };
  };

  const validate = (): boolean => {
    if (years.length > 0 && months.length === 0) {
      setError("Pick at least one month to go with the selected year(s).");
      return false;
    }
    if (months.length > 0 && years.length === 0) {
      setError("Pick at least one year to go with the selected month(s).");
      return false;
    }
    setError(null);
    return true;
  };

  const handleRunNow = async () => {
    if (!validate()) return;
    await onRunNow(buildScope());
  };

  const handleRunLater = async () => {
    if (!validate()) return;
    if (!runAt) {
      setError("Pick the date and time the run should execute.");
      return;
    }
    const scheduledAt = new Date(runAt);
    if (Number.isNaN(scheduledAt.getTime())) {
      setError("That date and time could not be read.");
      return;
    }
    if (scheduledAt.getTime() <= Date.now()) {
      setError("The execution time is in the past.");
      return;
    }
    await onRunLater(buildScope(), scheduledAt);
  };

  const busy = isRunning || isScheduling;

  return (
    <ComposedModal open size="lg" onClose={onClose}>
      <ModalHeader title={`Schedule a run for ${table.table_id}`} />
      <ModalBody hasScrollingContent>
        <Stack gap={5}>
          <p style={{ color: "var(--cds-text-secondary)" }}>
            Choose the data this scan should cover. Leave everything blank to scan the whole table.
          </p>

          {canScopePeriod ? (
            <Stack orientation="horizontal" gap={4}>
              <div style={{ flex: 1 }}>
                <MultiSelect
                  id="dqa-scope-years"
                  titleText="Years"
                  label={years.length ? `${years.length} selected` : "Any year"}
                  items={recentYears()}
                  itemToString={(item) => String(item)}
                  selectedItems={years}
                  onChange={({ selectedItems }) => setYears(selectedItems ?? [])}
                />
              </div>
              <div style={{ flex: 1 }}>
                <MultiSelect
                  id="dqa-scope-months"
                  titleText="Months"
                  label={months.length ? `${months.length} selected` : "Any month"}
                  items={MONTHS}
                  selectedItems={months}
                  onChange={({ selectedItems }) => setMonths(selectedItems ?? [])}
                />
              </div>
            </Stack>
          ) : (
            <InlineNotification
              kind="info"
              title="No period column configured"
              subtitle={`Set a period column on ${table.table_id} to scope runs by month.`}
              lowContrast
              hideCloseButton
            />
          )}

          {periods.length > 0 ? (
            <p style={{ fontSize: "0.75rem", color: "var(--cds-text-secondary)" }}>
              Scanning {periods.length} period{periods.length === 1 ? "" : "s"}.
            </p>
          ) : null}

          {filterColumns.length > 0 ? (
            <Stack gap={4}>
              {filterColumns.map((column) => (
                <TextInput
                  key={column}
                  id={`dqa-scope-filter-${column}`}
                  labelText={column}
                  placeholder="Any value"
                  value={filterValues[column] ?? ""}
                  onChange={(e) =>
                    setFilterValues((prev) => ({ ...prev, [column]: e.target.value }))
                  }
                />
              ))}
            </Stack>
          ) : (
            <p style={{ fontSize: "0.75rem", color: "var(--cds-text-secondary)" }}>
              No filter columns are configured for this table.
            </p>
          )}

          <TextInput
            id="dqa-schedule-at"
            type="datetime-local"
            labelText="Run later: date and time"
            helperText="Leave empty and press Run now to scan immediately."
            value={runAt}
            onChange={(e) => setRunAt(e.target.value)}
          />

          {error ? (
            <InlineNotification kind="error" title="Check the form" subtitle={error} lowContrast hideCloseButton />
          ) : null}
        </Stack>
      </ModalBody>
      <ModalFooter>
        <Button kind="secondary" onClick={onClose} disabled={busy}>
          Cancel
        </Button>
        <Button kind="tertiary" onClick={handleRunLater} disabled={busy}>
          {isScheduling ? "Scheduling…" : "Run later"}
        </Button>
        <Button kind="primary" onClick={handleRunNow} disabled={busy}>
          {isRunning ? "Running…" : "Run now"}
        </Button>
      </ModalFooter>
    </ComposedModal>
  );
}
