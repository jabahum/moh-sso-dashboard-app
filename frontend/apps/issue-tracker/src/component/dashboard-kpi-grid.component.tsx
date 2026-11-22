import type { DashboardGrandTotals } from "../types";

export interface DashboardKpiGridProps {
  grandTotals: DashboardGrandTotals;
  isLoading: boolean;
}

export const DashboardKpiGrid = ({ grandTotals, isLoading }: DashboardKpiGridProps) => {
  const resolutionRate =
    grandTotals.total > 0
      ? Math.round((grandTotals.resolved / grandTotals.total) * 100)
      : 0;

  return (
    <div className="kpi-grid">
      <div className="kpi-card kpi-card--total">
        <div className="kpi-header">
          <span className="kpi-label">Total Issues</span>
        </div>
        <div className="kpi-value">{isLoading ? "..." : grandTotals.total}</div>
        <p className="kpi-subtext">
          Across <strong>{grandTotals.activeRegionsCount}</strong> affected regions
        </p>
      </div>

      <div className="kpi-card kpi-card--critical">
        <div className="kpi-header">
          <span className="kpi-label">Critical Issues</span>
          <span className="kpi-badge kpi-badge--critical">Critical</span>
        </div>
        <div className="kpi-value kpi-value--critical">
          {isLoading ? "..." : grandTotals.critical}
        </div>
        <p className="kpi-subtext">
          <strong>
            {grandTotals.open > 0
              ? `${((grandTotals.critical / grandTotals.open) * 100).toFixed(1)}%`
              : "0%"}
          </strong>{" "}
          of open issues
        </p>
      </div>

      <div className="kpi-card kpi-card--open">
        <div className="kpi-header">
          <span className="kpi-label">Open Issues</span>
          <span className="kpi-badge kpi-badge--open">Open</span>
        </div>
        <div className="kpi-value kpi-value--open">
          {isLoading ? "..." : grandTotals.open}
        </div>
        <p className="kpi-subtext">
          <strong>
            {grandTotals.total > 0
              ? `${((grandTotals.open / grandTotals.total) * 100).toFixed(1)}%`
              : "0%"}
          </strong>{" "}
          of total issues
        </p>
      </div>

      <div className="kpi-card kpi-card--resolved">
        <div className="kpi-header">
          <span className="kpi-label">Resolved Issues</span>
          <span className="kpi-badge kpi-badge--resolved">Resolved</span>
        </div>
        <div className="kpi-value kpi-value--resolved">
          {isLoading ? "..." : grandTotals.resolved}
        </div>
        <p className="kpi-subtext">
          <strong>
            {grandTotals.total > 0
              ? `${((grandTotals.resolved / grandTotals.total) * 100).toFixed(1)}%`
              : "0%"}
          </strong>{" "}
          of total issues
        </p>
      </div>

      <div className="kpi-card kpi-card--rate">
        <div className="kpi-header">
          <span className="kpi-label">Resolution Rate</span>
          <span className="kpi-badge kpi-badge--rate">{resolutionRate}%</span>
        </div>
        <div className="kpi-value kpi-value--rate">
          {isLoading ? "..." : `${resolutionRate}%`}
        </div>
        <p className="kpi-subtext">
          Resolved vs <strong>{grandTotals.open}</strong> open issues
        </p>
      </div>
    </div>
  );
};
