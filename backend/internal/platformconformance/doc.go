// Package platformconformance runs the platform API conformance suite
// (backend/core/platform/conformance) against the CI stack in
// infra/compose/platform-conformance. The test is skipped unless
// PLATFORM_CONFORMANCE_PORTAL is set; infra/scripts/test-platform-conformance.sh
// starts the stack, sets the environment and runs it.
package platformconformance
