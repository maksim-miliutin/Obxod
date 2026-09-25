package preset

import (
	"strings"

	"obxod/internal/rules"
)

const blocked = "discord.com,discord.gg,discordapp.com,discordapp.net,discordcdn.com,discord.media,youtube.com,googlevideo.com,x.com"

type Preset struct {
	Name string
	way  string
}

func All() []Preset {
	return []Preset{
		{Name: "Обычный", way: "hostfake:mail.ru,ts"},
		{Name: "С повторами", way: "hostfake:mail.ru,ts,repeats:3"},
		{Name: "Подпись", way: "hostfake:mail.ru,md5sig"},
		{Name: "Разрез", way: "hostfake:mail.ru,cut:name"},
		{Name: "Агрессивный", way: "decoy,cut:name,ttl:4"},
	}
}

func Named(name string) (Preset, bool) {
	for _, p := range All() {
		if p.Name == name {
			return p, true
		}
	}

	return Preset{}, false
}

func Names() []string {
	all := All()
	names := make([]string, len(all))

	for i, p := range all {
		names[i] = p.Name
	}

	return names
}

func Hosts() []string {
	return strings.Split(blocked, ",")
}

func (p Preset) Rules() (rules.Set, error) {
	return rules.Several(blocked + "=" + p.way)
}
