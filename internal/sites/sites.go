package sites

import (
	"bufio"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"
)

type Set struct {
	names map[string]bool
}

func New() *Set {
	return &Set{names: map[string]bool{}}
}

func (s *Set) Add(name string) {
	name = domain(name)
	if name == "" {
		return
	}

	s.names[name] = true
}

func (s *Set) Remove(name string) {
	delete(s.names, domain(name))
}

func domain(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return ""
	}

	if !strings.Contains(input, "://") {
		input = "//" + input
	}

	u, err := url.Parse(input)
	if err != nil {
		return ""
	}

	host := strings.TrimPrefix(u.Hostname(), "www.")

	// A real domain has a dot; a bare word is a typo, not a host to bypass.
	if !strings.Contains(host, ".") {
		return ""
	}

	return host
}

func (s *Set) List() []string {
	out := make([]string, 0, len(s.names))

	for name := range s.names {
		out = append(out, name)
	}

	sort.Strings(out)

	return out
}

func Read(r io.Reader) (*Set, error) {
	s := New()
	scan := bufio.NewScanner(r)

	for scan.Scan() {
		s.Add(scan.Text())
	}

	return s, scan.Err()
}

func (s *Set) Write(w io.Writer) error {
	for _, name := range s.List() {
		if _, err := io.WriteString(w, name+"\n"); err != nil {
			return err
		}
	}

	return nil
}

func Load(path string) (*Set, error) {
	f, err := os.Open(path)
	if err != nil {
		// No file yet means no sites added, not a failure.
		if os.IsNotExist(err) {
			return New(), nil
		}

		return nil, err
	}
	defer f.Close()

	return Read(f)
}

func (s *Set) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return s.Write(f)
}
