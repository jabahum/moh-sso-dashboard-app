import { Tile, Tag, Stack, SkeletonText } from "@carbon/react";
import { Link } from "react-router-dom";
import "./signal-tile.scss";

import { getSeverityTagType, type Severity } from "@moh-sso/ui";

type Props = {
  label: string;
  /** `null`/`undefined` renders an em dash, e.g. when the data failed to load. */
  value?: number | string | null;
  severity?: Severity;
  helperText?: string;
  loading?: boolean;
  /** Optional route to open the list behind this number. */
  href?: string;
};

export function SignalTile({ label, value, severity, helperText, loading = false, href }: Props) {
  const displayValue =
    value === null || value === undefined || value === ""
      ? "—"
      : typeof value === "number"
        ? value.toLocaleString()
        : value;

  return (
    <Tile className={`signal-tile ${severity ? `signal-tile--${severity}` : ""}`}>
      <Stack gap={2}>
        {href ? (
          <Link className="signal-tile__label signal-tile__label--link" to={href}>
            {label}
          </Link>
        ) : (
          <span className="signal-tile__label">{label}</span>
        )}

        {loading ? (
          <SkeletonText heading width="40%" className="signal-tile__skeleton" />
        ) : (
          <div className="signal-tile__value">{displayValue}</div>
        )}

        {!loading && (severity || helperText) && (
          <Stack orientation="horizontal" gap={2}>
            {severity && (
              <Tag size="sm" type={getSeverityTagType(severity)}>
                {severity}
              </Tag>
            )}

            {helperText && <span className="signal-tile__helper">{helperText}</span>}
          </Stack>
        )}
      </Stack>
    </Tile>
  );
}
