import {
  ComboBox,
  Modal,
  TextArea,
  Search,
  TreeView,
  NumberInput,
} from "@carbon/react";
import { ChevronDown, ChevronUp } from "@carbon/react/icons";
import { useCallback, useEffect, useState } from "react";
import { useSelector } from "react-redux";
import { selectUser } from "@moh-sso/auth";

import {
  useCreateIssueMutation,
  useUpdateIssueMutation,
  useGetIssueDataSetsQuery,
  useLazyGetIssueDataSetElementsQuery,
  useGetIssueOrgHierarchyQuery,
} from "../api";
import { IssueTypes, Priority, Severity, periodType } from "../lib/constants";
import { getAvailablePeriods } from "../lib/periodUtils";
import type { Issue, IssuePayload, SelectEvent, ThemeElement } from "../types";
import { OrgUnitNode, type OrgUnitNodeData } from "./tree-node.component";

export const IssueModal = ({
  onClose,
  selectedIssue,
}: {
  onClose: () => void;
  selectedIssue?: Issue | null;
}) => {
  const CURRENT_YEAR = new Date().getFullYear();
  const initialPeriodType = "Quarterly";
  const isEdit = Boolean(selectedIssue);

  const [selectedDataset, setSelectedDataset] = useState(selectedIssue?.dataset ?? "");
  const [dataElements, setDataElements] = useState<ThemeElement[]>([]);
  const [description, setDescription] = useState(selectedIssue?.issue ?? "");
  const [selectedIssueType, setSelectedIssueType] = useState(selectedIssue?.issue_type ?? "");
  const [selectedDataElement, setSelectedDataElement] = useState(
    selectedIssue?.data_element ?? "",
  );
  const [priority, setPriority] = useState(selectedIssue?.priority ?? "");
  const [severity, setSeverity] = useState(selectedIssue?.severity ?? "");

  const { data: datasets = [] } = useGetIssueDataSetsQuery();
  const [triggerGetDatasetElements] = useLazyGetIssueDataSetElementsQuery();
  const { data: orgUnits = [] } = useGetIssueOrgHierarchyQuery();

  const [updateIssue, { isLoading: isUpdating }] = useUpdateIssueMutation();
  const [createIssue, { isLoading: isCreating }] = useCreateIssueMutation();
  const user = useSelector(selectUser);

  const [isOrgExpanded, setIsOrgExpanded] = useState(false);
  const [selectedOrgUnit, setSelectedOrgUnit] = useState(selectedIssue?.org_unit ?? "");
  const [orgSearchTerm, setOrgSearchTerm] = useState("");
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set());

  const [selectedYear, setSelectedYear] = useState(CURRENT_YEAR);
  const [selectedPeriodType, setSelectedPeriodType] = useState(
    !isEdit ? initialPeriodType : "Monthly",
  );
  const [availablePeriods, setAvailablePeriods] = useState(() =>
    getAvailablePeriods(!isEdit ? initialPeriodType : "Monthly", CURRENT_YEAR),
  );
  const [selectedPeriod, setSelectedPeriod] = useState(
    selectedIssue?.time_Period ?? selectedIssue?.time_period ?? "",
  );

  useEffect(() => {
    if (!selectedDataset || datasets.length === 0) return;
    const matched = datasets.find((item) => item?.display_name === selectedDataset);
    if (!matched?.dataset_id) return;

    triggerGetDatasetElements(matched.dataset_id)
      .unwrap()
      .then((elements) => setDataElements(elements))
      .catch((err) => {
        console.error("Failed to fetch data elements:", err);
      });
  }, [selectedDataset, datasets, triggerGetDatasetElements]);

  const onChangeSelectedDataSet = async ({ selectedItem }: SelectEvent<string>) => {
    const datasetName = selectedItem ?? "";
    setSelectedDataset(datasetName);
    setSelectedDataElement("");
    if (!datasetName) {
      setDataElements([]);
      return;
    }

    const datasetId = datasets.find((item) => item?.display_name === datasetName)?.dataset_id;
    if (datasetId) {
      try {
        const data = await triggerGetDatasetElements(datasetId).unwrap();
        setDataElements(data);
      } catch (err) {
        console.error("Error encountered while fetching data elements:", err);
      }
    }
  };

  const handleSubmit = async (formData: IssuePayload) => {
    try {
      if (isEdit && selectedIssue?.issue_code) {
        await updateIssue({ id: selectedIssue.issue_code, body: formData }).unwrap();
      } else {
        await createIssue(formData).unwrap();
      }
      onClose();
    } catch (err) {
      console.error("Failed to save the issue:", err);
    }
  };

  const findAncestorsInTree = (
    nodes: OrgUnitNodeData[],
    targetName: string,
    path: OrgUnitNodeData[] = [],
  ): OrgUnitNodeData[] | null => {
    if (!nodes || !Array.isArray(nodes) || !targetName) return null;
    const targetLower = targetName.trim().toLowerCase();

    for (const node of nodes) {
      const currentPath = [...path, node];
      if (node.name?.trim().toLowerCase() === targetLower) {
        return currentPath;
      }
      if (node.children && Array.isArray(node.children)) {
        const found = findAncestorsInTree(node.children, targetName, currentPath);
        if (found) return found;
      }
    }

    return null;
  };

  const nodeMatchesSearch = useCallback((node: OrgUnitNodeData, term: string): boolean => {
    if (node.name.toLowerCase().includes(term.toLowerCase())) {
      return true;
    }
    if (node.children) {
      return node.children.some((child) => nodeMatchesSearch(child, term));
    }
    return false;
  }, []);

  const handleSelect = (name: string) => {
    setSelectedOrgUnit((prev) => (prev === name ? "" : name));
    setOrgSearchTerm("");
    setTimeout(() => setIsOrgExpanded(false), 150);
  };

  const handleToggleNode = useCallback((nodeId: string, isExpanded: boolean) => {
    setExpandedNodes((prev) => {
      const next = new Set(prev);
      if (isExpanded) {
        next.add(nodeId);
      } else {
        next.delete(nodeId);
      }
      return next;
    });
  }, []);

  const renderRecursive = (nodes: OrgUnitNodeData[], idPrefix = "modal-org") => {
    if (!nodes || !Array.isArray(nodes)) return [];

    return nodes
      .filter((node) => !orgSearchTerm || nodeMatchesSearch(node, orgSearchTerm))
      .map((node) => (
        <OrgUnitNode
          key={node.id}
          node={node}
          searchTerm={orgSearchTerm}
          selectedOrgUnit={selectedOrgUnit}
          expandedNodes={expandedNodes}
          onSelect={handleSelect}
          onToggleNode={handleToggleNode}
          renderRecursive={renderRecursive}
          idPrefix={idPrefix}
        />
      ));
  };

  const onChangeYear = (_event: unknown, { value }: { value: string | number }) => {
    const paramYear = Number(value) || CURRENT_YEAR;
    setSelectedYear(paramYear);
    setAvailablePeriods(getAvailablePeriods(selectedPeriodType, paramYear));
    setSelectedPeriod("");
  };

  const onChangePeriodType = ({ selectedItem }: SelectEvent<string>) => {
    const paramPeriodType = selectedItem ?? "Monthly";
    setSelectedPeriodType(paramPeriodType);
    setAvailablePeriods(getAvailablePeriods(paramPeriodType, selectedYear));
    setSelectedPeriod("");
  };

  const getResolvedRegionAndDistrict = () => {
    const ancestors = findAncestorsInTree(orgUnits, selectedOrgUnit);
    let region = selectedIssue?.region ?? "";
    let district = selectedIssue?.district ?? "";

    if (ancestors && ancestors.length > 1) {
      region = ancestors[1]?.name ?? region;
    }
    if (ancestors && ancestors.length > 2) {
      district = ancestors[2]?.name ?? district;
    }

    return { region, district };
  };

  return (
    <Modal
      aria-label="issue-modal"
      open
      modalHeading={
        isEdit && selectedIssue
          ? `Edit Issue: ${selectedIssue.issue_code}`
          : "Register a New Issue"
      }
      primaryButtonText={isCreating || isUpdating ? "Saving..." : "Submit Issue"}
      secondaryButtonText="Cancel"
      primaryButtonDisabled={!description.trim() || isCreating || isUpdating}
      onRequestClose={onClose}
      onRequestSubmit={() => {
        const { region, district } = getResolvedRegionAndDistrict();
        const actor = user?.email || user?.username || "";
        handleSubmit({
          dataset: selectedDataset,
          data_element: selectedDataElement,
          org_unit: selectedOrgUnit,
          region,
          district,
          issue: description.trim(),
          issue_type: selectedIssueType,
          reported_by: isEdit ? selectedIssue?.reported_by || actor : actor,
          updated_by: isEdit ? actor : "",
          priority,
          severity,
          time_period: selectedPeriod,
          time_Period: selectedPeriod,
        });
      }}
    >
      <p className="issue-modal-description">
        Register or update a data quality issue, including the affected organisation unit,
        dataset, reporting period, priority, and severity.
      </p>

      <div className="issue-modal-form">
        <div className="issue-modal-field">
          <p className="cds--label">Organisation Unit</p>
          <button
            type="button"
            className="issue-org-selector-trigger"
            onClick={() => setIsOrgExpanded(!isOrgExpanded)}
            aria-expanded={isOrgExpanded}
          >
            <span
              className={
                selectedOrgUnit
                  ? "issue-org-selector-value"
                  : "issue-org-selector-placeholder"
              }
            >
              {selectedOrgUnit || "Select Organisation Unit"}
            </span>
            {isOrgExpanded ? <ChevronUp /> : <ChevronDown />}
          </button>

          {isOrgExpanded && (
            <div className="issue-org-selector-dropdown">
              <Search
                labelText="Search organisation units"
                size="md"
                id="org-search-stable"
                placeholder="Search for org unit..."
                value={orgSearchTerm}
                onChange={(e) => setOrgSearchTerm(e.target.value)}
              />
              <div className="issue-org-selector-tree">
                <TreeView label="Org Units" hideLabel>
                  {renderRecursive(orgUnits, "modal-org")}
                </TreeView>
              </div>
            </div>
          )}
        </div>

        <div className="issue-modal-field">
          <ComboBox
            allowCustomValue
            autoAlign
            id="data-set-combobox"
            onChange={onChangeSelectedDataSet}
            items={datasets.map((item) => item?.display_name).filter(Boolean)}
            titleText="Dataset"
            placeholder="Select dataset"
            selectedItem={selectedDataset}
          />
        </div>

        <div className="issue-modal-field">
          <ComboBox
            allowCustomValue
            autoAlign
            id="data-element-combobox"
            onChange={({ selectedItem }: SelectEvent<string>) =>
              setSelectedDataElement(selectedItem ?? "")
            }
            items={dataElements.map((item) => item?.data_element_short_name).filter(Boolean)}
            titleText="Data Element"
            placeholder="Select data element"
            selectedItem={selectedDataElement}
          />
        </div>

        <div className="issue-modal-grid-2">
          <ComboBox
            allowCustomValue
            autoAlign
            id="period-type-combobox"
            onChange={onChangePeriodType}
            items={periodType.map((item) => item.label)}
            titleText="Period Type"
            placeholder="Select period type"
            selectedItem={selectedPeriodType}
          />

          <NumberInput
            defaultValue={CURRENT_YEAR}
            id="period-year-input"
            invalidText="Input is not a valid year"
            label="Year"
            locale="en"
            max={CURRENT_YEAR}
            min={1900}
            size="md"
            step={1}
            value={selectedYear}
            onChange={onChangeYear}
          />
        </div>

        <div className="issue-modal-field">
          <ComboBox
            allowCustomValue
            autoAlign
            id="period-element-combobox"
            onChange={({ selectedItem }: SelectEvent<string>) =>
              setSelectedPeriod(selectedItem ?? "")
            }
            items={availablePeriods.map((item) => item.label)}
            titleText="Reporting Period"
            placeholder="Select reporting period"
            selectedItem={selectedPeriod}
          />
        </div>

        <div className="issue-modal-field">
          <ComboBox
            allowCustomValue
            autoAlign
            id="issue-type-combobox"
            onChange={({ selectedItem }: SelectEvent<string>) =>
              setSelectedIssueType(selectedItem ?? "")
            }
            items={IssueTypes}
            titleText="Issue Type"
            placeholder="Select issue type"
            selectedItem={selectedIssueType}
          />
        </div>

        <div className="issue-modal-grid-2">
          <ComboBox
            allowCustomValue
            autoAlign
            id="priority-combobox"
            onChange={({ selectedItem }: SelectEvent<string>) =>
              setPriority(selectedItem ?? "")
            }
            items={Priority}
            titleText="Priority"
            placeholder="Select priority"
            selectedItem={priority}
          />

          <ComboBox
            allowCustomValue
            autoAlign
            id="severity-combobox"
            onChange={({ selectedItem }: SelectEvent<string>) =>
              setSeverity(selectedItem ?? "")
            }
            items={Severity}
            titleText="Severity"
            placeholder="Select severity"
            selectedItem={severity}
          />
        </div>

        <div className="issue-modal-field">
          <TextArea
            id="issue-text-area"
            labelText="Issue Description"
            placeholder="Describe the data quality anomaly and any known causes..."
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={5}
          />
        </div>
      </div>
    </Modal>
  );
};
