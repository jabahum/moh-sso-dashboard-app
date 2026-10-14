import { useEffect, useState } from "react";
import { Loading } from "@carbon/react";
import PivotTableUI from "react-pivottable/PivotTableUI";
import createPlotlyRenderers from "react-pivottable/PlotlyRenderers";
import TableRenderers from "react-pivottable/TableRenderers";
import { aggregatorTemplates } from "react-pivottable/Utilities";
import Plot from "react-plotly.js";

import "react-pivottable/pivottable.css";
import { useLazyGetDataValuesQuery } from "./ChartOptions.ts";

const ChartRenderer = ({
  queryParams,
  onSaveLoadedData,
  onSavePivotData,
  loadedData,
  pivotData,
  periods,
  updateTrigger,
}) => {
  const utils = createPlotlyRenderers(Plot);
  const intFormat = (val) => Math.round(val).toLocaleString();
  const [loading, setLoading] = useState(!!queryParams);
  const [chartData, setChartData] = useState(loadedData);
  const [pivotTableData, setPivotTableData] = useState(pivotData);
  const [triggerGetDataValues] = useLazyGetDataValuesQuery();

  const [prevQueryParams, setPrevQueryParams] = useState(queryParams);
  if (queryParams !== prevQueryParams) {
    setPrevQueryParams(queryParams);
    setLoading(true);
    setChartData([]);
  }

  const customAggregators = {
    Sum: aggregatorTemplates.sum(intFormat)
  };
  useEffect(() => {
    if (!queryParams) return;

    const fetchChartData = async () => {
      setLoading(true);
      setChartData([]);
      try {
        const data = await triggerGetDataValues(queryParams).unwrap();
        const rows = data?.["rows"] || [];

        const mappedPivotData =
          rows?.map((item) => ({
            "Data Element": item?.dataelement,
            "Organisation Unit": item?.org_unit_name,
            Period: periods?.find((period) => period?.id === item?.period)?.label ?? "",
            Value: item?.value,
            "Level of Care": item?.level_of_care,
            Ownership: item?.ownership
          })) ?? [];

        setChartData(rows);
        onSaveLoadedData(rows);

        setPivotTableData(mappedPivotData);
        onSavePivotData(mappedPivotData);
      } catch (error) {
        setChartData([]);
        console.error("Error Encountered while fetching data elements:: " + error);
      } finally {
        setLoading(false);
      }
    };

    fetchChartData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [queryParams, updateTrigger]);

  return (
    <>
      {loading ? (
        <div className="dv-loading-container">
          <Loading description="Loading data..." withOverlay={false} />
        </div>
      ) : (
        <>
          {chartData.length === 0 && !loading && (
            <div className="dv-canvas-placeholder">
              <i className="fas fa-chart-bar"></i>
              <h6>No Data Available for the selection</h6>
            </div>
          )}

          {chartData?.length > 0 && (
            <div className={`dwh-chart-container`}>
              <PivotTableUI
                data={pivotTableData}
                cols={["Data Element"]}
                rows={["Organisation Unit"]}
                aggregators={customAggregators}
                aggregatorName="Sum"
                vals={["Value"]}
                hiddenFromDragDrop={["Value"]}
                hiddenFromAggregators={[
                  "Data Element",
                  "Organisation Unit",
                  "Age-Sex Disaggregation",
                  "District",
                  "Period",
                  "Region",
                  "SubCounty",
                ]}
                onChange={(s) => {
                  setPivotTableData(s);
                }}
                renderers={{ ...TableRenderers, ...utils }}
                {...pivotTableData}
              />
            </div>
          )}
        </>
      )}
    </>
  );
};

export default ChartRenderer;
