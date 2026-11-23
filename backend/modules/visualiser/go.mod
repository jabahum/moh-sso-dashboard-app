module github.com/mohuganda/moh-sso-dashboard/backend/modules/visualiser

go 1.25.4

require (
	github.com/lib/pq v1.10.9
	github.com/mohuganda/moh-sso-dashboard/backend/core v0.2.0
)

// Built from this repository until backend/core v0.2.0 is tagged.
replace github.com/mohuganda/moh-sso-dashboard/backend/core => ../../core
