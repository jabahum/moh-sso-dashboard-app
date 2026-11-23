package documents

import "testing"

func TestShouldQueueProcessing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		contentType  string
		filename     string
		isTemplate   bool
		templateCode string
		want         bool
	}{
		{
			name:        "spreadsheet without template is stored only",
			contentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			filename:    "report.xlsx",
			want:        false,
		},
		{
			name:         "spreadsheet with template is processed",
			contentType:  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			filename:     "report.xlsx",
			templateCode: "monthly-report",
			want:         true,
		},
		{
			name:         "template upload is not processed",
			contentType:  "text/csv",
			filename:     "template.csv",
			isTemplate:   true,
			templateCode: "monthly-report",
			want:         false,
		},
		{
			name:         "pdf is never processed",
			contentType:  "application/pdf",
			filename:     "guide.pdf",
			templateCode: "monthly-report",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := shouldQueueProcessing(
				tt.contentType,
				tt.filename,
				tt.isTemplate,
				tt.templateCode,
			)
			if got != tt.want {
				t.Fatalf("shouldQueueProcessing() = %v, want %v", got, tt.want)
			}
		})
	}
}
