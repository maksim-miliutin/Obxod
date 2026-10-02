package lang

type Text int

const (
	StatusOff Text = iota
	StatusOn
	TurnOn
	TurnOff
	MethodLabel
	Caveat
	Autostart
	SpeedFmt
	CountFmt
	AddSite
	YourSites
	AlwaysSites
	Copy
	Clear
	TabBypass
	TabSites
	TabLogs
	TabAbout
	TrayShow
	TrayQuit
	AboutWhat
	AboutOpens
	AboutUnsigned
	AboutMethods
	LanguageLabel
	RestartNote
	AutoPick
	Stop
	Now
	AddFav
	DropFav
	Searching
	AutoPickNote
	NoneWorked
	last
)

type Lang [last]string

var ru = Lang{
	StatusOff:     "Выключено",
	StatusOn:      "Работает",
	TurnOn:        "Включить",
	TurnOff:       "Выключить",
	MethodLabel:   "Способ:",
	Caveat:        "Не подошёл — попробуйте другой.\nУ разных провайдеров работают разные.",
	Autostart:     "Запускать при старте Windows",
	SpeedFmt:      "Скорость: %s",
	CountFmt:      "Обходится сайтов: %d",
	AddSite:       "Добавить сайт",
	YourSites:     "Ваши сайты:",
	AlwaysSites:   "Всегда обходятся:",
	Copy:          "Скопировать",
	Clear:         "Очистить",
	TabBypass:     "Обход",
	TabSites:      "Сайты",
	TabLogs:       "Логи",
	TabAbout:      "О программе",
	TrayShow:      "Показать",
	TrayQuit:      "Выход",
	AboutWhat:     "Obxod — обход DPI-блокировок.",
	AboutOpens:    "Открывает то, что режут по имени хоста:\nDiscord, YouTube, X и добавленные вами.",
	AboutUnsigned: "При запуске Windows может сказать\n«неизвестный издатель» — это нормально,\nподписи пока нет: Подробнее, затем Всё равно запустить.",
	AboutMethods:  "Способы перебирайте, если сайт не открылся.\nСтримы и демонстрация экрана — «Битая сумма»\n(включается не сразу). Быстрее всего — «Разрез в начале».\nНачните с «Обычный» или «Подпись».",
	LanguageLabel: "Язык:",
	RestartNote:   "Смена языка вступит в силу после перезапуска.",
	AutoPick:      "Подобрать",
	Stop:          "Стоп",
	Now:           "Сейчас: %s",
	AddFav:        "В избранное",
	DropFav:       "Убрать",
	Searching:     "Пробую: %s",
	AutoPickNote:  "Проверяет только, открывается ли сайт.\nГолос и стримы проверьте сами.",
	NoneWorked:    "Ни один способ не подошёл.",
}

var en = Lang{
	StatusOff:     "Off",
	StatusOn:      "Running",
	TurnOn:        "Turn on",
	TurnOff:       "Turn off",
	MethodLabel:   "Method:",
	Caveat:        "Didn't work — try another.\nDifferent providers need different ones.",
	Autostart:     "Start with Windows",
	SpeedFmt:      "Speed: %s",
	CountFmt:      "Bypassing %d sites",
	AddSite:       "Add site",
	YourSites:     "Your sites:",
	AlwaysSites:   "Always bypassed:",
	Copy:          "Copy",
	Clear:         "Clear",
	TabBypass:     "Bypass",
	TabSites:      "Sites",
	TabLogs:       "Logs",
	TabAbout:      "About",
	TrayShow:      "Show",
	TrayQuit:      "Quit",
	AboutWhat:     "Obxod — a DPI bypass.",
	AboutOpens:    "Opens what is blocked by host name:\nDiscord, YouTube, X and sites you add.",
	AboutUnsigned: "Windows may say \"unknown publisher\" —\nthat is normal, the app is unsigned:\nMore info, then Run anyway.",
	AboutMethods:  "Try methods in turn if a site stays blocked.\nFor streams and screen share, try Bad checksum\n(it connects a bit slowly). Quickest to connect: Cut at start.\nStart with Plain or Signature.",
	LanguageLabel: "Language:",
	RestartNote:   "The language changes after a restart.",
	AutoPick:      "Auto-pick",
	Stop:          "Stop",
	Now:           "Now: %s",
	AddFav:        "Pin to top",
	DropFav:       "Unpin",
	Searching:     "Trying: %s",
	AutoPickNote:  "Only checks that the site opens.\nCheck voice and streams yourself.",
	NoneWorked:    "No method worked.",
}

func Of(code string) Lang {
	if code == "en" {
		return en
	}

	return ru
}

func (l Lang) T(x Text) string {
	return l[x]
}
