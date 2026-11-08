//go:build !windows

package owners

type Resolver struct{}

func New() *Resolver { return &Resolver{} }

func (r *Resolver) Of(uint16, bool) (string, bool) { return "", false }
