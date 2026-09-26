package preset

import (
	"strings"

	"obxod/internal/rules"
)

const blocked = "discord.com,discord.gg,discordapp.com,discordapp.net,discordcdn.com,discord.media,youtube.com,googlevideo.com,ytimg.com,x.com,twitter.com,twimg.com,t.co,instagram.com,cdninstagram.com,fbcdn.net,facebook.com,fbsbx.com,telegram.org,web.telegram.org,webk.telegram.org,webz.telegram.org,linkedin.com,licdn.com,soundcloud.com,sndcdn.com,rutracker.org"

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

func HostsWith(extra []string) []string {
	return append(Hosts(), extra...)
}

func (p Preset) Rules() (rules.Set, error) {
	return p.RulesWith(nil)
}

func (p Preset) RulesWith(extra []string) (rules.Set, error) {
	hosts := blocked
	if len(extra) > 0 {
		hosts += "," + strings.Join(extra, ",")
	}

	return rules.Several(hosts + "=" + p.way)
}

func (p Preset) RulesVoice(extra []string) (rules.Set, error) {
	hosts := blocked
	if len(extra) > 0 {
		hosts += "," + strings.Join(extra, ",")
	}

	return rules.Several(hosts + "=" + p.way + ",fakeudp:5")
}
