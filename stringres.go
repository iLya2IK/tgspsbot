/*===============================================================*/
/* The SPS Bot (strings)                                         */
/*                                                               */
/* Copyright 2024 Ilya Medvedkov                                 */
/*===============================================================*/

package main

type LanguageStrings struct {
	IETFCode           string
	Greetings          string
	AlreadyAuthorized  string
	NotAuthorized      string
	CommandStart       string
	CommandNewRoom     string
	CommandJoinRoom    string
	CommandCloseRoom   string
	CommandSett        string
	CommandExitRoom    string
	CommandRestartRoom string
	CommandGetStat     string
	MemberDisconnected string
	MemberConnected    string
	ChooseSPS          string
	RResYouWin         string
	RResYouLoose       string
	RResWinNobody      string
	RResRoundFinished  string
	PSTPlaying         string
	PSTWatching        string
	PSTUnknown         string
	GameFinished       string
	Congratulations    string
	UserStat           string
	EvtYourTurn        string
	EvtWaitForTurn     string
	EvtRoomFinished    string
	EvtRoomClosed      string
	RoomNotReady       string
	RoomAlreadyClosed  string
	NoRoomDetected     string
	NoParams           string
	NoSuchRoom         string
	NotValidRoom       string
	EmptyCallback      string
	RoomCreated        string
	JoinRoomInvite     string
	SetNewRoomName     string
	SetExistRoomName   string
	UnsupportedMsg     string
	RoomClosed         string
	NoActiveRooms      string
	ErrorDetected      string
}

var EN_STRINGS = LanguageStrings{
	IETFCode: "en",
	Greetings: "<b>Hello, my name is %s</b>\n" +
		"<a href=\"%s\">%s</a> to start working with the bot",
	AlreadyAuthorized: "<b>%s</b> is already in room %s",
	NotAuthorized: "<b>%s</b> is not in room now\n" +
		"<a href=\"%s\">%s</a> or <a href=\"%s\">%s</a> to start working with the bot",
	CommandStart:       "Start this bot",
	CommandNewRoom:     "Add new room",
	CommandJoinRoom:    "Join to the room",
	CommandCloseRoom:   "Close the room and start game",
	CommandSett:        "View all settings options",
	CommandExitRoom:    "Exit from current room",
	CommandRestartRoom: "Restart room \"%s\"",
	CommandGetStat:     "Get users's game statistics",

	MemberDisconnected: "Player @%s disconnected from the @%s.\"%s\" room",
	MemberConnected:    "New player @%s connected to the @%s.\"%s\" room",

	ChooseSPS:         "Choose %s",
	RResYouWin:        "You win this round!",
	RResYouLoose:      "You loose this round",
	RResWinNobody:     "Nobody wins. Try next round!",
	RResRoundFinished: "Round finished",

	PSTPlaying:  "playing",
	PSTWatching: "watching",
	PSTUnknown:  "unknown",

	GameFinished:    "Game in your room is finished",
	Congratulations: "\U0001f44f",
	UserStat:        "The game bot @%s \U0000270A\U0000270C\U0000270B introducing\nThe game statistic for @%s\n\n\U0001F973 %d\n\U0001F614 %d",

	EvtYourTurn:     "Now is your turn <b>%s</b>! Make your choose",
	EvtWaitForTurn:  "Now is the round %d in progress. Waiting",
	EvtRoomFinished: "Room @%s.\"%s\" is finished by owner",
	EvtRoomClosed:   "Room @%s.\"%s\" is closed. The game is started",

	RoomCreated:       "Room %s created",
	JoinRoomInvite:    "You was invited to play \U0000270A\U0000270C\U0000270B\n <a href=\"https://t.me/%s?start=%s_%s\">Join</a> to room <b>%s</b>",
	RoomNotReady:      "Room is not ready",
	RoomAlreadyClosed: "Room already closed",
	NoRoomDetected:    "No room detected for the user. Try to create a new one",
	NoParams:          "Parameters not received",
	NoSuchRoom:        "Room \"%s\" is not found",
	NotValidRoom:      "Requested room is not found",
	SetNewRoomName:    "%s\nSet the new room name:",
	SetExistRoomName:  "Set the exist room name:",
	EmptyCallback:     "Empty callback",
	UnsupportedMsg:    "Unsupported message format",
	RoomClosed:        "The room is closed. Try to connect later",
	NoActiveRooms:     "No active rooms",
	ErrorDetected: "<pre>Error detected</pre>\n" +
		"<pre>%s</pre>\nTry to use the bot service properly",
}

var RU_STRINGS = LanguageStrings{
	IETFCode: "ru",
	Greetings: "<b>Привет! Меня зовут %s</b>\n" +
		"<a href=\"%s\">%s</a> для начала работы с мной",
	AlreadyAuthorized: "<b>%s</b> уже в комнате %s",
	NotAuthorized: "<b>%s</b> не в комнате\n" +
		"<a href=\"%s\">%s</a> или <a href=\"%s\">%s</a> для начала работы с ботом",
	CommandStart:       "Запустить бот",
	CommandNewRoom:     "Добавить новую комнату",
	CommandJoinRoom:    "Присоединиться к комнате",
	CommandCloseRoom:   "Закрыть комнату и начать игру",
	CommandSett:        "Показать все настройки",
	CommandExitRoom:    "Выйти из текущей комнаты",
	CommandRestartRoom: "Перезапуск комнаты \"%s\"",
	CommandGetStat:     "Показать игровую статистику",

	MemberDisconnected: "Игрок @%s вышел из комнаты @%s.\"%s\"",
	MemberConnected:    "Игрок @%s зашел в комнату @%s.\"%s\"",

	ChooseSPS:         "Выбрать %s",
	RResYouWin:        "Вы выиграли этот раунд!",
	RResYouLoose:      "Вы проиграли этот раунд",
	RResWinNobody:     "Никто не победил. Попробуйте в следующем раунде!",
	RResRoundFinished: "Раунд завершен",

	PSTPlaying:  "играет",
	PSTWatching: "наблюдает",
	PSTUnknown:  "неизвестно",

	GameFinished:    "Игра в вашей комнате завершена",
	Congratulations: "\U0001f44f",
	UserStat:        "Бот @%s для игры в \U0000270A\U0000270C\U0000270B представляет\nИгровую статистику для @%s\n\n\U0001F973 %d\n\U0001F614 %d",

	EvtYourTurn:     "Сейчас ваш ход <b>%s</b>! Сделайте выбор",
	EvtWaitForTurn:  "Раунд %d в прогрессе. Ожидание",
	EvtRoomFinished: "Комната @%s.\"%s\" закрыта, игра завершена пользователем",
	EvtRoomClosed:   "Комната @%s.\"%s\" закрыта. Игра начата",

	RoomCreated:       "Комната %s создана",
	JoinRoomInvite:    "Вас пригласили для игры в \U0000270A\U0000270C\U0000270B\n <a href=\"https://t.me/%s?start=%s_%s\">Присоединитесь</a> к комнате <b>%s</b>",
	RoomNotReady:      "Комната не готова",
	RoomAlreadyClosed: "Комната уже закрыта",
	NoRoomDetected:    "Нет открытых комнат для вас. Попробуйте создать новую",
	NoParams:          "Параметры не переданы",
	NoSuchRoom:        "Комната \"%s\" не найдена",
	NotValidRoom:      "Запрашиваемая комната не найдена",
	SetNewRoomName:    "%s\nЗадайте имя комнаты:",
	SetExistRoomName:  "Задайте существующее имя комнаты:",
	EmptyCallback:     "Пустой возврат",
	UnsupportedMsg:    "Формат сообщения не поддерживается",
	NoActiveRooms:     "Нет активных комнат",
	RoomClosed:        "Комната закрыта сейчас. Попробуйте присоединиться позже",
	ErrorDetected: "<pre>Обнаружена ошибка</pre>\n" +
		"<pre>%s</pre>\nЧто-то пошло не так",
}

func GetLocale(locale string) *LanguageStrings {
	switch locale {
	case "ru":
		return &RU_STRINGS
	}
	return &EN_STRINGS
}

func DefaultLocale() *LanguageStrings {
	return &EN_STRINGS
}
