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

// Best applies each method, scores how it does once active, and returns the one
// that scored highest. A method that will not apply is skipped; stop, checked
// before each, ends the search and keeps the best found so far. score is read
// after the method is active, so the caller settles and reads the live score.
func Best(methods []string, apply func(name string) error, score func() int, stop func() bool) (string, bool) {
	best := ""
	top := 0
	found := false

	for _, name := range methods {
		if stop() {
			break
		}

		if apply(name) != nil {
			continue
		}

		if s := score(); !found || s > top {
			best, top, found = name, s, true
		}
	}

	return best, found
}
