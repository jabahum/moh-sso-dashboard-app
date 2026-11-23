module github.com/mohuganda/moh-sso-dashboard/backend/modules/documents

go 1.25.4

require (
	github.com/google/uuid v1.6.0
	github.com/lib/pq v1.10.9
	github.com/mohuganda/moh-sso-dashboard/backend/core v0.2.0
	github.com/xuri/excelize/v2 v2.10.1
)

require (
	github.com/richardlehane/mscfb v1.0.6 // indirect
	github.com/richardlehane/msoleps v1.0.6 // indirect
	github.com/tiendc/go-deepcopy v1.7.2 // indirect
	github.com/xuri/efp v0.0.1 // indirect
	github.com/xuri/nfp v0.0.2-0.20250530014748-2ddeb826f9a9 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/text v0.34.0 // indirect
)

// Built from this repository until backend/core v0.2.0 is tagged.
replace github.com/mohuganda/moh-sso-dashboard/backend/core => ../../core
