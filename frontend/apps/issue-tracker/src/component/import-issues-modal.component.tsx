import { useState, useCallback } from "react";
import * as XLSX from "xlsx";
import {
  Button,
  Modal,
  FileUploaderDropContainer,
  FileUploaderItem,
  InlineLoading,
  InlineNotification,
  Stack,
  Table,
  TableHead,
  TableRow,
  TableHeader,
  TableBody,
  TableCell,
} from "@carbon/react";
import { Download } from "@carbon/react/icons";
import { useCreateIssueMutation } from "../api";
import {
  IMPORT_TEMPLATE_EXAMPLE_ROW,
  IMPORT_TEMPLATE_HEADERS,
} from "../lib/constants";
import type { IssuePayload } from "../types";

type RowData = Record<string, unknown>;

type ImportSummary = {
  total: number;
  success: number;
  failed: number;
  errors: { row: number; message: string }[];
};

export const ImportIssuesModal = ({ onClose }: { onClose: () => void }) => {
  const [file, setFile] = useState<File | null>(null);
  const [parsedRows, setParsedRows] = useState<RowData[]>([]);
  const [isImporting, setIsImporting] = useState(false);
  const [summary, setSummary] = useState<ImportSummary | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [createIssue] = useCreateIssueMutation();

  const parseExcelFile = useCallback((excelFile: File) => {
    const reader = new FileReader();

    reader.onerror = () => {
      setError("Failed to read the Excel file.");
    };

    reader.onload = (event) => {
      try {
        const binary = event.target?.result;
        if (!binary) {
          setError("Invalid Excel file.");
          return;
        }

        const workbook = XLSX.read(binary, { type: "binary" });
        const firstSheetName = workbook.SheetNames[0];
        if (!firstSheetName) {
          setError("Excel file has no sheets.");
          return;
        }

        const firstSheet = workbook.Sheets[firstSheetName];
        const rows = XLSX.utils
          .sheet_to_json<RowData>(firstSheet, { defval: "" })
          .filter((row) =>
            Object.values(row).some((value) => String(value ?? "").trim() !== ""),
          );

        if (!rows.length) {
          setError("The Excel sheet contains no data rows.");
          return;
        }

        setParsedRows(rows);
      } catch (err) {
        setError("Failed to parse Excel file: " + (err as Error).message);
      }
    };

    reader.readAsBinaryString(excelFile);
  }, []);

  const handleDrop = useCallback(
    (event: { target?: { files?: FileList | null } }) => {
      const selectedFile = event.target?.files?.[0];
      if (!selectedFile) return;

      const ext = selectedFile.name.split(".").pop()?.toLowerCase();
      if (ext !== "xlsx" && ext !== "xls") {
        setError("Please upload an Excel file (.xlsx or .xls).");
        return;
      }

      setFile(selectedFile);
      setError(null);
      setSummary(null);
      parseExcelFile(selectedFile);
    },
    [parseExcelFile],
  );

  const mapRowToPayload = (row: RowData): IssuePayload => {
    const period = String(row["time_period"] ?? row["time_Period"] ?? "").trim();
    return {
      program: String(row["program"] ?? "").trim() || undefined,
      dataset: String(row["dataset"] ?? "").trim(),
      data_element: String(row["data_element"] ?? "").trim(),
      org_unit: String(row["org_unit"] ?? "").trim(),
      region: String(row["region"] ?? "").trim(),
      district: String(row["district"] ?? "").trim(),
      issue: String(row["issue"] ?? "").trim(),
      issue_type: String(row["issue_type"] ?? "").trim(),
      priority: String(row["priority"] ?? "").trim(),
      severity: String(row["severity"] ?? "").trim(),
      time_period: period,
      time_Period: period,
    };
  };

  const handleImport = async () => {
    if (!parsedRows.length) return;

    setIsImporting(true);
    setSummary(null);
    setError(null);

    const result: ImportSummary = {
      total: parsedRows.length,
      success: 0,
      failed: 0,
      errors: [],
    };

    for (let i = 0; i < parsedRows.length; i++) {
      try {
        const payload = mapRowToPayload(parsedRows[i]);
        await createIssue(payload).unwrap();
        result.success++;
      } catch (err) {
        result.failed++;
        result.errors.push({
          row: i + 2,
          message: (err as Error)?.message || "Unknown error",
        });
      }
    }

    setSummary(result);
    setIsImporting(false);
  };

  const downloadTemplate = () => {
    const wb = XLSX.utils.book_new();
    const ws = XLSX.utils.aoa_to_sheet([
      IMPORT_TEMPLATE_HEADERS,
      IMPORT_TEMPLATE_EXAMPLE_ROW,
    ]);

    ws["!cols"] = IMPORT_TEMPLATE_HEADERS.map(() => ({ wch: 22 }));

    XLSX.utils.book_append_sheet(wb, ws, "Issues Template");
    XLSX.writeFile(wb, "issue-import-template.xlsx");
  };

  const previewHeaders = parsedRows.length > 0 ? Object.keys(parsedRows[0]) : [];
  const previewRows = parsedRows.slice(0, 10);

  const resetFile = () => {
    setFile(null);
    setParsedRows([]);
    setSummary(null);
    setError(null);
  };

  return (
    <Modal
      open
      modalHeading="Import Issues from Excel"
      primaryButtonText={summary ? "Close" : "Import"}
      secondaryButtonText={summary ? "Import Another" : "Cancel"}
      primaryButtonDisabled={(!summary && parsedRows.length === 0) || isImporting}
      onRequestClose={onClose}
      onRequestSubmit={() => {
        if (summary) {
          onClose();
        } else {
          handleImport();
        }
      }}
      onSecondarySubmit={summary ? resetFile : onClose}
      size="lg"
    >
      <Stack gap={5}>
        {error && !file && (
          <InlineNotification
            kind="error"
            title="Error"
            subtitle={error}
            lowContrast
            onClose={() => setError(null)}
          />
        )}

        <div className="import-template-section">
          <p className="cds--file--label">ISSUE IMPORT TEMPLATE</p>
          <p className="cds--label-description">
            Download the template to see the expected column format.
          </p>
          <Button
            kind="tertiary"
            size="sm"
            renderIcon={Download}
            onClick={downloadTemplate}
            className="import-template-btn"
          >
            Download Template
          </Button>
        </div>

        <div>
          <p className="cds--file--label">UPLOAD EXCEL FILE</p>
          <p className="cds--label-description">Supported file type: .xlsx, .xls</p>

          {!file ? (
            <FileUploaderDropContainer
              labelText="Drag and drop Excel file here or click to upload"
              accept={[".xlsx", ".xls"]}
              multiple={false}
              onAddFiles={(
                _event: unknown,
                content: { addedFiles?: File[] } | undefined,
              ) => {
                const addedFile = content?.addedFiles?.[0];
                if (addedFile) {
                  const fileList = {
                    0: addedFile,
                    length: 1,
                    item: (idx: number) => (idx === 0 ? addedFile : null),
                  } as unknown as FileList;
                  handleDrop({ target: { files: fileList } });
                } else if (
                  _event &&
                  typeof _event === "object" &&
                  "target" in _event
                ) {
                  handleDrop(_event as { target?: { files?: FileList | null } });
                }
              }}
            />
          ) : (
            <FileUploaderItem name={file.name} status="edit" onDelete={resetFile} />
          )}
        </div>

        {error && file && (
          <InlineNotification
            kind="error"
            title="Parse Error"
            subtitle={error}
            lowContrast
            onClose={() => setError(null)}
          />
        )}

        {summary && (
          <div className="import-summary">
            <InlineNotification
              kind={summary.failed === 0 ? "success" : "warning"}
              title="Import Complete"
              subtitle={`${summary.success} of ${summary.total} issues created successfully.${
                summary.failed > 0 ? ` ${summary.failed} failed.` : ""
              }`}
              lowContrast
            />
            {summary.errors.length > 0 && (
              <div className="import-errors">
                <p className="import-errors-heading">Row Errors:</p>
                <ul>
                  {summary.errors.slice(0, 10).map((e, idx) => (
                    <li key={idx}>
                      Row {e.row}: {e.message}
                    </li>
                  ))}
                  {summary.errors.length > 10 && (
                    <li>...and {summary.errors.length - 10} more errors.</li>
                  )}
                </ul>
              </div>
            )}
          </div>
        )}

        {isImporting && <InlineLoading description="Importing issues..." />}

        {parsedRows.length > 0 && !summary && (
          <div className="import-preview">
            <p className="cds--file--label import-preview-title">
              PREVIEW: {file?.name}
            </p>
            <p className="cds--label-description">
              Showing first {previewRows.length} rows of {parsedRows.length} total rows.
            </p>
            <div className="import-preview-table-wrapper">
              <Table size="sm" aria-label="Import preview table">
                <TableHead>
                  <TableRow>
                    {previewHeaders.map((header) => (
                      <TableHeader key={header}>{header}</TableHeader>
                    ))}
                  </TableRow>
                </TableHead>
                <TableBody>
                  {previewRows.map((row, rowIndex) => (
                    <TableRow key={rowIndex}>
                      {previewHeaders.map((header) => (
                        <TableCell key={`${rowIndex}-${header}`}>
                          {String(row[header] ?? "")}
                        </TableCell>
                      ))}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </div>
        )}
      </Stack>
    </Modal>
  );
};
