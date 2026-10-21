import { Button, Modal, TextArea, Tile } from "@carbon/react";
import { ArrowLeft } from "@carbon/react/icons";
import React, { useState } from "react";
import { useSelector } from "react-redux";

import {
  useCreateTransactionMutation,
  useGetIssuesQuery,
  useGetTransactionsQuery,
} from "../../api";
import { PERMISSIONS, PermissionGuard, selectUser } from "@moh-sso/auth";

import "./issue-detail.scss";
import { formatReportingPeriod, type Issue } from "../issue-tracker.component.tsx";
import { IssueModal } from "../../component/issue-modal.component.tsx";
import { AssignModal } from "../../component/assign-modal.component.tsx";

type ModalMode = "resolve" | "close" | "comment" | null;

type IssueDetailProps = {
  selectedIssue: Issue;
  goToBack: () => void;
  showBack?: boolean;
};

const IssueDetail = ({
  selectedIssue: initialIssue,
  goToBack,
  showBack = true,
}: IssueDetailProps) => {
  const { data: latestIssues } = useGetIssuesQuery();

  const selectedIssue =
    latestIssues?.items?.find((issueItem: Issue) => issueItem?.issue_code === initialIssue?.issue_code) ??
    initialIssue;

  const [comment, setComment] = useState("");
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [isAssignModalOpen, setIsAssignModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<ModalMode>(null);
  const [isViewModalResolution, setIsViewModalResolution] = useState(false);

  const { data: transactions, isLoading: isLoadingTransactions } = useGetTransactionsQuery(
    selectedIssue?.issue_code,
    {
      skip: !selectedIssue?.issue_code,
    },
  );

  const [createTransaction, { isLoading: isResolving }] = useCreateTransactionMutation();

  const user = useSelector(selectUser);

  const handleTextChange = (event: React.ChangeEvent<HTMLTextAreaElement>) => {
    setComment(event.target.value);
  };

  const openActionModal = (mode: Exclude<ModalMode, null>) => {
    setModalMode(mode);
    setIsViewModalResolution(true);
    setComment("");
  };

  const closeActionModal = () => {
    if (isResolving) {
      return;
    }

    setIsViewModalResolution(false);
    setModalMode(null);
    setComment("");
  };

  const handleUpdateAction = async () => {
    const trimmedComment = comment.trim();

    if (!trimmedComment || !modalMode || !selectedIssue?.issue_code) {
      return;
    }

    const status =
      modalMode === "resolve"
        ? "resolved"
        : modalMode === "close"
          ? "closed"
          : selectedIssue.status;

    const payload = {
      id: selectedIssue.issue_code,
      body: {
        resolution_action: trimmedComment,
        resolved_by: user?.email || user?.username || "",
        status,
      },
    };

    try {
      await createTransaction(payload).unwrap();

      setIsViewModalResolution(false);
      setModalMode(null);
      setComment("");
    } catch (error) {
      console.error(`Failed to ${modalMode} issue:`, error);
    }
  };

  const getPrimaryButtonText = () => {
    if (isResolving) {
      return "Processing...";
    }

    switch (modalMode) {
      case "resolve":
        return "Confirm Resolution";
      case "close":
        return "Confirm Closure";
      case "comment":
        return "Add Comment";
      default:
        return "Submit";
    }
  };

  const getModalHeading = () => {
    switch (modalMode) {
      case "resolve":
        return "Resolve Issue";
      case "close":
        return "Close Issue";
      case "comment":
        return "Add Comment";
      default:
        return "Update Issue";
    }
  };

  const getModalDescription = () => {
    switch (modalMode) {
      case "resolve":
        return "Describe how this issue was resolved.";
      case "close":
        return "Provide a reason for closing this issue.";
      case "comment":
        return "Enter your comment below.";
      default:
        return "";
    }
  };

  const isFinalStatus = selectedIssue?.status?.toLowerCase() === "closed" || selectedIssue?.status?.toLowerCase() === "resolved";

  const renderActionModal = () => {
    if (!isViewModalResolution || !modalMode) {
      return null;
    }

    const modal = (
      <Modal
        open
        primaryButtonText={getPrimaryButtonText()}
        secondaryButtonText="Cancel"
        modalHeading={getModalHeading()}
        onRequestClose={closeActionModal}
        onRequestSubmit={handleUpdateAction}
        primaryButtonDisabled={!comment.trim() || isResolving || !selectedIssue?.issue_code}
      >
        <p style={{ marginBottom: "1rem" }}>{getModalDescription()}</p>

        <TextArea
          data-modal-primary-focus
          id={`issue-${modalMode}-comment`}
          labelText={
            modalMode === "comment"
              ? "Comment"
              : modalMode === "resolve"
                ? "Resolution details"
                : "Closure reason"
          }
          hideLabel
          value={comment}
          onChange={handleTextChange}
          rows={4}
          disabled={isResolving}
        />
      </Modal>
    );

    switch (modalMode) {
      case "comment":
        return (
          <PermissionGuard permission={PERMISSIONS.issueTrackerComment}>{modal}</PermissionGuard>
        );

      case "resolve":
        return (
          <PermissionGuard permission={PERMISSIONS.issueTrackerManage}>{modal}</PermissionGuard>
        );

      case "close":
        return (
          <PermissionGuard permission={PERMISSIONS.issueTrackerClose}>{modal}</PermissionGuard>
        );

      default:
        return null;
    }
  };

  return (
    <PermissionGuard permission={PERMISSIONS.issueTrackerRead}>
      <>
        {showBack && (
          <div style={{ marginBottom: "1rem" }}>
            <Button kind="tertiary" size="lg" renderIcon={ArrowLeft} onClick={goToBack}>
              Back to Issues
            </Button>
          </div>
        )}

        <div className="issue-detail-container">
          <div style={{ width: "65%" }}>
            <Tile className="issue-tile">
              <div style={{ padding: "0.5rem" }}>
                <div className="issue-label">Reported Issue: {selectedIssue?.issue_code}</div>

                <p>
                  <span className="issue-p">
                    <strong>Issue Type:</strong> {selectedIssue?.issue_type}
                  </span>

                  <span className="issue-p">
                    <strong>Priority:</strong> {selectedIssue?.priority ?? "<<Not set>>"}
                  </span>

                  <span className="issue-p">
                    <strong>Severity:</strong> {selectedIssue?.severity ?? "<<Not set>>"}
                  </span>
                </p>

                <p className="issue-p-top">
                  <strong>Organization Unit:</strong> {selectedIssue?.org_unit}
                </p>

                {selectedIssue?.region && (
                  <p className="issue-p-top">
                    <strong>Region:</strong> {selectedIssue.region}
                  </p>
                )}

                {selectedIssue?.district && (
                  <p className="issue-p-top">
                    <strong>District:</strong> {selectedIssue.district}
                  </p>
                )}

                <p className="issue-p-top">
                  <strong>Dataset:</strong> {selectedIssue?.dataset}
                </p>

                <p className="issue-p-top">
                  <strong>Data Element:</strong> {selectedIssue?.data_element}
                </p>

                <p className="issue-p-top">
                  <strong>Reporting Period:</strong>{" "}
                  {formatReportingPeriod(selectedIssue?.time_Period ?? selectedIssue?.time_period) || "<<Not set>>"}
                </p>

                <p className="issue-p-top">
                  <strong>Description:</strong>
                </p>

                <p>{selectedIssue?.issue}</p>
              </div>
            </Tile>

            <Tile className="issue-tile issue-history-container">
              <div>
                <div className="issue-label">Issue History / Transactions</div>

                {isLoadingTransactions ? (
                  <p>Loading history...</p>
                ) : (
                  <div className="transaction-list">
                    {(transactions?.length ?? 0) > 0 ? (
                      transactions?.map((transaction, index) => (
                        <div
                          key={transaction?.id ?? `${transaction?.resolution_date}-${index}`}
                          className="transaction-item"
                          style={{
                            marginBottom: "1rem",
                            fontSize: "0.875rem",
                          }}
                        >
                          <div style={{ fontWeight: "bold" }}>
                            {transaction?.resolution_date || "N/A"} - {transaction?.resolved_by || "System / Unknown"}
                          </div>

                          <div style={{ color: "#525252" }}>
                            {transaction?.resolution_action || "No comment provided"}
                          </div>

                          {transaction?.assigned_to && (
                            <div style={{ marginTop: "0.25rem", fontSize: "0.8125rem", color: "#0f62fe" }}>
                              <strong>Assigned To:</strong> {transaction.assigned_to}
                            </div>
                          )}
                        </div>
                      ))
                    ) : (
                      <p
                        style={{
                          fontStyle: "italic",
                          color: "#6f6f6f",
                        }}
                      >
                        No transactions found for this issue.
                      </p>
                    )}
                  </div>
                )}
              </div>
            </Tile>
          </div>

          <div className="issue-reporter-details" style={{ width: "30%" }}>
            <Tile className="issue-tile">
              <div style={{ padding: "0.5rem" }}>
                <p className="issue-p-top">
                  <strong>Issue Status:</strong> <span className="issue-status">{selectedIssue?.status} </span>
                </p>

                <p>
                  <strong>Assigned To:</strong>{" "}
                  {selectedIssue?.assigned_to ? (
                    <span style={{ color: "#0f62fe", fontWeight: 600 }}>{selectedIssue.assigned_to}</span>
                  ) : (
                    <em style={{ color: "#6f6f6f" }}>Unassigned</em>
                  )}
                </p>

                <p>
                  <strong>Reported By:</strong> {selectedIssue?.reported_by}
                </p>

                <p>
                  <strong>Date:</strong> {selectedIssue?.date_reported}
                </p>
              </div>
            </Tile>

            <Tile className="issue-tile issue-actions-container">
              {!isFinalStatus && (
                <PermissionGuard permission={PERMISSIONS.issueTrackerAssign}>
                  <Button
                    className="btn-issue btn-full-width"
                    kind="tertiary"
                    onClick={() => setIsAssignModalOpen(true)}
                  >
                    {selectedIssue?.assigned_to ? "Re-assign Issue" : "Assign Issue"}
                  </Button>
                </PermissionGuard>
              )}

              {!isFinalStatus && (
                <PermissionGuard permission={PERMISSIONS.issueTrackerWrite}>
                  <Button
                    className="btn-issue btn-full-width"
                    size="md"
                    kind="primary"
                    onClick={() => setIsEditModalOpen(true)}
                  >
                    Edit Issue
                  </Button>
                </PermissionGuard>
              )}

              <PermissionGuard permission={PERMISSIONS.issueTrackerComment}>
                <Button
                  className="btn-issue btn-full-width"
                  kind="secondary"
                  onClick={() => openActionModal("comment")}
                >
                  Add Comment
                </Button>
              </PermissionGuard>

              {!isFinalStatus && (
                <PermissionGuard permission={PERMISSIONS.issueTrackerManage}>
                  <Button
                    className="btn-issue custom-btn-success btn-full-width"
                    onClick={() => openActionModal("resolve")}
                  >
                    Resolve Issue
                  </Button>
                </PermissionGuard>
              )}

              {selectedIssue?.status?.toLowerCase() !== "closed" && (
                <PermissionGuard permission={PERMISSIONS.issueTrackerClose}>
                  <Button
                    className="btn-issue btn-full-width"
                    kind="danger--tertiary"
                    onClick={() => openActionModal("close")}
                  >
                    Close Issue
                  </Button>
                </PermissionGuard>
              )}
            </Tile>
          </div>
        </div>

        {renderActionModal()}

        <PermissionGuard permission={PERMISSIONS.issueTrackerWrite}>
          {isEditModalOpen && (
            <IssueModal onClose={() => setIsEditModalOpen(false)} selectedIssue={selectedIssue} />
          )}
        </PermissionGuard>

        <PermissionGuard permission={PERMISSIONS.issueTrackerAssign}>
          {isAssignModalOpen && selectedIssue?.issue_code && (
            <AssignModal
              issueCodes={[selectedIssue.issue_code]}
              isReassign={Boolean(selectedIssue?.assigned_to)}
              onClose={() => setIsAssignModalOpen(false)}
            />
          )}
        </PermissionGuard>
      </>
    </PermissionGuard>
  );
};

export default IssueDetail;
