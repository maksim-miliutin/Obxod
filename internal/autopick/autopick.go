package autopick

// Try applies each method in turn and checks whether it works, returning the
// first that does. apply makes a method active; works reports whether a host is
// reachable under whatever is now active; stop, checked before each method, aborts
// the search when it reports true. A method that will not apply is skipped rather
// than fatal, since the next one may still work.
func Try(methods []string, apply func(name string) error, works func() bool, stop func() bool) (string, bool) {
	for _, name := range methods {
		if stop() {
			return "", false
		}

		if apply(name) != nil {
			continue
		}

		if works() {
			return name, true
		}
	}

	return "", false
}
