package documenttemplates

func toTemplateResponse(template *DocumentTemplate) *TemplateResponse {
	if template == nil {
		return nil
	}
	return &TemplateResponse{
		ID:            template.ID,
		DocumentID:    template.DocumentID,
		Code:          template.Code,
		Name:          template.Name,
		Description:   template.Description,
		FileType:      template.FileType,
		Version:       template.Version,
		IsActive:      template.IsActive,
		Configuration: template.Configuration,
		CreatedBy:     template.CreatedBy,
		CreatedAt:     template.CreatedAt,
		UpdatedAt:     template.UpdatedAt,
		ArchivedAt:    template.ArchivedAt,
	}
}

func toTemplateResponses(templates []DocumentTemplate) []TemplateResponse {
	out := make([]TemplateResponse, 0, len(templates))
	for _, template := range templates {
		item := toTemplateResponse(&template)
		if item != nil {
			out = append(out, *item)
		}
	}
	return out
}

func toSheetResponse(sheet *DocumentTemplateSheet) *SheetResponse {
	if sheet == nil {
		return nil
	}
	return &SheetResponse{
		ID:                    sheet.ID,
		TemplateID:            sheet.TemplateID,
		Code:                  sheet.Code,
		Name:                  sheet.Name,
		DisplayName:           sheet.DisplayName,
		Required:              sheet.Required,
		SheetOrder:            sheet.SheetOrder,
		HeaderRow:             sheet.HeaderRow,
		StartRow:              sheet.StartRow,
		AllowExtraColumns:     sheet.AllowExtraColumns,
		AllowDuplicateHeaders: sheet.AllowDuplicateHeaders,
		Configuration:         sheet.Configuration,
		CreatedAt:             sheet.CreatedAt,
		UpdatedAt:             sheet.UpdatedAt,
		ArchivedAt:            sheet.ArchivedAt,
	}
}

func toSheetResponses(sheets []DocumentTemplateSheet) []SheetResponse {
	out := make([]SheetResponse, 0, len(sheets))
	for _, sheet := range sheets {
		item := toSheetResponse(&sheet)
		if item != nil {
			out = append(out, *item)
		}
	}
	return out
}

func toColumnResponse(column *DocumentTemplateColumn) *ColumnResponse {
	if column == nil {
		return nil
	}
	return &ColumnResponse{
		ID:            column.ID,
		SheetID:       column.SheetID,
		ColumnKey:     column.ColumnKey,
		ColumnName:    column.ColumnName,
		DisplayName:   column.DisplayName,
		DataType:      column.DataType,
		Required:      column.Required,
		IsUnique:      column.IsUnique,
		ColumnOrder:   column.ColumnOrder,
		DefaultValue:  column.DefaultValue,
		AllowedValues: column.AllowedValues,
		Aliases:       column.Aliases,
		Configuration: column.Configuration,
		CreatedAt:     column.CreatedAt,
		UpdatedAt:     column.UpdatedAt,
		ArchivedAt:    column.ArchivedAt,
	}
}

func toColumnResponses(columns []DocumentTemplateColumn) []ColumnResponse {
	out := make([]ColumnResponse, 0, len(columns))
	for _, column := range columns {
		item := toColumnResponse(&column)
		if item != nil {
			out = append(out, *item)
		}
	}
	return out
}

func toTemplateRuntimeResponse(runtime *TemplateRuntime) *TemplateRuntimeResponse {
	if runtime == nil {
		return nil
	}
	out := &TemplateRuntimeResponse{
		ID:       runtime.ID,
		Code:     runtime.Code,
		Version:  runtime.Version,
		IsActive: runtime.IsActive,
		Sheets:   make(map[string]SheetRuntimeResponse, len(runtime.Sheets)),
	}
	for key, sheet := range runtime.Sheets {
		item := toSheetRuntimeResponse(sheet)
		if item != nil {
			out.Sheets[key] = *item
		}
	}
	return out
}

func toSheetRuntimeResponse(sheet *TemplateSheetRuntime) *SheetRuntimeResponse {
	if sheet == nil {
		return nil
	}
	out := &SheetRuntimeResponse{
		ID:                    sheet.ID,
		Code:                  sheet.Code,
		Name:                  sheet.Name,
		DisplayName:           sheet.DisplayName,
		Required:              sheet.Required,
		SheetOrder:            sheet.SheetOrder,
		HeaderRow:             sheet.HeaderRow,
		StartRow:              sheet.StartRow,
		AllowExtraColumns:     sheet.AllowExtraColumns,
		AllowDuplicateHeaders: sheet.AllowDuplicateHeaders,
		Configuration:         sheet.Configuration,
		Columns:               make(map[string]ColumnRuntimeResponse, len(sheet.Columns)),
	}
	for key, column := range sheet.Columns {
		item := toColumnRuntimeResponse(column)
		if item != nil {
			out.Columns[key] = *item
		}
	}
	return out
}

func toColumnRuntimeResponse(column *TemplateColumnRuntime) *ColumnRuntimeResponse {
	if column == nil {
		return nil
	}
	return &ColumnRuntimeResponse{
		ID:            column.ID,
		ColumnKey:     column.ColumnKey,
		ColumnName:    column.ColumnName,
		DisplayName:   column.DisplayName,
		DataType:      column.DataType,
		Required:      column.Required,
		IsUnique:      column.IsUnique,
		ColumnOrder:   column.ColumnOrder,
		DefaultValue:  column.DefaultValue,
		AllowedValues: column.AllowedValues,
		Aliases:       column.Aliases,
		Configuration: column.Configuration,
	}
}

func toTemplateStructureResponse(structure *TemplateStructure) *TemplateStructureResponse {
	if structure == nil {
		return nil
	}
	template := toTemplateResponse(&structure.Template)
	out := &TemplateStructureResponse{
		Sheets: make([]SheetStructureResponse, 0, len(structure.Sheets)),
	}
	if template != nil {
		out.Template = *template
	}
	for _, sheet := range structure.Sheets {
		out.Sheets = append(out.Sheets, toSheetStructureResponse(sheet))
	}
	return out
}

func toSheetStructureResponse(sheet TemplateSheetStructure) SheetStructureResponse {
	out := SheetStructureResponse{
		ID:                    sheet.ID,
		TemplateID:            sheet.TemplateID,
		Code:                  sheet.Code,
		Name:                  sheet.Name,
		DisplayName:           sheet.DisplayName,
		Required:              sheet.Required,
		SheetOrder:            sheet.SheetOrder,
		HeaderRow:             sheet.HeaderRow,
		StartRow:              sheet.StartRow,
		AllowExtraColumns:     sheet.AllowExtraColumns,
		AllowDuplicateHeaders: sheet.AllowDuplicateHeaders,
		Configuration:         sheet.Configuration,
		Columns:               make([]ColumnStructureResponse, 0, len(sheet.Columns)),
	}
	for _, column := range sheet.Columns {
		out.Columns = append(out.Columns, toColumnStructureResponse(column))
	}
	return out
}

func toColumnStructureResponse(column TemplateColumnStructure) ColumnStructureResponse {
	return ColumnStructureResponse{
		ID:            column.ID,
		SheetID:       column.SheetID,
		ColumnKey:     column.ColumnKey,
		ColumnName:    column.ColumnName,
		DisplayName:   column.DisplayName,
		DataType:      column.DataType,
		Required:      column.Required,
		IsUnique:      column.IsUnique,
		ColumnOrder:   column.ColumnOrder,
		DefaultValue:  column.DefaultValue,
		AllowedValues: column.AllowedValues,
		Aliases:       column.Aliases,
		Configuration: column.Configuration,
	}
}
