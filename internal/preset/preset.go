package preset

import (
	"strings"

	"obxod/internal/rules"
)

const blocked = "discord.com,discord.gg,discordapp.com,discordapp.net,discordcdn.com,discord.media,hcaptcha.com,challenges.cloudflare.com,youtube.com,googlevideo.com,ytimg.com,x.com,twitter.com,twimg.com,t.co,instagram.com,cdninstagram.com,fbcdn.net,facebook.com,fbsbx.com,telegram.org,web.telegram.org,webk.telegram.org,webz.telegram.org,linkedin.com,licdn.com,soundcloud.com,sndcdn.com,rutracker.org,spotify.com,scdn.co,spotifycdn.com"

type Preset struct {
	Key     string
	ru      string
	en      string
	way     string
	descRu  string
	descEn  string
	pattern string // overlay hello key; empty lays the default decoy over the stream
	fake    string // decoy hello key; empty sends the default decoy
	seqovl  int    // bytes the overlay is laid over; zero lays over the whole hello
}

func All() []Preset {
	return []Preset{
		{Key: "plain", ru: "Лёгкий", en: "Light", way: "hostfake:mail.ru,ts",
			descRu: "Просто подменяет имя сайта на месте и состаривает копию. Быстрый, и несложному DPI его хватает.",
			descEn: "Just swaps the site name in place and ages the copy. Fast, and enough when the DPI is simple."},
		{Key: "google_overlay", ru: "Наложение Google", en: "Google overlay", way: "overlap:1", pattern: "google", seqovl: 679,
			descRu: "Накладывает google-приветствие на поток перед настоящим: DPI читает google, сервер берёт твоё. Это Zapret ALT7, на котором ты сейчас. Рекомендуемый.",
			descEn: "Lays a google hello over the stream ahead of the real one: the DPI reads google, the server takes yours. This is Zapret's ALT7, your current method. Recommended."},
		{Key: "ya_decoy", ru: "Обманка ya.ru", en: "ya.ru decoy", way: "fake,ts,cut:name,disorder,rnd",
			descRu: "Свежая каждый раз ya.ru-обманка вперёд, затем разрезанный настоящий. Боевой, под свежий DPI, как Zapret ALT3.",
			descEn: "A fresh ya.ru hello as a decoy each time, then the real one cut. The workhorse for a modern DPI, like Zapret's ALT3."},
		{Key: "sferum_decoy", ru: "Обманка Sferum", en: "Sferum decoy", way: "fake,ts,overlap:1,repeats:8", fake: "sferum", pattern: "sferum",
			descRu: "Вперёд летит sferum-обманка и накладывается на поток, восемь копий со старением. Запасной в духе Zapret ALT11/12.",
			descEn: "A sferum decoy goes ahead and overlays the stream, eight aged copies. A fallback in the spirit of Zapret's ALT11/12."},
		{Key: "ya_steady", ru: "Обманка ровная", en: "Steady decoy", way: "fake,ts,cut:name,rnd",
			descRu: "Как «Обманка ya.ru», но половинки идут по порядку, не задом наперёд. Мягче к серверу, если голос рвётся в сессии.",
			descEn: "Like the ya.ru decoy, but the halves go in order, not reversed. Gentler on the server, for when voice drops mid-session."},
		{Key: "ya_ttl", ru: "Короткий TTL", en: "Short TTL", way: "fake,ts,cut:name,ttl:4,rnd",
			descRu: "Та же обманка ya.ru, но копия живёт лишь четыре хопа: доходит до DPI, но не до сервера. Если «Обманка» барахлит.",
			descEn: "The same ya.ru decoy, but the copy lives only four hops: it reaches the DPI, not the server. If the plain decoy acts up."},
		{Key: "signature", ru: "Подпись", en: "Signature", way: "hostfake:mail.ru,ts,md5sig",
			descRu: "Подмена имени и копия с TCP-подписью, которую сервер отвергает, а DPI глотает. Другая техника.",
			descEn: "Name swap plus a copy carrying a TCP signature the server rejects and the DPI swallows. A different technique."},
		{Key: "shift", ru: "Сдвиг", en: "Shift", way: "hostfake:mail.ru,ts,badseq:-10000",
			descRu: "Подмена имени и копия со сбитым номером последовательности. Ещё один обман, если подпись не берёт.",
			descEn: "Name swap plus a copy with a shifted sequence number. Another fooling trick, if the signature does not take."},
	}
}

func (p Preset) Name(code string) string {
	if code == "en" {
		return p.en
	}

	return p.ru
}

func (p Preset) Desc(code string) string {
	if code == "en" {
		return p.descEn
	}

	return p.descRu
}

func (p Preset) Pattern() string {
	return p.pattern
}

func (p Preset) Fake() string {
	return p.fake
}

func (p Preset) Seqovl() int {
	return p.seqovl
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

const favouriteMark = "* "

// Label is how a method shows in a favourite-sorted list: its name, with a leading
// mark when the method is among the favourites.
func Label(p Preset, code string, favourite []string) string {
	for _, key := range favourite {
		if key == p.Key {
			return favouriteMark + p.Name(code)
		}
	}

	return p.Name(code)
}

// NamesByFavourite lists method labels with the favourites first and marked, each
// group kept in All's order. favourite holds preset keys; a key that names nothing
// is ignored.
func NamesByFavourite(code string, favourite []string) []string {
	starred := map[string]bool{}
	for _, key := range favourite {
		starred[key] = true
	}

	top := make([]string, 0, len(All()))
	rest := make([]string, 0, len(All()))

	for _, p := range All() {
		if starred[p.Key] {
			top = append(top, Label(p, code, favourite))

			continue
		}

		rest = append(rest, Label(p, code, favourite))
	}

	return append(top, rest...)
}

// ByLabel finds the preset for a label from a favourite-sorted list, removing the
// leading mark a favourite carries before matching the plain name.
func ByLabel(label, code string) (Preset, bool) {
	return ByName(strings.TrimPrefix(label, favouriteMark), code)
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
