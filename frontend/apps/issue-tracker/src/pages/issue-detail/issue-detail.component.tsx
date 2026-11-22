import { Button, Modal, Tag, TextArea, Tile } from "@carbon/react";
import { ArrowLeft } from "@carbon/react/icons";
import React, { useState } from "react";
import { useSelector } from "react-redux";
import { TableStatusTag } from "@moh-sso/ui";
import { PERMISSIONS, PermissionGuard, selectUser } from "@moh-sso/auth";

import {
  useCreateTransactionMutation,
  useGetIssueByCodeQuery,
  useGetTransactionsQuery,
} from "../../api";
import type { Issue } from "../../types";
import { formatReportingPeriod } from "../../lib/periodUtils";
import { IssueModal } from "../../component/issue-modal.component";
import { AssignModal } from "../../component/assign-modal.component";

import "./issue-detail.scss";

type ModalMode = "resolve" | "close" | "comment" | null;

type IssueDetailProps = {
  selectedIssue: Issue;
  goToBack: () => void;
  showBack?: boolean;
};

export function mapIssueStatusToTagKind(status?: string | null) {
  const normalized = (status ?? "").trim().toLowerCase();
  if (["resolved", "closed", "done", "completed"].includes(normalized)) {
    return "green" as const;
  }
  if (["open", "new", "reopened"].includes(normalized)) {
    return "red" as const;
  }
  if (["in_progress", "in progress", "assigned", "pending"].includes(normalized)) {
    return "blue" as const;
  }
  return "gray" as const;
}

export function mapSeverityToTagKind(level?: string | null) {
  const normalized = (level ?? "").trim().toLowerCase();
  if (normalized === "critical") return "red" as const;
  if (normalized === "high") return "magenta" as const;
  if (normalized === "moderate" || normalized === "medium") return "warm-gray" as const;
  if (normalized === "low") return "blue" as const;
  return "gray" as const;
}

const IssueDetail = ({
  selectedIssue: initialIssue,
  goToBack,
  showBack = true,
}: IssueDetailProps) => {
  const issueCode = initialIssue?.issue_code ?? "";
  const { data: fetchedIssue } = useGetIssueByCodeQuery(issueCode, {
    skip: !issueCode,
  });

  const selectedIssue = fetchedIssue ?? initialIssue;

  const [comment, setComment] = useState("");
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [isAssignModalOpen, setIsAssignModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<ModalMode>(null);
  const [isViewModalResolution, setIsViewModalResolution] = useState(false);

  const { data: transactions = [], isLoading: isLoadingTransactions } =
    useGetTransactionsQuery(issueCode, {
      skip: !issueCode,
    });

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

    try {
      await createTransaction({
        id: selectedIssue.issue_code,
        body: {
          resolution_action: trimmedComment,
          resolved_by: user?.email || user?.username || "",
          status,
        },
      }).unwrap();

      setIsViewModalResolution(false);
      setModalMode(null);
      setComment("");
    } catch (error) {
      console.error(`Failed to ${modalMode} issue:`, error);
    }
  };

  const getPrimaryButtonText = () => {
    if (isResolving) return "Processing...";
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

  const normalizedStatus = (selectedIssue?.status ?? "").trim().toLowerCase();
  const isFinalStatus = normalizedStatus === "closed" || normalizedStatus === "resolved";
  const formattedPeriod =
    formatReportingPeriod(selectedIssue?.time_Period ?? selectedIssue?.time_period) || "—";

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
        <p className="issue-detail-modal-description">{getModalDescription()}</p>

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
      <div className={`issue-detail-wrapper ${showBack ? "" : "issue-detail-wrapper--inline"}`}>
        {showBack && (
          <div className="issue-detail-back-bar">
            <Button kind="tertiary" size="md" renderIcon={ArrowLeft} onClick={goToBack}>
              Back to Issues
            </Button>
          </div>
        )}

        <div className="issue-detail-container">
          <div className="issue-detail-main">
            <Tile className="issue-tile">
              <div className="issue-detail-header">
                <div>
                  <h4 className="issue-label">
                    Reported Issue: {selectedIssue?.issue_code || "—"}
                  </h4>
                  {selectedIssue?.program && (
                    <span className="issue-detail-program-subtitle">
                      Program: {selectedIssue.program}
                    </span>
                  )}
                </div>

                <div className="issue-detail-badges">
                  {selectedIssue?.issue_type && (
                    <Tag type="cool-gray" size="sm">
                      {selectedIssue.issue_type}
                    </Tag>
                  )}
                  {selectedIssue?.priority && (
                    <Tag type={mapSeverityToTagKind(selectedIssue.priority)} size="sm">
                      Priority: {selectedIssue.priority}
                    </Tag>
                  )}
                  {selectedIssue?.severity && (
                    <Tag type={mapSeverityToTagKind(selectedIssue.severity)} size="sm">
                      Severity: {selectedIssue.severity}
                    </Tag>
                  )}
                </div>
              </div>

              <dl className="issue-detail-meta-grid">
                <div className="issue-detail-meta-item">
                  <dt>Organisation Unit</dt>
                  <dd>{selectedIssue?.org_unit || "—"}</dd>
                </div>
                <div className="issue-detail-meta-item">
                  <dt>District</dt>
                  <dd>{selectedIssue?.district || "—"}</dd>
                </div>
                <div className="issue-detail-meta-item">
                  <dt>Region</dt>
                  <dd>{selectedIssue?.region || "—"}</dd>
                </div>
                <div className="issue-detail-meta-item">
                  <dt>Dataset</dt>
                  <dd>{selectedIssue?.dataset || "—"}</dd>
                </div>
                <div className="issue-detail-meta-item">
                  <dt>Data Element</dt>
                  <dd>{selectedIssue?.data_element || "—"}</dd>
                </div>
                <div className="issue-detail-meta-item">
                  <dt>Reporting Period</dt>
                  <dd>{formattedPeriod}</dd>
                </div>
                <div className="issue-detail-meta-item">
                  <dt>Issue Type</dt>
                  <dd>{selectedIssue?.issue_type || "—"}</dd>
                </div>
                <div className="issue-detail-meta-item">
                  <dt>Priority</dt>
                  <dd>{selectedIssue?.priority || "—"}</dd>
                </div>
                <div className="issue-detail-meta-item">
                  <dt>Severity</dt>
                  <dd>{selectedIssue?.severity || "—"}</dd>
                </div>
              </dl>

              <div className="issue-detail-description">
                <span className="issue-detail-section-label">Description</span>
                <p className="issue-detail-description-text">
                  {selectedIssue?.issue || "No description provided."}
                </p>
              </div>
            </Tile>

            <Tile className="issue-tile issue-history-container">
              <h5 className="issue-detail-sub-heading">Issue History / Transactions</h5>

              {isLoadingTransactions ? (
                <p className="issue-transaction-empty">Loading history...</p>
              ) : transactions.length > 0 ? (
                <div className="transaction-list">
                  {transactions.map((transaction, index) => (
                    <div
                      key={transaction?.id ?? `${transaction?.resolution_date}-${index}`}
                      className="transaction-item"
                    >
                      <div className="transaction-item-header">
                        <span className="transaction-author">
                          {transaction?.resolved_by || "System"}
                        </span>
                        <span className="transaction-date">
                          {transaction?.resolution_date || "N/A"}
                        </span>
                      </div>

                      <div className="transaction-action">
                        {transaction?.resolution_action || "No comment provided"}
                      </div>

                      {transaction?.assigned_to && (
                        <div className="transaction-assigned">
                          <strong>Assigned To:</strong> {transaction.assigned_to}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              ) : (
                <p className="issue-transaction-empty">
                  No transactions recorded for this issue yet.
                </p>
              )}
            </Tile>
          </div>

          <div className="issue-reporter-details">
            <Tile className="issue-tile">
              <h5 className="issue-detail-sub-heading">Status &amp; Ownership</h5>
              <dl className="issue-detail-sidebar-list">
                <div className="issue-detail-sidebar-row">
                  <dt>Issue Status</dt>
                  <dd>
                    <TableStatusTag
                      status={selectedIssue?.status || "OPEN"}
                      mapStatusToKind={mapIssueStatusToTagKind}
                    />
                  </dd>
                </div>
                <div className="issue-detail-sidebar-row">
                  <dt>Assigned To</dt>
                  <dd>
                    {selectedIssue?.assigned_to ? (
                      <span className="issue-assignee-text">{selectedIssue.assigned_to}</span>
                    ) : (
                      <span className="issue-unassigned-text">Unassigned</span>
                    )}
                  </dd>
                </div>
                <div className="issue-detail-sidebar-row">
                  <dt>Reported By</dt>
                  <dd>{selectedIssue?.reported_by || "—"}</dd>
                </div>
                <div className="issue-detail-sidebar-row">
                  <dt>Date Reported</dt>
                  <dd>{selectedIssue?.date_reported || "—"}</dd>
                </div>
                {selectedIssue?.updated_date && (
                  <div className="issue-detail-sidebar-row">
                    <dt>Last Updated</dt>
                    <dd>{selectedIssue.updated_date}</dd>
                  </div>
                )}
              </dl>
            </Tile>

            <Tile className="issue-tile issue-actions-container">
              <h5 className="issue-detail-sub-heading">Actions</h5>
              <div className="issue-actions-stack">
                {!isFinalStatus && (
                  <PermissionGuard permission={PERMISSIONS.issueTrackerAssign}>
                    <Button
                      className="btn-issue"
                      size="md"
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
                      className="btn-issue"
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
                    className="btn-issue"
                    size="md"
                    kind="secondary"
                    onClick={() => openActionModal("comment")}
                  >
                    Add Comment
                  </Button>
                </PermissionGuard>

                {!isFinalStatus && (
                  <PermissionGuard permission={PERMISSIONS.issueTrackerManage}>
                    <Button
                      className="btn-issue"
                      size="md"
                      kind="primary"
                      onClick={() => openActionModal("resolve")}
                    >
                      Resolve Issue
                    </Button>
                  </PermissionGuard>
                )}

                {normalizedStatus !== "closed" && (
                  <PermissionGuard permission={PERMISSIONS.issueTrackerClose}>
                    <Button
                      className="btn-issue"
                      size="md"
                      kind="danger--tertiary"
                      onClick={() => openActionModal("close")}
                    >
                      Close Issue
                    </Button>
                  </PermissionGuard>
                )}
              </div>
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
      </div>
    </PermissionGuard>
  );
};

export default IssueDetail;
