import React, { useEffect, useMemo, useState } from "react";
import {
  DataTable,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  TableExpandHeader,
  TableExpandRow,
  TableExpandedRow,
  TableSelectAll,
  TableSelectRow,
} from "@carbon/react";
import { DataTablePagination, TableStatusTag } from "@moh-sso/ui";
import IssueDetail, {
  mapIssueStatusToTagKind,
} from "../pages/issue-detail/issue-detail.component";
import type { Issue } from "../types";

export type IssueTableColumn = {
  key: string;
  header: string;
};

export interface IssuesDataTableProps {
  columns: IssueTableColumn[];
  data: Issue[];
  closeView: () => void;
  handleIssueClick: (row: { id?: string }) => void;
  totalItems?: number;
  currentPage?: number;
  currentPageSize?: number;
  onPageChange?: (page: number, pageSize: number) => void;
  selectedRowIds?: string[];
  onSelectRow?: (rowId: string, checked: boolean) => void;
  onSelectAll?: (checked: boolean) => void;
}

export const IssuesDataTable: React.FC<IssuesDataTableProps> = ({
  columns,
  data,
  handleIssueClick,
  closeView,
  totalItems,
  currentPage,
  currentPageSize,
  onPageChange,
  selectedRowIds,
  onSelectRow,
  onSelectAll,
}) => {
  const [localPage, setLocalPage] = useState(1);
  const [localPageSize, setLocalPageSize] = useState(10);

  const isServerSide = onPageChange !== undefined;

  const page = isServerSide ? (currentPage ?? 1) : localPage;
  const pageSize = isServerSide ? (currentPageSize ?? 10) : localPageSize;

  const paginatedData = useMemo(() => {
    if (isServerSide) {
      return data;
    }
    const start = (page - 1) * pageSize;
    const end = start + pageSize;
    return data.slice(start, end);
  }, [data, page, pageSize, isServerSide]);

  useEffect(() => {
    if (isServerSide) return;
    const lastPage = Math.ceil(data.length / pageSize) || 1;
    if (page > lastPage) {
      setLocalPage(lastPage);
    }
  }, [data.length, pageSize, page, isServerSide]);

  const total = isServerSide ? (totalItems ?? 0) : data.length;

  const isAllSelected =
    Boolean(onSelectAll) &&
    paginatedData.length > 0 &&
    paginatedData.every((item) =>
      selectedRowIds?.includes(String(item.id ?? item.issue_id)),
    );

  const tableRows = useMemo(
    () =>
      paginatedData.map((item) => ({
        ...item,
        id: String(item.id ?? item.issue_id),
      })),
    [paginatedData],
  );

  return (
    <>
      <DataTable rows={tableRows} headers={columns}>
        {({
          rows,
          headers,
          getTableProps,
          getHeaderProps,
          getRowProps,
          getExpandHeaderProps,
        }) => (
          <Table {...getTableProps()}>
            <TableHead>
              <TableRow>
                <TableExpandHeader {...getExpandHeaderProps()} />
                {onSelectAll && (
                  <TableSelectAll
                    id="select-all-issues"
                    name="select-all-issues"
                    aria-label="Select all issues"
                    checked={isAllSelected}
                    onSelect={(e: React.SyntheticEvent<HTMLInputElement>) =>
                      onSelectAll(e.currentTarget.checked)
                    }
                  />
                )}
                {headers.map((header) => {
                  const { key: headerKey, ...headerProps } = getHeaderProps({ header });
                  return (
                    <TableHeader key={headerKey} {...headerProps}>
                      {header.header}
                    </TableHeader>
                  );
                })}
              </TableRow>
            </TableHead>
            <TableBody>
              {rows.map((row) => {
                const issue = data.find(
                  (item) => String(item.id ?? item.issue_id) === String(row.id),
                );
                const { key: rowKey, ...rowProps } = getRowProps({ row });
                const isSelected = Boolean(selectedRowIds?.includes(String(row.id)));

                return (
                  <React.Fragment key={row.id}>
                    <TableExpandRow key={rowKey} {...rowProps}>
                      {onSelectRow && (
                        <TableSelectRow
                          id={`select-issue-${row.id}`}
                          name={`select-issue-${row.id}`}
                          aria-label={`Select issue ${row.id}`}
                          checked={isSelected}
                          onSelect={(e: React.SyntheticEvent<HTMLInputElement>) =>
                            onSelectRow(String(row.id), e.currentTarget.checked)
                          }
                        />
                      )}
                      {row.cells.map((cell) => {
                        if (cell.info.header === "issue") {
                          return (
                            <TableCell key={cell.id}>
                              <button
                                type="button"
                                className="issue-clickable-cell"
                                onClick={() => handleIssueClick(row)}
                              >
                                {cell.value || "—"}
                              </button>
                            </TableCell>
                          );
                        }
                        if (cell.info.header === "status") {
                          return (
                            <TableCell key={cell.id} className="issue-status-cell">
                              <TableStatusTag
                                status={String(cell.value || "OPEN")}
                                mapStatusToKind={mapIssueStatusToTagKind}
                              />
                            </TableCell>
                          );
                        }
                        return <TableCell key={cell.id}>{cell.value || "—"}</TableCell>;
                      })}
                    </TableExpandRow>
                    <TableExpandedRow colSpan={headers.length + (onSelectRow ? 2 : 1)}>
                      {row.isExpanded && issue && (
                        <section aria-label={`Details for issue ${issue.issue_code}`}>
                          <IssueDetail
                            selectedIssue={issue}
                            goToBack={closeView}
                            showBack={false}
                          />
                        </section>
                      )}
                    </TableExpandedRow>
                  </React.Fragment>
                );
              })}
            </TableBody>
          </Table>
        )}
      </DataTable>

      <DataTablePagination
        page={page}
        pageSize={pageSize}
        totalItems={total}
        onChange={({ page: newPage, pageSize: newPageSize }) => {
          if (isServerSide) {
            onPageChange(newPage, newPageSize);
          } else {
            setLocalPage(newPage);
            setLocalPageSize(newPageSize);
          }
        }}
      />
    </>
  );
};

export default IssuesDataTable;
