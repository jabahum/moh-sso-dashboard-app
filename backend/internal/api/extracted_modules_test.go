package router

// extractedModules lists the modules extracted from host features. Tests that
// exercise one of the original six packaged modules disable these, because
// their handlers are not part of those fixtures.
const extractedModules = "storage-locations,visualiser,document-templates,data-quality,documents,surveillance"
