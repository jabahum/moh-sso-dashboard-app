import type { HealthBIParameter } from "../types";

export function populateReportParameters(
  schema: HealthBIParameter[],
  overrides: Record<string, unknown>,
): Record<string, unknown> {
  const defaults: Record<string, unknown> = {};
  for (const parameter of schema) {
    if (parameter.metadata?.noDefault && !parameter.metadata?.hideNone) continue;
    const value =
      parameter.defaultValue !== undefined
        ? parameter.defaultValue
        : parameter.metadata?.defaultValue;
    if (value !== undefined && value !== null) defaults[parameter.name] = value;
  }
  // Explicit blanks and falsy values are intentional overrides as well.
  return { ...defaults, ...overrides };
}

export function parameterInputValue(value: unknown): string {
  if (value == null) return "";
  if (Array.isArray(value)) return value.join(", ");
  return String(value);
}
