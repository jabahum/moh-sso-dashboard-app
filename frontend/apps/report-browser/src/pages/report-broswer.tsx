import "./report-browser.scss";

const ReportBrowser = () => {
  const config = (window as Window & { __APP_CONFIG__?: { HEALTH_BI_BASE_URL?: string } })
    .__APP_CONFIG__;
  const baseUrl =
    config?.HEALTH_BI_BASE_URL ||
    import.meta.env.VITE_HEALTH_BI_BASE_URL ||
    "https://dashboards.health.go.ug/report-browser";
  return (
    <div className="report-browser">
      <iframe
        src={`${baseUrl.replace(/\/$/, "")}/`}
        title="Health Dashboard Report Browser"
        className="report-browser__frame"
        allowFullScreen
      />
    </div>
  );
};

export default ReportBrowser;
