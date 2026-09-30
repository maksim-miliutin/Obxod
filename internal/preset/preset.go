package preset

import (
	"strings"

	"obxod/internal/rules"
)

const blocked = "discord.com,discord.gg,discordapp.com,discordapp.net,discordcdn.com,discord.media,youtube.com,googlevideo.com,ytimg.com,x.com,twitter.com,twimg.com,t.co,instagram.com,cdninstagram.com,fbcdn.net,facebook.com,fbsbx.com,telegram.org,web.telegram.org,webk.telegram.org,webz.telegram.org,linkedin.com,licdn.com,soundcloud.com,sndcdn.com,rutracker.org,spotify.com,scdn.co,spotifycdn.com"

type Preset struct {
	Key string
	ru  string
	en  string
	way string
}

func All() []Preset {
	return []Preset{
		{Key: "plain", ru: "Обычный", en: "Plain", way: "hostfake:mail.ru,ts"},
		{Key: "repeats", ru: "С повторами", en: "With repeats", way: "hostfake:mail.ru,ts,repeats:3"},
		{Key: "signature", ru: "Подпись", en: "Signature", way: "hostfake:mail.ru,md5sig"},
		{Key: "cut", ru: "Разрез", en: "Cut", way: "hostfake:mail.ru,cut:name"},
		{Key: "aggressive", ru: "Агрессивный", en: "Aggressive", way: "decoy,cut:name,ttl:4"},
		{Key: "disorder", ru: "Разнобой", en: "Disorder", way: "hostfake:mail.ru,ts,disorder"},
		{Key: "overlap", ru: "Наложение", en: "Overlap", way: "hostfake:mail.ru,ts,overlap:2"},
		{Key: "signed_repeats", ru: "Подпись с повторами", en: "Signature + repeats", way: "hostfake:mail.ru,ts,md5sig,repeats:3"},
		{Key: "decoy_shift", ru: "Обманка со сдвигом", en: "Decoy + shift", way: "decoy,badseq:-10000,ts"},
		{Key: "short_ttl", ru: "Короткий TTL", en: "Short TTL", way: "hostfake:mail.ru,ts,ttl:4"},
	}
}

func (p Preset) Name(code string) string {
	if code == "en" {
		return p.en
	}

	return p.ru
}

func Named(key string) (Preset, bool) {
	for _, p := range All() {
		if p.Key == key {
			return p, true
		}
	}

	return Preset{}, false
}

func ByName(name, code string) (Preset, bool) {
	for _, p := range All() {
		if p.Name(code) == name {
			return p, true
		}
	}

	return Preset{}, false
}

func Names(code string) []string {
	all := All()
	names := make([]string, len(all))

	for i, p := range all {
		names[i] = p.Name(code)
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
