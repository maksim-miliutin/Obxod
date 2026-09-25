package preset

import "obxod/internal/rules"

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

func (p Preset) Rules() (rules.Set, error) {
	return rules.Several(blocked + "=" + p.way)
}
