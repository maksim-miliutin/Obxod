package preset

import (
	"strings"

	"obxod/internal/rules"
)

const blocked = "discord.com,discord.gg,discordapp.com,discordapp.net,discordcdn.com,discord.media,hcaptcha.com,challenges.cloudflare.com,youtube.com,googlevideo.com,ytimg.com,x.com,twitter.com,twimg.com,t.co,instagram.com,cdninstagram.com,fbcdn.net,facebook.com,fbsbx.com,telegram.org,web.telegram.org,webk.telegram.org,webz.telegram.org,linkedin.com,licdn.com,soundcloud.com,sndcdn.com,rutracker.org,spotify.com,scdn.co,spotifycdn.com"

type Preset struct {
	Key    string
	ru     string
	en     string
	way    string
	descRu string
	descEn string
}

func All() []Preset {
	return []Preset{
		{Key: "plain", ru: "Обычный", en: "Plain", way: "hostfake:mail.ru,ts",
			descRu: "Подменяет имя сайта и состаривает копию. Базовый — начните с него.",
			descEn: "Swaps the site name and ages the copy. The baseline — start here."},
		{Key: "hostfake_ya", ru: "Обманка ya.ru", en: "ya.ru decoy", way: "fake,ts,cut:name,disorder",
			descRu: "Шлёт готовый ya.ru-хелло обманкой, затем режет настоящий по имени и половинками задом наперёд. Под свежий DPI, как боевой Zapret ALT3.",
			descEn: "Sends a ready ya.ru hello as a decoy, then cuts the real one through its name and sends the halves back to front. For the current DPI, like Zapret's ALT3."},
		{Key: "repeats", ru: "С повторами", en: "With repeats", way: "hostfake:mail.ru,ts,repeats:3",
			descRu: "То же, но подмена шлётся трижды — на случай потери пакетов.",
			descEn: "The same, but the swap is sent three times, for lossy links."},
		{Key: "signature", ru: "Подпись", en: "Signature", way: "hostfake:mail.ru,md5sig",
			descRu: "Подмена имени плюс TCP-подпись, которую сервер отвергнет.",
			descEn: "Name swap plus a TCP signature the server rejects."},
		{Key: "cut", ru: "Разрез", en: "Cut", way: "hostfake:mail.ru,cut:name",
			descRu: "Подмена имени и разрез посреди имени на два пакета.",
			descEn: "Name swap, then a cut through the middle of the name."},
		{Key: "aggressive", ru: "Агрессивный", en: "Aggressive", way: "decoy,cut:name,ttl:4",
			descRu: "Обманный хелло с коротким TTL и разрез. Пожёстче.",
			descEn: "A decoy hello with a short TTL, plus a cut. Harder."},
		{Key: "aggressive_ttl6", ru: "Агрессивный, TTL 6", en: "Aggressive, TTL 6", way: "decoy,cut:name,ttl:6",
			descRu: "Та же обманка, копия живёт 6 хопов. Если «Агрессивный» берёт через раз.",
			descEn: "The same decoy, living 6 hops. If Aggressive works only sometimes."},
		{Key: "aggressive_ttl8", ru: "Агрессивный, TTL 8", en: "Aggressive, TTL 8", way: "decoy,cut:name,ttl:8",
			descRu: "Та же обманка, копия живёт 8 хопов. Если DPI дальше по маршруту.",
			descEn: "The same decoy, living 8 hops. If the DPI sits further along."},
		{Key: "overlay", ru: "Реальный хелло внахлёст", en: "Real hello overlay", way: "overlap:1",
			descRu: "Кладёт настоящий google-хелло внахлёст. Приём свежего Zapret, против злых DPI.",
			descEn: "Lays a real google hello over the start. The current Zapret trick, for tough DPI."},
		{Key: "fakehello", ru: "Реальный хелло, обманка", en: "Real hello fake", way: "fake,ts,repeats:11",
			descRu: "Шлёт настоящий google-хелло обманкой 11 раз со старой меткой. Второй приём Zapret.",
			descEn: "Sends a real google hello as a decoy 11 times with a stale timestamp. Zapret's other trick."},
		{Key: "overlay_fake", ru: "Обманка плюс внахлёст", en: "Fake plus overlay", way: "fake,ts,overlap:1,repeats:8",
			descRu: "Оба приёма разом: обманка настоящим хелло и он же внахлёст. Самый сильный, как боевой Zapret.",
			descEn: "Both tricks at once: a real hello as a decoy and the same laid over. The strongest, like Zapret's ALT."},
		{Key: "disorder", ru: "Разнобой", en: "Disorder", way: "hostfake:mail.ru,ts,disorder",
			descRu: "Подмена имени, половинки уходят задом наперёд.",
			descEn: "Name swap, the halves go out back to front."},
		{Key: "overlap", ru: "Наложение", en: "Overlap", way: "hostfake:mail.ru,ts,overlap:2",
			descRu: "Поверх потока кладётся кусок, который сервер отбросит.",
			descEn: "A chunk the server discards is laid over the stream."},
		{Key: "signed_repeats", ru: "Подпись с повторами", en: "Signature + repeats", way: "hostfake:mail.ru,ts,md5sig,repeats:3",
			descRu: "Подпись плюс три копии.",
			descEn: "Signature, plus three copies."},
		{Key: "decoy_shift", ru: "Обманка со сдвигом", en: "Decoy + shift", way: "decoy,badseq:-10000,ts",
			descRu: "Обманка со сдвинутым номером и старой меткой времени.",
			descEn: "A decoy with a shifted number and a stale timestamp."},
		{Key: "short_ttl", ru: "Короткий TTL", en: "Short TTL", way: "hostfake:mail.ru,ts,ttl:4",
			descRu: "Подмена имени, копия живёт лишь несколько хопов.",
			descEn: "Name swap, the copy lives only a few hops."},
		{Key: "badsum", ru: "Битая сумма", en: "Bad checksum", way: "hostfake:mail.ru,ts,badsum",
			descRu: "Подмена имени, у копии битая сумма. Для стримов.",
			descEn: "Name swap, the copy has a bad checksum. For streams."},
		{Key: "cut_start", ru: "Разрез в начале", en: "Cut at start", way: "hostfake:mail.ru,ts,cut:start",
			descRu: "Подмена имени и разрез у начала записи. Быстрый.",
			descEn: "Name swap, a cut near the record start. Quick."},
		{Key: "signed_shift", ru: "Подпись со сдвигом", en: "Signature + shift", way: "hostfake:mail.ru,ts,md5sig,badseq:-10000",
			descRu: "Подпись со сдвинутым номером последовательности.",
			descEn: "Signature with a shifted sequence number."},
		{Key: "overlap_disorder", ru: "Наложение с разнобоем", en: "Overlap + disorder", way: "hostfake:mail.ru,ts,overlap:3,disorder",
			descRu: "Наложение плюс половинки задом наперёд.",
			descEn: "Overlap, plus the halves back to front."},
		{Key: "scatter", ru: "Вразнобой кусками", en: "Scatter", way: "multidisorder:4",
			descRu: "Режет хелло на 4 куска вразнобой. Без подмены имени.",
			descEn: "Cuts the hello into four out-of-order pieces. No name swap."},
		{Key: "multisplit", ru: "Разрез в точках", en: "Multi-split", way: "multisplit:1+name",
			descRu: "Режет хелло в начале и посреди имени. Без подмены имени.",
			descEn: "Cuts the hello at the start and mid-name. No name swap."},
		{Key: "ipfrag", ru: "IP-фрагменты", en: "IP fragment", way: "ipfrag:24",
			descRu: "Рвёт пакет на IP-фрагменты. Без подмены имени; не на всех DPI.",
			descEn: "Splits the packet into IP fragments. No name swap; not on every DPI."},
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
