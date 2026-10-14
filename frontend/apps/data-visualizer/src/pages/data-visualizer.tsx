import React, { useState } from "react";

import DataModal from "./modals/data-model/DataModal.tsx";
import PeriodModal from "./modals/period/PeriodModal.tsx";
import OrgUnitModal from "./modals/orgunit/OrgUnitModal.tsx";

import "./data-visualizer.scss";
import { Button } from "@carbon/react";
import { Download, FilterRemove, UpdateNow } from "@carbon/react/icons";

import GeneralModal from "./modals/GeneralModal.tsx";
import ChartRenderer from "./chartrenderer/ChartRenderer.tsx";
import ClearDataConfirmationModal from "./modals/ClearDataConfirmationModal.tsx";

type VisualizerQuery = {
  dx: string[];
  pe: string[];
  ou: string[];
  levelOfCare: string[];
  ownership: string[];
  aggregationLevel?: string;
};

const DataVisualizer = () => {
  const [selectedData, setSelectedData] = useState([]);
  const [selectedPeriods, setSelectedPeriods] = useState([]);
  const [selectedOrgUnits, setSelectedOrgUnits] = useState([]);
  const [selectedLevelOfCare, setSelectedLevelOfCare] = useState<string[]>([]);
  const [selectedOwnership, setSelectedOwnership] = useState<string[]>([]);
  const [aggregationLevel, setAggregationLevel] = useState<string>("");
  const [loadedChartData, setLoadedChartData] = useState([]);
  const [pivotChartData, setPivotChartData] = useState([]);
  const [showModal, setShowModal] = useState(null);
  const [appliedQuery, setAppliedQuery] = useState<VisualizerQuery | null>(null);
  const [isClearModalOpen, setIsClearModalOpen] = useState(false);
  const [updateTrigger, setUpdateTrigger] = useState(0);

  // Load saved state from localStorage and URL query strings on component mount
  React.useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const ouParam = params.get("ou");
    const levelOfCareParam = params.get("levelOfCare");
    const ownershipParam = params.get("ownership");
    const aggregationLevelParam = params.get("aggregationLevel");

    const savedData = localStorage.getItem("selectedData");
    const savedPeriods = localStorage.getItem("selectedPeriods");
    const savedOrgUnits = ouParam ? ouParam.split(",").filter(Boolean) : JSON.parse(localStorage.getItem("selectedOrgUnits") || "[]");
    const savedLevelOfCare = levelOfCareParam ? levelOfCareParam.split(",").filter(Boolean) : JSON.parse(localStorage.getItem("selectedLevelOfCare") || "[]");
    const savedOwnership = ownershipParam ? ownershipParam.split(",").filter(Boolean) : JSON.parse(localStorage.getItem("selectedOwnership") || "[]");
    const savedAggregationLevel = aggregationLevelParam || localStorage.getItem("aggregationLevel") || "";
    const savedLoadedData = localStorage.getItem("loadedChartData");
    const savedPivotData = localStorage.getItem("pivotChartData");

    if (savedData) setSelectedData(JSON.parse(savedData));
    if (savedPeriods) setSelectedPeriods(JSON.parse(savedPeriods));
    setSelectedOrgUnits(savedOrgUnits);
    setSelectedLevelOfCare(savedLevelOfCare);
    setSelectedOwnership(savedOwnership);
    setAggregationLevel(savedAggregationLevel);
    if (savedLoadedData) setLoadedChartData(JSON.parse(savedLoadedData));
    if (savedPivotData) setPivotChartData(JSON.parse(savedPivotData));
  }, []);

  // Save state to localStorage whenever selections change
  React.useEffect(() => {
    localStorage.setItem("selectedData", JSON.stringify(selectedData));
  }, [selectedData]);

  React.useEffect(() => {
    localStorage.setItem("selectedPeriods", JSON.stringify(selectedPeriods));
  }, [selectedPeriods]);

  React.useEffect(() => {
    localStorage.setItem("selectedOrgUnits", JSON.stringify(selectedOrgUnits));
  }, [selectedOrgUnits]);

  React.useEffect(() => {
    localStorage.setItem("selectedLevelOfCare", JSON.stringify(selectedLevelOfCare));
  }, [selectedLevelOfCare]);

  React.useEffect(() => {
    localStorage.setItem("selectedOwnership", JSON.stringify(selectedOwnership));
  }, [selectedOwnership]);

  React.useEffect(() => {
    localStorage.setItem("aggregationLevel", aggregationLevel);
  }, [aggregationLevel]);

  React.useEffect(() => {
    localStorage.setItem("loadedChartData", JSON.stringify(loadedChartData));
  }, [loadedChartData]);

  React.useEffect(() => {
    localStorage.setItem("pivotChartData", JSON.stringify(pivotChartData));
  }, [pivotChartData]);

  const open = (which) => {
    setShowModal(which);
  };
  const close = () => {
    setShowModal(null);
  };

  // Function to clear all selections
  const clearAllSelections = () => {
    setSelectedData([]);
    setSelectedPeriods([]);
    setSelectedOrgUnits([]);
    setSelectedLevelOfCare([]);
    setSelectedOwnership([]);
    setAggregationLevel("");
    setPivotChartData([]);
    setLoadedChartData([]);
    setAppliedQuery(null);
    setIsClearModalOpen(false);
  };

  const saveOrgUnitFilters = (orgUnits, levelOfCare, ownership, aggLevel = "") => {
    setSelectedOrgUnits(orgUnits);
    setSelectedLevelOfCare(levelOfCare);
    setSelectedOwnership(ownership);
    setAggregationLevel(aggLevel);
  };

  // Function to check if all required dimensions are selected
  const isDataReady =
    selectedData?.length > 0 && selectedPeriods?.length > 0 && selectedOrgUnits?.length > 0;

  const query = React.useMemo<VisualizerQuery | null>(() => {
    if (!isDataReady) return null;
    const baseQuery: VisualizerQuery = {
      dx: selectedData.map((item: any) => item.data_element_id),
      pe: selectedPeriods.map((item: any) => item.id),
      ou: selectedOrgUnits,
      levelOfCare: selectedLevelOfCare,
      ownership: selectedOwnership,
    };
    if (aggregationLevel) {
      baseQuery.aggregationLevel = aggregationLevel;
    }
    return baseQuery;
  }, [selectedData, selectedPeriods, selectedOrgUnits, selectedLevelOfCare, selectedOwnership, aggregationLevel, isDataReady]);

  React.useEffect(() => {
    setAppliedQuery(query);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [updateTrigger]);

  React.useEffect(() => {
    if (appliedQuery && JSON.stringify(query) !== JSON.stringify(appliedQuery)) {
      setLoadedChartData([]);
      setPivotChartData([]);
      setAppliedQuery(null);
    }
  }, [query, appliedQuery]);

  const getDynamicFileName = (baseName = "Data_Report") => {
    const now = new Date();
    const datePart = now.toISOString().split("T")[0];
    const hours = String(now.getHours()).padStart(2, "0");
    const minutes = String(now.getMinutes()).padStart(2, "0");
    const seconds = String(now.getSeconds()).padStart(2, "0");

    return `${baseName}_${datePart}_${hours}-${minutes}-${seconds}.csv`;
  };

  const exportDataToCSV = () => {
    const table = document.querySelector(".pvtTable");
    if (!table) return;

    const filename = getDynamicFileName();

    // @ts-ignore
    const rows = Array.from(table.rows);
    const grid: string[][] = [];

    rows.forEach((row, rowIndex) => {
      if (!grid[rowIndex]) {
        // @ts-ignore
        grid[rowIndex] = [];
      }

      let colIndex = 0;
      // @ts-ignore
      Array.from(row.cells).forEach((cell) => {
        while (grid[rowIndex][colIndex] !== undefined) {
          colIndex++;
        }

        // @ts-ignore
        const rowspan = cell.rowSpan || 1;
        // @ts-ignore
        const colspan = cell.colSpan || 1;
        // @ts-ignore
        const content = `"${cell.innerText.trim().replace(/"/g, '""')}"`;

        // Fill the grid for the current rowspan and colspan
        for (let r = 0; r < rowspan; r++) {
          for (let c = 0; c < colspan; c++) {
            const targetRow = rowIndex + r;
            const targetCol = colIndex + c;

            if (!grid[targetRow]) {
              // @ts-ignore
              grid[targetRow] = [];
            }

            // @ts-ignore
            grid[targetRow][targetCol] = r === 0 && c === 0 ? content : '""';
          }
        }
        colIndex += colspan;
      });
    });

    // @ts-ignore
    const csvContent = grid.map((row) => row.join(",")).join("\n");

    const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.setAttribute("href", url);
    link.setAttribute("download", filename);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  };

  return (
    <div className="dv-container">
      {/* Main Workspace */}
      <div className="dv-workspace">
        {/* Toolbar */}
        <div className="dv-toolbar">
          <Button
            size="sm"
            kind="tertiary"
            renderIcon={UpdateNow}
            className={`dwh-btn-width`}
            onClick={() => {
              setUpdateTrigger((prev) => prev + 1);
            }}
          >
            Update
          </Button>

          {loadedChartData?.length > 0 && (
              <Button
                size="sm"
                kind="primary"
                renderIcon={Download}
                className={`dwh-btn-width`}
                onClick={exportDataToCSV}
              >
                Download
              </Button>
          )}

         <Button
            size="sm"
            kind="danger--tertiary"
            renderIcon={FilterRemove}
            className={`dwh-btn-width`}
            onClick={() => setIsClearModalOpen(true)}
          >
            Clear All
          </Button>
        </div>

        {/* Layout Area */}
        <div className="dv-layout">
          {/* Status Indicator */}
          {!isDataReady && (
            <div className="dv-alert dv-alert-warning">
              <i className="fas fa-exclamation-triangle"></i>
              Please select Data, Period, and Organisation Unit to view data
            </div>
          )}

          {/* Dimension Layout */}
          <div className="dv-dimension-layout">
            <div className="dv-dimension-grid">
              <div className="dv-dimension-section">
                <div className="dv-section-label">Data</div>
                <div className="dv-dropzone">
                  <div className="dv-dropzone-content">
                    {selectedData.length >= 0 ? (
                      <div
                        className="dv-badge"
                        onClick={() => {
                          open("data");
                        }}
                        style={{ cursor: "pointer" }}
                      >
                        <i className="fas fa-database dv-badge-icon"></i>
                        <span>Data</span>
                        <span className="dv-badge-count">{selectedData.length}</span>
                      </div>
                    ) : (
                      <span className="dv-placeholder">Select data dimension</span>
                    )}
                  </div>
                </div>
              </div>

              <div className="dv-dimension-section">
                <div className="dv-section-label">Period</div>
                <div className="dv-dropzone">
                  <div className="dv-dropzone-content">
                    {selectedPeriods.length >= 0 ? (
                      <div
                        className="dv-badge"
                        onClick={() => {
                          open("period");
                        }}
                        style={{ cursor: "pointer" }}
                      >
                        <i className="fas fa-clock dv-badge-icon"></i>
                        <span>Period</span>
                        <span className="dv-badge-count">{selectedPeriods.length}</span>
                      </div>
                    ) : (
                      <span className="dv-placeholder">Select period dimension</span>
                    )}
                  </div>
                </div>
              </div>

              <div className="dv-dimension-section">
                <div className="dv-section-label">Organisation Unit</div>
                <div className="dv-dropzone">
                  <div className="dv-dropzone-content">
                    {selectedOrgUnits.length >= 0 ? (
                      <div
                        className="dv-badge"
                        onClick={() => {
                          open("orgunit");
                        }}
                        style={{ cursor: "pointer" }}
                      >
                        <i className="fas fa-sitemap dv-badge-icon"></i>
                        <span>Organisation Unit</span>
                        <span className="dv-badge-count">{selectedOrgUnits.length}</span>
                      </div>
                    ) : (
                      <span className="dv-placeholder">Select organisation unit dimension</span>
                    )}
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div className="dv-canvas">
            {appliedQuery || loadedChartData?.length > 0 ? (
              <ChartRenderer
                queryParams={appliedQuery}
                onSaveLoadedData={setLoadedChartData}
                onSavePivotData={setPivotChartData}
                loadedData={loadedChartData}
                pivotData={pivotChartData}
                periods={selectedPeriods}
                updateTrigger={updateTrigger}
              />
            ) : (
              <div className="dv-canvas-placeholder">
                <i className="fas fa-chart-bar"></i>
                <h6>No Data Available</h6>
                <p>
                  Select data elements, periods, and organization units, then click Update to see
                  your visualization
                </p>
              </div>
            )}
          </div>
        </div>
      </div>

      {showModal === "data" && (
        <DataModal
          onClose={close}
          selected={selectedData}
          onSave={setSelectedData}
          updateTrigger={() => setUpdateTrigger((prev) => prev + 1)}
        />
      )}
      {showModal === "period" && (
        <PeriodModal
          onClose={close}
          selected={selectedPeriods}
          onSave={setSelectedPeriods}
          updateTrigger={() => setUpdateTrigger((prev) => prev + 1)}
        />
      )}
      {showModal === "orgunit" && (
        <OrgUnitModal
          onClose={close}
          selected={selectedOrgUnits}
          selectedLevelOfCare={selectedLevelOfCare}
          selectedOwnership={selectedOwnership}
          aggregationLevel={aggregationLevel}
          onSave={saveOrgUnitFilters}
          updateTrigger={() => setUpdateTrigger((prev) => prev + 1)}
        />
      )}
      {showModal === "general" && (
        <GeneralModal
          onClose={close}
          selectedData={selectedData}
          onSaveData={setSelectedData}
          selectedPeriods={selectedPeriods}
          onSavePeriods={setSelectedPeriods}
          selectedOrgUnits={selectedOrgUnits}
          selectedLevelOfCare={selectedLevelOfCare}
          selectedOwnership={selectedOwnership}
          aggregationLevel={aggregationLevel}
          onSaveOrgUnits={saveOrgUnitFilters}
          updateTrigger={() => setUpdateTrigger((prev) => prev + 1)}
        />
      )}
      {isClearModalOpen && (
        <ClearDataConfirmationModal
          setIsClearModalOpen={setIsClearModalOpen}
          isClearModalOpen={isClearModalOpen}
          handleConfirmClear={clearAllSelections}
        />
      )}
    </div>
  );
};

export default DataVisualizer;
