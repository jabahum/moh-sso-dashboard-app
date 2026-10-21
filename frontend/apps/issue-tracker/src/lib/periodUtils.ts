import { getAvailablePeriods } from "../../../data-visualizer/src/pages/Constants.tsx";

/**
 * Wraps getAvailablePeriods to ensure all periods (including future quarters/months)
 * are enabled for filtering issues.
 */
export function fetchAvailablePeriods(periodType: string, year: string | number) {
  const periods = getAvailablePeriods(periodType, year.toString());
  return periods.map((p) => ({ ...p, disabled: false }));
}

/**
 * Utility to convert period selections, labels, or raw period strings to standard DHIS2 codes:
 * - Year + Quarter: YYYYQ# (e.g., 2026Q1)
 * - Year + Month: YYYYMM (e.g., 202606)
 * - Year + Week: YYYYWww (e.g., 2026W02)
 */
export function convertPeriodToCode(
  periodInput: unknown,
  fallbackYear?: number | string
): string {
  if (periodInput === null || periodInput === undefined || periodInput === "") {
    return "";
  }

  let inputStr = "";
  if (typeof periodInput === "object") {
    const obj = periodInput as { id?: string; label?: string; name?: string; value?: string };
    if (obj.id && /^\d{4}(Q[1-4]|(0[1-9]|1[0-2])|W\d{2})$/i.test(obj.id)) {
      return obj.id.toUpperCase();
    }
    inputStr = obj.id || obj.label || obj.name || obj.value || "";
  } else {
    inputStr = String(periodInput).trim();
  }

  if (!inputStr) return "";

  const clean = inputStr.trim();

  // 1. Quarterly code: 2026Q1, 2026-Q1, Q1 2026, Q1-2026, 2026Q01
  const qCodeMatch = clean.match(/^(\d{4})[-_\s]?[qQ]0?([1-4])$/);
  if (qCodeMatch) {
    return `${qCodeMatch[1]}Q${qCodeMatch[2]}`;
  }
  const qRevCodeMatch = clean.match(/^[qQ]0?([1-4])[-_\s]?(\d{4})$/);
  if (qRevCodeMatch) {
    return `${qRevCodeMatch[2]}Q${qRevCodeMatch[1]}`;
  }

  // 2. Weekly code: 2026W02, 2026W2, 2026-W02, W02 2026
  const wCodeMatch = clean.match(/^(\d{4})[-_\s]?[wW](\d{1,2})$/);
  if (wCodeMatch) {
    const year = wCodeMatch[1];
    const week = String(wCodeMatch[2]).padStart(2, "0");
    return `${year}W${week}`;
  }
  const wRevCodeMatch = clean.match(/^[wW](\d{1,2})[-_\s]?(\d{4})$/);
  if (wRevCodeMatch) {
    const year = wRevCodeMatch[2];
    const week = String(wRevCodeMatch[1]).padStart(2, "0");
    return `${year}W${week}`;
  }

  // 3. Monthly code: 202606 (YYYYMM 6 digits)
  const mCodeMatch = clean.match(/^(\d{4})(0[1-9]|1[0-2])$/);
  if (mCodeMatch) {
    return `${mCodeMatch[1]}${mCodeMatch[2]}`;
  }

  // 4. Monthly format: YYYY-MM, YYYY/MM, MM-YYYY, MM/YYYY
  const mDashMatch = clean.match(/^(\d{4})[-/](0[1-9]|1[0-2])$/);
  if (mDashMatch) {
    return `${mDashMatch[1]}${mDashMatch[2]}`;
  }
  const mRevDashMatch = clean.match(/^(0[1-9]|1[0-2])[-/](\d{4})$/);
  if (mRevDashMatch) {
    return `${mRevDashMatch[2]}${mRevDashMatch[1]}`;
  }

  // Extract year if present in label, otherwise use fallbackYear
  const yearMatch = clean.match(/\b(19\d\d|20\d\d)\b/);
  const year = yearMatch ? yearMatch[1] : fallbackYear ? String(fallbackYear) : "";

  // Quarter text matching: e.g. "January - March 2026", "Jan - Mar 2026", "Quarter 1", "Q1"
  const qWordMatch = clean.match(/quarter\s*0?([1-4])/i);
  if (qWordMatch && year) {
    return `${year}Q${qWordMatch[1]}`;
  }

  if (/jan(?:uary)?\s*-\s*mar(?:ch)?/i.test(clean) || /\bquarter\s*1\b/i.test(clean) || /\bq1\b/i.test(clean)) {
    if (year) return `${year}Q1`;
  }
  if (/apr(?:il)?\s*-\s*jun(?:e)?/i.test(clean) || /\bquarter\s*2\b/i.test(clean) || /\bq2\b/i.test(clean)) {
    if (year) return `${year}Q2`;
  }
  if (/jul(?:y)?\s*-\s*sep(?:tember)?/i.test(clean) || /\bquarter\s*3\b/i.test(clean) || /\bq3\b/i.test(clean)) {
    if (year) return `${year}Q3`;
  }
  if (/oct(?:ober)?\s*-\s*dec(?:ember)?/i.test(clean) || /\bquarter\s*4\b/i.test(clean) || /\bq4\b/i.test(clean)) {
    if (year) return `${year}Q4`;
  }

  // Week text matching: e.g. "Week 2 - 2026-01-05 - 2026-01-11", "Week 02 2026", "Week 2"
  const weekTextMatch = clean.match(/\bweek\s*(\d{1,2})\b/i);
  if (weekTextMatch) {
    const weekNum = String(weekTextMatch[1]).padStart(2, "0");
    if (year) return `${year}W${weekNum}`;
  }

  // Month text matching: e.g. "June 2026", "January 2026", "Jun 2026"
  const monthsMap: Record<string, string> = {
    january: "01", jan: "01",
    february: "02", feb: "02",
    march: "03", mar: "03",
    april: "04", apr: "04",
    may: "05",
    june: "06", jun: "06",
    july: "07", jul: "07",
    august: "08", aug: "08",
    september: "09", sep: "09", sept: "09",
    october: "10", oct: "10",
    november: "11", nov: "11",
    december: "12", dec: "12",
  };

  const lowerClean = clean.toLowerCase();
  for (const [mName, mNum] of Object.entries(monthsMap)) {
    if (new RegExp(`\\b${mName}\\b`, "i").test(lowerClean)) {
      if (year) return `${year}${mNum}`;
    }
  }

  return clean;
}

/**
 * Checks if a target filter period matches an issue's reporting period.
 * Performs full string matching (matching exact raw string or exact normalized period code).
 */
export function isPeriodMatching(
  targetPeriodInput: unknown,
  issuePeriodInput: unknown,
  fallbackYear?: number | string
): boolean {
  if (!targetPeriodInput) return true; // No filter selected

  const rawTarget = String(targetPeriodInput).trim();
  const rawIssue = String(issuePeriodInput ?? "").trim();

  if (!rawIssue) return false;

  // 1. Direct raw string match (case-insensitive)
  if (rawTarget.toLowerCase() === rawIssue.toLowerCase()) {
    return true;
  }

  // 2. Normalized code full string match (case-insensitive)
  const targetCode = convertPeriodToCode(targetPeriodInput, fallbackYear);
  const issueCode = convertPeriodToCode(issuePeriodInput, fallbackYear);

  if (targetCode && issueCode && targetCode.toLowerCase() === issueCode.toLowerCase()) {
    return true;
  }

  if (targetCode && rawIssue.toLowerCase() === targetCode.toLowerCase()) {
    return true;
  }

  if (issueCode && rawTarget.toLowerCase() === issueCode.toLowerCase()) {
    return true;
  }

  return false;
}

const MONTH_NAMES = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

const QUARTER_NAMES: Record<string, string> = {
  "1": "January - March",
  "2": "April - June",
  "3": "July - September",
  "4": "October - December",
};

const BI_MONTH_NAMES: Record<string, string> = {
  "1": "January - February",
  "2": "March - April",
  "3": "May - June",
  "4": "July - August",
  "5": "September - October",
  "6": "November - December",
};

const SIX_MONTH_NAMES: Record<string, string> = {
  "1": "January - June",
  "2": "July - December",
};

/**
 * Converts a reporting period (e.g., DHIS2 code, timestamp, or raw string)
 * into a human-readable month, week, or quarter plus year name:
 * - 2026Q3 -> "July - September 2026"
 * - 2026Q1 -> "January - March 2026"
 * - 202602 -> "February 2026"
 * - 2026W02 -> "Week 2 2026"
 */
export function formatReportingPeriod(periodInput: unknown): string {
  if (periodInput === null || periodInput === undefined || periodInput === "") {
    return "";
  }

  let str = "";
  if (typeof periodInput === "object") {
    const obj = periodInput as { label?: string; name?: string; value?: string; id?: string };
    str = obj.label || obj.name || obj.value || obj.id || "";
  } else {
    str = String(periodInput).trim();
  }

  if (!str) return "";

  const clean = str.trim();

  // 1. Quarterly: 2026Q3, 2026-Q3, 2026_Q3, 2026 Q3, 2026Q03, Q3 2026, Q3-2026, Q3/2026
  const qMatch =
    clean.match(/^(\d{4})[-_\s/]?[qQ]0?([1-4])$/) ||
    clean.match(/^[qQ]0?([1-4])[-_\s/]?(\d{4})$/);
  if (qMatch) {
    const year = qMatch[1].length === 4 ? qMatch[1] : qMatch[2];
    const qNum = qMatch[1].length === 4 ? qMatch[2] : qMatch[1];
    return `${QUARTER_NAMES[qNum]} ${year}`;
  }

  const qWordMatch =
    clean.match(/quarter\s*0?([1-4])[,\s-]+(\d{4})/i) ||
    clean.match(/(\d{4})[,\s-]+quarter\s*0?([1-4])/i);
  if (qWordMatch) {
    const year = qWordMatch[1].length === 4 ? qWordMatch[1] : qWordMatch[2];
    const qNum = qWordMatch[1].length === 4 ? qWordMatch[2] : qWordMatch[1];
    return `${QUARTER_NAMES[qNum]} ${year}`;
  }

  // 2. Weekly: 2026W02, 2026W2, 2026-W02, W02 2026, Week 2 - 2026-01-05 - 2026-01-11
  const wMatch =
    clean.match(/^(\d{4})[-_\s/]?[wW]0?(\d{1,2})$/) ||
    clean.match(/^[wW]0?(\d{1,2})[-_\s/]?(\d{4})$/);
  if (wMatch) {
    const year = wMatch[1].length === 4 ? wMatch[1] : wMatch[2];
    const wNum = parseInt(wMatch[1].length === 4 ? wMatch[2] : wMatch[1], 10);
    return `Week ${wNum} ${year}`;
  }

  const wRangeMatch = clean.match(/\bweek\s*0?(\d{1,2})\b.*?\b(19\d\d|20\d\d)\b/i);
  if (wRangeMatch) {
    const wNum = parseInt(wRangeMatch[1], 10);
    const year = wRangeMatch[2];
    return `Week ${wNum} ${year}`;
  }

  // 3. Bi-monthly: 202601B, 2026B1
  const bMatch =
    clean.match(/^(\d{4})[-_\s]?[bB]0?([1-6])$/) ||
    clean.match(/^(\d{4})0([1-6])[bB]$/);
  if (bMatch) {
    return `${BI_MONTH_NAMES[bMatch[2]]} ${bMatch[1]}`;
  }

  // 4. Six-monthly: 2026S1, 2026S2
  const sMatch = clean.match(/^(\d{4})[-_\s]?[sS]0?([1-2])$/);
  if (sMatch) {
    return `${SIX_MONTH_NAMES[sMatch[2]]} ${sMatch[1]}`;
  }

  // 5. Full date (ISO YYYY-MM-DD): 2026-02-15 -> February 2026
  const dateMatch = clean.match(/^(\d{4})-(0[1-9]|1[0-2])-\d{2}$/);
  if (dateMatch) {
    const year = dateMatch[1];
    const mIdx = parseInt(dateMatch[2], 10) - 1;
    return `${MONTH_NAMES[mIdx]} ${year}`;
  }

  // 6. Monthly code: 202602 (YYYYMM 6 digits)
  const mCodeMatch = clean.match(/^(\d{4})(0[1-9]|1[0-2])$/);
  if (mCodeMatch) {
    const year = mCodeMatch[1];
    const mIdx = parseInt(mCodeMatch[2], 10) - 1;
    return `${MONTH_NAMES[mIdx]} ${year}`;
  }

  // 7. Monthly with separator: 2026-02, 2026/02, 02-2026, 02/2026, 2026-2
  const mDashMatch = clean.match(/^(\d{4})[-/](0?[1-9]|1[0-2])$/);
  if (mDashMatch) {
    const year = mDashMatch[1];
    const mIdx = parseInt(mDashMatch[2], 10) - 1;
    return `${MONTH_NAMES[mIdx]} ${year}`;
  }
  const mRevDashMatch = clean.match(/^(0?[1-9]|1[0-2])[-/](\d{4})$/);
  if (mRevDashMatch) {
    const year = mRevDashMatch[2];
    const mIdx = parseInt(mRevDashMatch[1], 10) - 1;
    return `${MONTH_NAMES[mIdx]} ${year}`;
  }

  // 8. Quarter range text: e.g. "Jul - Sep 2026", "July - September 2026"
  const qTextMatch = clean.match(
    /(jan(?:uary)?\s*-\s*mar(?:ch)?|apr(?:il)?\s*-\s*jun(?:e)?|jul(?:y)?\s*-\s*sep(?:tember)?|oct(?:ober)?\s*-\s*dec(?:ember)?)[,\s-]+(\d{4})/i
  );
  if (qTextMatch) {
    const t = qTextMatch[1].toLowerCase();
    const year = qTextMatch[2];
    if (t.includes("jan")) return `January - March ${year}`;
    if (t.includes("apr")) return `April - June ${year}`;
    if (t.includes("jul")) return `July - September ${year}`;
    if (t.includes("oct")) return `October - December ${year}`;
  }

  // 9. Month name text: e.g. "Feb 2026", "February 2026", "2026 February"
  const monthWordMatch =
    clean.match(
      /\b(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|sept|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\b[,\s-]+(\d{4})/i
    ) ||
    clean.match(
      /(\d{4})[,\s-]+\b(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|sept|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\b/i
    );
  if (monthWordMatch) {
    const part1 = monthWordMatch[1];
    const part2 = monthWordMatch[2];
    const year = /^\d{4}$/.test(part1) ? part1 : part2;
    const mName = /^\d{4}$/.test(part1) ? part2.toLowerCase() : part1.toLowerCase();
    const mIndexMap: Record<string, number> = {
      jan: 0,
      january: 0,
      feb: 1,
      february: 1,
      mar: 2,
      march: 2,
      apr: 3,
      april: 3,
      may: 4,
      jun: 5,
      june: 5,
      jul: 6,
      july: 6,
      aug: 7,
      august: 7,
      sep: 8,
      sept: 8,
      september: 8,
      oct: 9,
      october: 9,
      nov: 10,
      november: 10,
      dec: 11,
      december: 11,
    };
    if (mName in mIndexMap) {
      return `${MONTH_NAMES[mIndexMap[mName]]} ${year}`;
    }
  }

  return clean;
}

export { formatReportingPeriod as convertReportingPeriod };
