// Package documenttemplates is the Document Templates application module: the
// versioned spreadsheet templates (sheets and their columns) that document
// uploads are validated against and imported with.
//
// It owns its HTTP API, OpenAPI contract, services and persistence. The
// platform host supplies verified access, the selected health context,
// notifications and the databases through the core/modulekit contracts; the
// document_templates, document_template_sheets and document_template_columns
// tables stay in the host's primary-schema migrations, and the has-data check
// reads the warehouse's import.custom_data_files table.
package documenttemplates
