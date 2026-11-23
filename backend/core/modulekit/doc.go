// Package modulekit is the shared HTTP contract between the trusted platform
// host and compiled application modules.
//
// The host authenticates the caller, resolves access and selects the health
// context, then attaches the result to each request with WithRequest. A module
// declares its operations as Endpoints, and its Router enforces the declared
// permissions against that verified request state, so modules never read
// credentials, headers or host types. Responses use the platform's standard
// envelope ({success, data|error, meta}).
//
// The package uses only the standard library, so a module that depends on it
// gains no framework or host dependency.
package modulekit
