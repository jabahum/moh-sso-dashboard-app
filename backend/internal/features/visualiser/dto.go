package visualiser

type DataElementsRequest struct {
	DataSetID *string `json:"data_set_id" form:"data_set_id"`
}

type DataValuesRequest struct {
	// AggregationLevel overrides output grouping; currently only facility level is supported.
	AggregationLevel string   `json:"aggregationLevel" binding:"omitempty,oneof=6"`
	OU               []string `json:"ou"`
	PE               []string `json:"pe"`
	DX               []string `json:"dx"`
	LevelOfCare      []string `json:"levelOfCare"`
	Ownership        []string `json:"ownership"`
	StartDate        string   `json:"startDate"`
	EndDate          string   `json:"endDate"`
	OrgunitLevel     any      `json:"orgunitLevel"`
}

type DataElementsByThemeRequest struct {
	ThemeID string `json:"theme_id" binding:"required"`
}

type DatasetResponse struct {
	DatasetKey  int64  `json:"dataset_key"`
	DatasetID   string `json:"dataset_id"`
	DisplayName string `json:"display_name"`
	IsCurrent   bool   `json:"is_current"`
	CreateDate  string `json:"create_date"`
}

type DataElementResponse struct {
	DimDataElementMapKey int64  `json:"dim_data_element_map_key"`
	DataElementID        string `json:"data_element_id"`
	DataSetID            string `json:"data_set_id"`
	DataElementShortName string `json:"data_element_short_name"`
	DataElementLongName  string `json:"data_element_long_name"`
	RowVersion           int64  `json:"row_version"`
	IsCurrent            bool   `json:"is_current"`
}

type DataValueRowResponse struct {
	OrgUnitID     string `json:"org_unit_id"`
	OrgUnitName   string `json:"org_unit_name"`
	DataElementID string `json:"data_element_id"`
	Period        string `json:"period"`
	Value         int64  `json:"value"`
	Level         string `json:"level"`
	LevelOfCare   string `json:"level_of_care"`
	Ownership     string `json:"ownership"`
	Dataelement   string `json:"dataelement"`
}

type DataValuesResponse struct {
	Rows []DataValueRowResponse `json:"rows"`
}
