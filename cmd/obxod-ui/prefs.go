package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"obxod/internal/preset"
)

type prefs struct {
	Favourites []string       `json:"favourites"`
	Uses       map[string]int `json:"uses"`
}

func prefsFile() string {
	exe, err := os.Executable()
	if err != nil {
		return "obxod-ui.json"
	}

	return filepath.Join(filepath.Dir(exe), "obxod-ui.json")
}

func loadPrefs() prefs {
	p := prefs{Uses: map[string]int{}}

	data, err := os.ReadFile(prefsFile())
	if err != nil {
		return p
	}

	_ = json.Unmarshal(data, &p)
	if p.Uses == nil {
		p.Uses = map[string]int{}
	}

	return p
}

func (p prefs) save() {
	data, err := json.Marshal(p)
	if err != nil {
		return
	}

	_ = os.WriteFile(prefsFile(), data, 0o644)
}

func starred(favourites []string, key string) bool {
	for _, k := range favourites {
		if k == key {
			return true
		}
	}

	return false
}

func toggleFav(favourites []string, key string) []string {
	next := make([]string, 0, len(favourites)+1)
	for _, k := range favourites {
		if k != key {
			next = append(next, k)
		}
	}

	if !starred(favourites, key) {
		next = append(next, key)
	}

	return next
}

// methodOrder lists indices into all: favourites first in their pinned order, then
// the rest by use count high to low, ties keeping the catalogue order.
func methodOrder(all []preset.Preset, favourites []string, uses map[string]int) []int {
	fav := map[string]bool{}
	for _, k := range favourites {
		fav[k] = true
	}

	order := make([]int, 0, len(all))
	for _, k := range favourites {
		for i := range all {
			if all[i].Key == k {
				order = append(order, i)

				break
			}
		}
	}

	rest := make([]int, 0, len(all))
	for i := range all {
		if !fav[all[i].Key] {
			rest = append(rest, i)
		}
	}
	sort.SliceStable(rest, func(a, b int) bool {
		return uses[all[rest[a]].Key] > uses[all[rest[b]].Key]
	})

	return append(order, rest...)
}
