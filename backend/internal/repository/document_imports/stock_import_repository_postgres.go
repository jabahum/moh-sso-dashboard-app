package documentimports

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moh-sso-dashboard/internal/model"
)

type stockImportRepository struct{}

func NewStockImportRepository() StockImportRepository {
	return &stockImportRepository{}
}

func toJSONB(payload map[string]any) ([]byte, error) {
	if payload == nil {
		payload = map[string]any{}
	}

	return json.Marshal(payload)
}

func placeholders(rowIndex, colsPerRow int) string {
	offset := rowIndex * colsPerRow
	parts := make([]string, colsPerRow)

	for i := 0; i < colsPerRow; i++ {
		parts[i] = fmt.Sprintf("$%d", offset+i+1)
	}

	return "(" + strings.Join(parts, ",") + ")"
}

func (r *stockImportRepository) InsertNMSStockIssue(
	ctx context.Context,
	tx *sql.Tx,
	row model.NMSStockIssue,
) error {
	return r.InsertNMSStockIssuesBatch(ctx, tx, []model.NMSStockIssue{row})
}

func (r *stockImportRepository) InsertNMSStockIssuesBatch(
	ctx context.Context,
	tx *sql.Tx,
	rows []model.NMSStockIssue,
) error {
	if len(rows) == 0 {
		return nil
	}

	const colsPerRow = 28

	args := make([]any, 0, len(rows)*colsPerRow)
	valueStrings := make([]string, 0, len(rows))

	for i, row := range rows {
		raw, err := toJSONB(row.RawPayload)
		if err != nil {
			return err
		}

		valueStrings = append(valueStrings, placeholders(i, colsPerRow))

		args = append(args,
			row.DocumentID,
			row.RowNumber,
			row.OrderType,
			row.ShipToFacilityCode,
			row.ShipToFacilityName,
			row.BillToFacilityCode,
			row.BillToFacilityName,
			row.District,
			row.LOC,
			row.FundingSource,
			row.Cycle,
			row.ItemCode,
			row.ItemDescription,
			row.UnitOfMeasure,
			row.OrderNumber,
			row.OrderQuantity,
			row.LineID,
			row.QuantityShipped,
			row.LotNumber,
			row.LotExpirationDate,
			row.ListPrice,
			row.SellingPrice,
			row.CurrencyCode,
			row.AmountUGX,
			row.ShipToLocation,
			row.ShipConfirmDate,
			row.ShipConfirmedBy,
			raw,
		)
	}

	query := fmt.Sprintf(`
		INSERT INTO import.nms_stock_issues (
			document_id,
			row_number,
			order_type,
			ship_to_facility_code,
			ship_to_facility_name,
			bill_to_facility_code,
			bill_to_facility_name,
			district,
			loc,
			funding_source,
			cycle,
			item_code,
			item_description,
			unit_of_measure,
			order_number,
			order_quantity,
			line_id,
			quantity_shipped,
			lot_number,
			lot_expiration_date,
			list_price,
			selling_price,
			currency_code,
			amount_ugx,
			ship_to_location,
			ship_confirm_date,
			ship_confirmed_by,
			raw_payload
		)
		VALUES %s
	`, strings.Join(valueStrings, ","))

	_, err := tx.ExecContext(ctx, query, args...)
	return err
}

func (r *stockImportRepository) InsertNMSStockOnHand(
	ctx context.Context,
	tx *sql.Tx,
	row model.NMSStockOnHand,
) error {
	return r.InsertNMSStockOnHandBatch(ctx, tx, []model.NMSStockOnHand{row})
}

func (r *stockImportRepository) InsertNMSStockOnHandBatch(
	ctx context.Context,
	tx *sql.Tx,
	rows []model.NMSStockOnHand,
) error {
	if len(rows) == 0 {
		return nil
	}

	const colsPerRow = 14

	args := make([]any, 0, len(rows)*colsPerRow)
	valueStrings := make([]string, 0, len(rows))

	for i, row := range rows {
		raw, err := toJSONB(row.RawPayload)
		if err != nil {
			return err
		}

		valueStrings = append(valueStrings, placeholders(i, colsPerRow))

		args = append(args,
			row.DocumentID,
			row.RowNumber,
			row.ItemCode,
			row.ItemDescription,
			row.ItemInventoryStatus,
			row.FundingSource,
			row.SubInventory,
			row.Locator,
			row.LotNumber,
			row.ExpiryDate,
			row.PalletID,
			row.UOM,
			row.OnHandQuantity,
			raw,
		)
	}

	query := fmt.Sprintf(`
		INSERT INTO import.nms_stock_on_hand (
			document_id,
			row_number,
			item_code,
			item_description,
			item_inventory_status,
			funding_source,
			subinventory,
			locator,
			lot_number,
			expiry_date,
			pallet_id,
			uom,
			on_hand_quantity,
			raw_payload
		)
		VALUES %s
	`, strings.Join(valueStrings, ","))

	_, err := tx.ExecContext(ctx, query, args...)
	return err
}

func (r *stockImportRepository) InsertJMSStockIssue(
	ctx context.Context,
	tx *sql.Tx,
	row model.JMSStockIssue,
) error {
	return r.InsertJMSStockIssuesBatch(ctx, tx, []model.JMSStockIssue{row})
}

func (r *stockImportRepository) InsertJMSStockIssuesBatch(
	ctx context.Context,
	tx *sql.Tx,
	rows []model.JMSStockIssue,
) error {
	if len(rows) == 0 {
		return nil
	}

	const colsPerRow = 21

	args := make([]any, 0, len(rows)*colsPerRow)
	valueStrings := make([]string, 0, len(rows))

	for i, row := range rows {
		raw, err := toJSONB(row.RawPayload)
		if err != nil {
			return err
		}

		valueStrings = append(valueStrings, placeholders(i, colsPerRow))

		args = append(args,
			row.DocumentID,
			row.RowNumber,
			row.OrderNo,
			row.SellToCustomerNo,
			row.CustomerName,
			row.District,
			row.Reference,
			row.PartNo,
			row.PartDescription,
			row.OwningCustomerNo,
			row.OwningCustomerName,
			row.UOM,
			row.ItemStatus,
			row.SoldQty,
			row.DesiredQty,
			row.UnitPrice,
			row.DateSold,
			row.AssociationNo,
			row.ExpiryDates,
			row.OFR,
			raw,
		)
	}

	query := fmt.Sprintf(`
		INSERT INTO import.jms_stock_issues (
			document_id,
			row_number,
			order_no,
			sell_to_customer_no,
			customer_name,
			district,
			reference,
			part_no,
			part_description,
			owning_customer_no,
			owning_customer_name,
			uom,
			item_status,
			sold_qty,
			desired_qty,
			unit_price,
			date_sold,
			association_no,
			expiry_dates,
			ofr,
			raw_payload
		)
		VALUES %s
	`, strings.Join(valueStrings, ","))

	_, err := tx.ExecContext(ctx, query, args...)
	return err
}

func (r *stockImportRepository) InsertJMSStockOnHand(
	ctx context.Context,
	tx *sql.Tx,
	row model.JMSStockOnHand,
) error {
	return r.InsertJMSStockOnHandBatch(ctx, tx, []model.JMSStockOnHand{row})
}

func (r *stockImportRepository) InsertJMSStockOnHandBatch(
	ctx context.Context,
	tx *sql.Tx,
	rows []model.JMSStockOnHand,
) error {
	if len(rows) == 0 {
		return nil
	}

	const colsPerRow = 23

	args := make([]any, 0, len(rows)*colsPerRow)
	valueStrings := make([]string, 0, len(rows))

	for i, row := range rows {
		raw, err := toJSONB(row.RawPayload)
		if err != nil {
			return err
		}

		valueStrings = append(valueStrings, placeholders(i, colsPerRow))

		args = append(args,
			row.DocumentID,
			row.RowNumber,
			row.LocationCode,
			row.LocationName,
			row.BinCode,
			row.ReceiptDate,
			row.ReceiptNo,
			row.PurchaseOrderNo,
			row.No,
			row.Description,
			row.LotNo,
			row.ExpirationDate,
			row.AvailableQuantity,
			row.UnitOfMeasureCode,
			row.UnitCost,
			row.Ownership,
			row.TotalCost,
			row.TransferOrderNo,
			row.FromLocation,
			row.ToLocation,
			row.PostingDescription,
			row.YourReference,
			raw,
		)
	}

	query := fmt.Sprintf(`
		INSERT INTO jms_stock_on_hand (
			document_id,
			row_number,
			location_code,
			location_name,
			bin_code,
			receipt_date,
			receipt_no,
			purchase_order_no,
			no,
			description,
			lot_no,
			expiration_date,
			available_quantity,
			unit_of_measure_code,
			unit_cost,
			ownership,
			total_cost,
			transfer_order_no,
			from_location,
			to_location,
			posting_description,
			your_reference,
			raw_payload
		)
		VALUES %s
	`, strings.Join(valueStrings, ","))

	_, err := tx.ExecContext(ctx, query, args...)
	return err
}
