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
		{Key: "google_full", ru: "OV1", en: "OV1", way: "multisplit:2+after,seqovl:679", pattern: "google", seqovl: 679,
			descRu: "Режет настоящий хелло на три куска и кладёт google-приветствие на первый. Самый быстрый под нынешний DPI. Рекомендуемый.",
			descEn: "Cuts the real hello into three pieces and lays a google hello over the first. Fastest against the current DPI. Recommended."},
		{Key: "google_overlay", ru: "OV2", en: "OV2", way: "overlap:1", pattern: "google", seqovl: 679,
			descRu: "То же наложение google, но одним разрезом, а не тремя. Легче и порой медленнее. Запасной к OV1.",
			descEn: "The same google overlay, but one cut instead of three. Lighter and sometimes slower. A fallback to OV1."},
		{Key: "google_ghost", ru: "OV3", en: "OV3", way: "fake,ts,ttl:4,overlap:1", pattern: "google", fake: "google", seqovl: 679,
			descRu: "Google-обманка с коротким TTL уходит вперёд и гаснет до сервера, а сверху google ложится на поток. DPI видит google дважды. Пробный.",
			descEn: "A short-TTL google decoy goes ahead and dies before the server, with a google overlay on top. The DPI sees google twice. Experimental."},
		{Key: "ya_decoy", ru: "DC1", en: "DC1", way: "fake,ts,cut:name,disorder,rnd",
			descRu: "Свежая каждый раз ya.ru-обманка уходит вперёд, затем настоящий хелло разрезан и переставлен задом наперёд. Боевой под свежий DPI.",
			descEn: "A fresh ya.ru decoy goes ahead each time, then the real hello cut and reordered back to front. The workhorse for a modern DPI."},
		{Key: "ya_steady", ru: "DC2", en: "DC2", way: "fake,ts,cut:name,rnd",
			descRu: "ya.ru-обманка, половинки по порядку, без перестановки. Мягче к серверу, когда голос рвётся посреди сессии.",
			descEn: "A ya.ru decoy with the halves in order, no reshuffle. Gentler on the server when voice drops mid-session."},
		{Key: "ya_ttl", ru: "DC3", en: "DC3", way: "fake,ts,cut:name,ttl:4,rnd",
			descRu: "ya.ru-обманка, но копия живёт лишь четыре хопа и до сервера не доходит. Когда обычная обманка барахлит.",
			descEn: "A ya.ru decoy whose copy lives only four hops and never reaches the server. For when the plain decoy acts up."},
		{Key: "sferum_decoy", ru: "DC4", en: "DC4", way: "fake,ts,overlap:1,repeats:8", fake: "sferum", pattern: "sferum",
			descRu: "sferum-обманка уходит вперёд и ложится на поток, восемь копий со старением. Запасной на другой обманке.",
			descEn: "A sferum decoy goes ahead and overlays the stream, eight aged copies. A fallback on a different decoy."},
		{Key: "scatter", ru: "DC5", en: "DC5", way: "fake,ts,multisplit:2+after,disorder,rnd",
			descRu: "Свежая ya.ru-обманка вперёд, затем настоящий рвётся на три куска и идёт задом наперёд. Тройная путаница. Пробный.",
			descEn: "A fresh ya.ru decoy ahead, then the real hello torn into three pieces sent back to front. Triple confusion. Experimental."},
		{Key: "badsum_decoy", ru: "DC6", en: "DC6", way: "fake,ts,badsum,cut:name,rnd",
			descRu: "ya.ru-обманка с битой контрольной суммой: сервер молча её отбрасывает, а DPI проглатывает как настоящий хелло. Другое надувательство, когда TTL и перестановка не берут.",
			descEn: "A ya.ru decoy with a broken checksum: the server drops it in silence while the DPI swallows it as the real hello. A different trick when TTL and reordering fall short."},
		{Key: "badack_decoy", ru: "DC7", en: "DC7", way: "fake,ts,badack:-66000,cut:name,rnd",
			descRu: "ya.ru-обманка со сбитым номером подтверждения: сервер её не примет, а DPI учтёт. Ещё один обман в ряд к подписи и битой сумме.",
			descEn: "A ya.ru decoy with a shifted acknowledgement number: the server will not take it, the DPI counts it. Another fooling trick alongside the signature and the bad checksum."},
		{Key: "multi_scatter", ru: "DC8", en: "DC8", way: "fake,ts,multidisorder:3,rnd",
			descRu: "Свежая обманка вперёд, затем настоящий хелло рвётся на три куска и все три идут вразнобой. Путает сильнее, чем перестановка половин. Пробный.",
			descEn: "A fresh decoy ahead, then the real hello torn into three pieces sent in scrambled order. Scrambles harder than reversing two halves. Experimental."},
		{Key: "frag_hello", ru: "FR1", en: "FR1", way: "fake,ts,cut:name,ipfrag:24,rnd",
			descRu: "ya.ru-обманка вперёд, затем настоящий хелло режется у имени, а сам IP-пакет дробится на два фрагмента. Бьёт по другому слою: DPI, что собирает TCP, но не IP, видит обрывок. Пробный.",
			descEn: "A ya.ru decoy ahead, then the real hello cut at the name and the IP packet itself split into two fragments. Hits a different layer: a DPI that reassembles TCP but not IP sees a scrap. Experimental."},
		{Key: "plain", ru: "SW1", en: "SW1", way: "hostfake:mail.ru,ts",
			descRu: "Просто подменяет имя сайта на месте и состаривает копию. Быстрый, простому DPI его хватает.",
			descEn: "Just swaps the site name in place and ages the copy. Fast, and enough for a simple DPI."},
		{Key: "signature", ru: "SW2", en: "SW2", way: "hostfake:mail.ru,ts,md5sig",
			descRu: "Подмена имени плюс копия с TCP-подписью, которую сервер отвергает, а DPI глотает. Другая техника, когда SW1 не берёт.",
			descEn: "Name swap plus a copy with a TCP signature the server rejects and the DPI swallows. A different technique when SW1 falls short."},
		{Key: "shift", ru: "SW3", en: "SW3", way: "hostfake:mail.ru,ts,badseq:-10000",
			descRu: "Подмена имени плюс копия со сбитым номером последовательности. Ещё один обман, когда подпись не берёт.",
			descEn: "Name swap plus a copy with a shifted sequence number. Another fooling trick when the signature falls short."},
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
