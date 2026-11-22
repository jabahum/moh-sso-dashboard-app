import React, { useEffect, useMemo, useState } from "react";
import {
  DataTable,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@carbon/react";
import { DataTablePagination } from "@moh-sso/ui";

interface ListProps {
  columns: Array<{ key: string; header: string }>;
  data: Array<Record<string, unknown>>;
  totalItems?: number;
  currentPage?: number;
  currentPageSize?: number;
  onPageChange?: (page: number, pageSize: number) => void;
}

const DataList: React.FC<ListProps> = ({
  columns,
  data,
  totalItems,
  currentPage,
  currentPageSize,
  onPageChange,
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
  const rowsWithId = useMemo(
    () =>
      paginatedData.map((row, index) => ({
        ...row,
        id: String(row.id ?? index),
      })),
    [paginatedData],
  );

  return (
    <>
      <DataTable rows={rowsWithId} headers={columns}>
        {({ rows, headers, getTableProps, getHeaderProps, getRowProps }) => (
          <Table {...getTableProps()}>
            <TableHead>
              <TableRow>
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
                const { key: rowKey, ...rowProps } = getRowProps({ row });
                return (
                  <TableRow key={rowKey} {...rowProps}>
                    {row.cells.map((cell) => (
                      <TableCell key={cell.id}>{String(cell.value ?? "-")}</TableCell>
                    ))}
                  </TableRow>
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

export default DataList;
