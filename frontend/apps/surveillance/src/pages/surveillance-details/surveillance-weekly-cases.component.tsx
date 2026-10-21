import { SimpleBarChart } from "@carbon/charts-react";
import { ScaleTypes } from "@carbon/charts";
import "@carbon/charts/styles.css";
import { useMemo } from "react";
import "./surveillance-details.scss";

type WeeklyCasesPoint = {
  week: string | number;
  value: number;
  label?: string;
};

type WeeklyCasesChartProps = {
  diseaseName?: string;
  data?: WeeklyCasesPoint[];
  height?: string;
};

function toWeekNumber(value: string | number): number | null {
  if (typeof value === "number") {
    return Number.isFinite(value) ? value : null;
  }

  const trimmed = String(value ?? "").trim();
  if (!trimmed) return null;

  const directNumber = Number(trimmed);
  if (Number.isFinite(directNumber)) {
    return directNumber;
  }

  const match = trimmed.match(/week\s*(\d{1,2})/i);
  if (match) {
    const parsed = Number(match[1]);
    return Number.isFinite(parsed) ? parsed : null;
  }

  return null;
}

export default function WeeklyCasesChart({
  diseaseName = "Disease",
  data = [],
  height = "320px",
}: WeeklyCasesChartProps) {
  const chartData = useMemo(() => {
    const weekMap = new Map<number, number>();

    for (const item of data) {
      const weekNumber = toWeekNumber(item.week);
      if (weekNumber === null || weekNumber < 1 || weekNumber > 53) {
        continue;
      }

      const currentValue = weekMap.get(weekNumber) ?? 0;
      weekMap.set(weekNumber, currentValue + Number(item.value ?? 0));
    }

    return Array.from({ length: 52 }, (_, index) => {
      const week = index + 1;

      return {
        group: "Cases",
        key: `Week ${week}`,
        value: weekMap.get(week) ?? 0,
      };
    });
  }, [data]);

  const options = useMemo(
    () => ({
      title: `${diseaseName} Weekly Cases`,
      axes: {
        left: {
          mapsTo: "value",
          title: "Total cases",
          scaleType: ScaleTypes.LINEAR,
        },
        bottom: {
          mapsTo: "key",
          title: "EPI Weeks",
          scaleType: ScaleTypes.LABELS,
        },
      },
      height,
      toolbar: {
        enabled: false,
      },
      legend: {
        enabled: false,
      },
    }),
    [diseaseName, height],
  );

  return (
    <div className="disease-details-page__chart">
      <SimpleBarChart data={chartData} options={options} />
    </div>
  );
}
