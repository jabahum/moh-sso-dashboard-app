import { ContentSwitcher, Modal, Switch } from "@carbon/react";
import { useState } from "react";

import DataModal from "./data-model/DataModal.tsx";
import OrgUnitModal from "./orgunit/OrgUnitModal.tsx";
import PeriodModal from "./period/PeriodModal.tsx";
import "./general-modal.scss";
export default function GeneralModal({
  onClose,
  selectedData,
  onSaveData,
  selectedPeriods,
  onSavePeriods,
  selectedOrgUnits,
  selectedLevelOfCare,
  selectedOwnership,
  aggregationLevel,
  onSaveOrgUnits,
  updateTrigger,
}) {
  const [selectedDimension, setSelectedDimension] = useState("data");

  const onChangeContentSwitcher = (item) => {
    setSelectedDimension(item?.name);
  };

  return (
    <>
      <Modal
        open
        size="lg"
        passiveModal
        isFullWidth
        preventCloseOnClickOutside={true}
        hasScrollingContent={true}
        modalHeading="MAIN DIMENSIONS"
        onRequestClose={onClose}
      >
        <div className={`dimension-modal-container`}>
          <ContentSwitcher onChange={onChangeContentSwitcher} selectedIndex={0} size="md">
            <Switch name="data" text="Data" />
            <Switch name="period" text="Period" />
            <Switch name="orgunit" text="Organization Unit" />
          </ContentSwitcher>
        </div>

        <div className={`dimension-modal-selection`}>
          {selectedDimension === "data" && (
            <DataModal onClose={onClose} selected={selectedData} onSave={onSaveData} updateTrigger={updateTrigger} />
          )}
          {selectedDimension === "period" && (
            <PeriodModal onClose={onClose} selected={selectedPeriods} onSave={onSavePeriods} updateTrigger={updateTrigger} />
          )}
          {selectedDimension === "orgunit" && (
            <OrgUnitModal
              onClose={onClose}
              selected={selectedOrgUnits}
              selectedLevelOfCare={selectedLevelOfCare}
              selectedOwnership={selectedOwnership}
              aggregationLevel={aggregationLevel}
              onSave={onSaveOrgUnits}
              updateTrigger={updateTrigger}
            />
          )}
        </div>
      </Modal>
    </>
  );
}
