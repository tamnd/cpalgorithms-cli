package cli

func isNotFound(_ error) bool {
	// cp-algorithms.com is a static site; 404s surface as http errors, not
	// typed sentinel values. Reserve this hook for future typed errors.
	return false
}
