package usecase

import "fmt"

// ServiceError is what an Invoker implementation carries when a service
// answered a call with a non-2xx status, so Orchestrator can tell "the
// service answered no" (a 4xx: the id does not exist, the value is
// invalid) apart from a platform or service fault (a 5xx, a timeout, an
// unreachable service) without parsing the invoker's error string.
// usecase stays stdlib-only (AGENTS.md rule 4's contract-first discipline
// extends to this: no net/http, no encoding/json here) - it is the
// adapter under internal/adapter/invoker that knows a service speaks
// HTTP and JSON, and it is the adapter that decodes a status and an
// optional {"message": "..."} body into this type. Orchestrator inspects
// it with errors.As, since the invoker's own error wraps it alongside
// its own request-line detail.
type ServiceError struct {
	// Status is the HTTP status code the service responded with.
	Status int
	// Message is the service's own message, read from a JSON error body
	// shaped like {"message": "..."} when it gave one, and "" when the
	// body was empty, not JSON, or had no such field.
	Message string
}

// Error satisfies the error interface, so a ServiceError is usable
// wherever an error is, in addition to being findable with errors.As.
func (e ServiceError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("service responded %d: %s", e.Status, e.Message)
	}

	return fmt.Sprintf("service responded %d", e.Status)
}
