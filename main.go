/*===============================================================*/
/* The SPS Bot                                                   */
/*                                                               */
/* Copyright 2024 Ilya Medvedkov                                 */
/*===============================================================*/

package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const PM_HTML = "HTML"

func check(e error) {
	if e != nil {
		log.Panic(e)
	}
}

type APIBotDebugConfig struct {
	Enabled            bool   `json:"enabled"`
	Endpoint           string `json:"endpoint"`
	Token              string `json:"token"`
	InsecureSkipVerify bool   `json:"skip_verify"`
}

type BotConfig struct {
	Database string            `json:"db"`
	BotToken string            `json:"token"`
	Timeout  int               `json:"timeout"`
	Debug    bool              `json:"debug"`
	APIDebug APIBotDebugConfig `json:"api_debug"`
}

const TG_COMMAND_START = "/start"
const TG_COMMAND_NEWROOM = "/newroom"
const TG_COMMAND_CLOSEROOM = "/closeroom"
const TG_COMMAND_JOINROOM = "/joinroom"
const TG_COMMAND_EXITROOM = "/exitroom"
const TG_COMMAND_CHOOSE = "/choose"
const TG_COMMAND_STAT = "/stat"
const TG_COMMAND_RESTARTROOM = "/restart"

const CHOOSE_STONE = 1
const CHOOSE_SCISSORS = 2
const CHOOSE_PAPER = 4

const SIGN_STONE = "\U0000270A"
const SIGN_SCISSORS = "\U0000270C"
const SIGN_PAPER = "\U0000270B"

func ErrorToString(err error) string {
	return fmt.Sprintf("%v", err)
}

/* Global Commands */

func PSTToStr(st int, locale *LanguageStrings) string {
	switch st {
	case PST_PLAYING:
		return locale.PSTPlaying
	case PST_WATCHING:
		return locale.PSTWatching
	default:
		return locale.PSTUnknown
	}
}

func ParseCommand(msg string) (string, []string) {
	var seq []string
	m1 := regexp.MustCompile(`([^a-zA-Z0-9_\&\/])+`)
	msg = m1.ReplaceAllString(msg, "_")
	if strings.Contains(msg, "&") {
		seq = strings.Split(msg, "&")
	} else {
		seq = strings.Split(msg, "_")
	}
	if len(seq) > 0 {
		if strings.HasPrefix(seq[0], "/") {
			return seq[0], seq[1:]
		}
	}
	return msg, nil
}

func CollapseParams(params []string) {
	if len(params) > 1 {
		res := ""
		for _, v := range params {
			if len(res) > 0 {
				res += "_"
			}
			res += v
		}
		params[0] = res
	}
}

func ExtractArgsFromTgUpdate(clientpool *Pool, update *tgbotapi.Update) (*PoolActor, error) {
	var cid int64
	var locale *LanguageStrings
	var from *tgbotapi.User

	if update.CallbackQuery != nil { // If we got a callback
		from = update.CallbackQuery.From
		cid = update.CallbackQuery.Message.Chat.ID
	} else if update.Message != nil { // If we got a message
		cid = update.Message.Chat.ID
		from = update.Message.From
	}
	tgid := NewUserId(from.ID, cid)
	locale = GetLocale(from.LanguageCode)
	actor, err := clientpool.GenCID(tgid, from.UserName, from.FirstName, from.LastName, locale)

	return actor, err
}

func ExtractCommandFromEntities(msg string, entities []tgbotapi.MessageEntity) (string, []string, bool) {
	cur_cmd := ""
	var params []string = make([]string, 0)
	for _, ent := range entities {
		switch ent.Type {
		case "bot_command":
			{
				str := string([]rune(msg)[ent.Offset : ent.Offset+ent.Length])
				cur_cmd, params = ParseCommand(str)
				CollapseParams(params)
			}
		case "pre":
			{
				str := string([]rune(msg)[ent.Offset : ent.Offset+ent.Length])
				cur_cmd, params = ParseCommand(str)
			}
		}
	}
	return cur_cmd, params, (len(cur_cmd) > 0)
}

func PrepareInitCommands(tid TgUserId, locale *LanguageStrings) tgbotapi.Chattable {
	req := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: TG_COMMAND_START, Description: locale.CommandStart},
		tgbotapi.BotCommand{Command: TG_COMMAND_STAT, Description: locale.CommandGetStat},
		tgbotapi.BotCommand{Command: TG_COMMAND_NEWROOM, Description: locale.CommandNewRoom},
		tgbotapi.BotCommand{Command: TG_COMMAND_EXITROOM, Description: locale.CommandExitRoom},
	)
	if tid.GetChatID() != 0 {
		req.Scope = &tgbotapi.BotCommandScope{Type: "chat", ChatID: tid.GetChatID()} //, UserID: tid.user_id}
	} else {
		req.Scope = &tgbotapi.BotCommandScope{Type: "all_private_chats"}
		if locale != DefaultLocale() {
			req.LanguageCode = locale.IETFCode
		}
	}

	return req
}

func PrepareDoLog(chatid int64, value string) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(chatid, value)
	msg.ParseMode = PM_HTML

	return msg
}

func PrepareAuthorized(actor *PoolActor) tgbotapi.MessageConfig {
	// if already authorized
	msg := tgbotapi.NewMessage(actor.GetChatID(),
		fmt.Sprintf(actor.GetLocale().AlreadyAuthorized, actor.GetUserName()))
	msg.ParseMode = PM_HTML
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				actor.GetLocale().CommandExitRoom,
				TG_COMMAND_EXITROOM),
		})
	return msg
}

func PrepareToAuthorize(actor *PoolActor) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(actor.GetChatID(),
		fmt.Sprintf(actor.GetLocale().NotAuthorized, actor.GetUserName(),
			TG_COMMAND_NEWROOM, TG_COMMAND_NEWROOM,
			TG_COMMAND_JOINROOM, TG_COMMAND_JOINROOM))
	msg.ParseMode = PM_HTML
	return msg
}

/* Bot handler */

type BotHandler struct {
	Bot      *tgbotapi.BotAPI
	ChatId   int64
	Actor    *PoolActor // may be nil
	Command  *string    // may be nil
	Params   []string   // may be nil
	ErrorStr string
}

func NewHandler(bot *tgbotapi.BotAPI, actor *PoolActor) *BotHandler {
	return &BotHandler{Bot: bot, Actor: actor}
}

func NewCommandHandler(bot *tgbotapi.BotAPI, actor *PoolActor, command *string, params []string) *BotHandler {
	return &BotHandler{Bot: bot, Actor: actor, Command: command, Params: params}
}

func (handler *BotHandler) GetLocale() *LanguageStrings {
	if handler.Actor != nil {
		return handler.Actor.GetLocale()
	} else {
		return DefaultLocale()
	}
}

func (handler *BotHandler) Send(msg tgbotapi.Chattable) {
	if handler.Bot != nil {
		handler.Bot.Send(msg)
	}
}

func (handler *BotHandler) GetUserName() string {
	if handler.Actor != nil {
		return handler.Actor.GetUserName()
	} else {
		return ""
	}
}

func (handler *BotHandler) GetChatID() int64 {
	if handler.Actor != nil {
		return handler.Actor.GetChatID()
	} else {
		return handler.ChatId
	}
}

func (handler *BotHandler) GetParamCnt() int {
	if handler.Params == nil {
		return 0
	}
	return len(handler.Params)
}

func (handler *BotHandler) GetParamAsInt64(id int) (int64, error) {
	i, err := strconv.ParseInt(handler.Params[id], 10, 64)
	if err != nil {
		return -1, err
	}
	return i, nil
}

func (handler *BotHandler) HandleStart() {
	if handler.Actor.IsAuthorized() {
		handler.Send(PrepareAuthorized(handler.Actor))
	} else {
		// if not authorized
		msg := tgbotapi.NewMessage(handler.GetChatID(),
			fmt.Sprintf(handler.GetLocale().Greetings,
				handler.Bot.Self.UserName,
				TG_COMMAND_NEWROOM, TG_COMMAND_NEWROOM))
		msg.ParseMode = PM_HTML
		handler.Send(msg)
	}
}

func (handler *BotHandler) HandleNewRoom() {
	if handler.Actor.IsAuthorized() {
		handler.Send(PrepareAuthorized(handler.Actor))
	} else {
		// if not authorized
		msg := tgbotapi.NewMessage(handler.GetChatID(),
			fmt.Sprintf(handler.GetLocale().SetNewRoomName,
				TG_COMMAND_NEWROOM))
		msg.ParseMode = PM_HTML
		msg.ReplyMarkup = tgbotapi.ForceReply{
			ForceReply:            true,
			InputFieldPlaceholder: "room",
		}
		handler.Send(msg)
	}
}

func (handler *BotHandler) HandleJoinRoom() {
	if len(handler.Params) == 0 {
		handler.ErrorStr = fmt.Sprintf(handler.GetLocale().NoSuchRoom, ".")
		return
	}

	_, err := handler.Actor.GetPool().AuthorizeWithHash(handler.Actor.GetClient(), handler.Params[0])
	if err != nil {
		handler.ErrorStr = ErrorToString(err)
		return
	}
}

func (handler *BotHandler) HandleGetStat() {
	t, w, err := handler.Actor.GetPool().GetUserStat(handler.Actor.GetID())
	if err != nil {
		handler.ErrorStr = ErrorToString(err)
		return
	}
	msg := tgbotapi.NewMessage(handler.GetChatID(),
		fmt.Sprintf(handler.GetLocale().UserStat,
			handler.Bot.Self.UserName,
			handler.Actor.GetUserName(),
			w, t-w))
	msg.ParseMode = PM_HTML
	handler.Send(msg)
}

func (handler *BotHandler) HandleNewRoomInput(new_name string) {
	if handler.Actor.IsAuthorized() {
		handler.Send(PrepareAuthorized(handler.Actor))
	} else {
		// if not authorized
		room, err := handler.Actor.GetPool().GenRoom(handler.Actor.GetClient(), new_name, true, handler.GetLocale())
		if err != nil {
			handler.ErrorStr = ErrorToString(err)
			return
		}
		err = handler.Actor.GetPool().ExitRoom(room, handler.Actor.GetClient())
		if err != nil {
			handler.ErrorStr = ErrorToString(err)
			return
		}

		msg := tgbotapi.NewMessage(handler.GetChatID(),
			fmt.Sprintf(handler.GetLocale().RoomCreated,
				room.GetName()))
		msg.ParseMode = PM_HTML
		handler.Send(msg)

		token, err := handler.Actor.GetPool().GetHashForRoom(room)
		if err != nil {
			handler.ErrorStr = ErrorToString(err)
			return
		}

		// gen message to send join invitation
		msg = tgbotapi.NewMessage(handler.GetChatID(),
			fmt.Sprintf(handler.GetLocale().JoinRoomInvite,
				handler.Bot.Self.UserName, TG_COMMAND_JOINROOM[1:], token, room.GetName()))
		msg.ParseMode = PM_HTML

		handler.Send(msg)
	}
}

func main() {
	cfgFile, err := os.Open("config.json")
	check(err)
	byteValue, err := io.ReadAll(cfgFile)
	check(err)
	var bot_cfg BotConfig
	err = json.Unmarshal(byteValue, &bot_cfg)
	check(err)
	cfgFile.Close()

	clientpool, err := NewPool(bot_cfg.Database)
	check(err)

	var bot *tgbotapi.BotAPI
	// debug cases only
	if bot_cfg.APIDebug.Enabled {
		tr := http.DefaultTransport.(*http.Transport).Clone()

		if bot_cfg.APIDebug.InsecureSkipVerify {
			tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		}
		client := &http.Client{Transport: tr}
		bot, err = tgbotapi.NewBotAPIWithClient(bot_cfg.APIDebug.Token, bot_cfg.APIDebug.Endpoint, client)
	} else {
		bot, err = tgbotapi.NewBotAPI(bot_cfg.BotToken)
	}
	check(err)

	bot.Debug = bot_cfg.Debug

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = bot_cfg.Timeout

	// prepare chans
	stop := make(chan int)
	pool_updates := clientpool.GetPoolUpdates()
	updates := bot.GetUpdatesChan(u)
	// start TG handler
	go func() {
		for update := range pool_updates {
			switch update.Type {
			case UPD_CLIENT_DISCONNECT_ROOM:
				{
					var to_whom *PoolClient = update.GetPoolClient(0)
					var from_room *PoolRoom = update.GetPoolRoom(1)
					var user_name string = update.GetString(2)

					txt := fmt.Sprintf(to_whom.GetLocale().MemberDisconnected, user_name, from_room.GetOwnerName(), from_room.GetName())

					msg := tgbotapi.NewMessage(to_whom.GetChatID(), txt)
					msg.ParseMode = PM_HTML

					bot.Send(msg)
				}
			case UPD_CLIENT_CONNECTED_ROOM:
				{
					var to_whom *PoolClient = update.GetPoolClient(0)
					var from_room *PoolRoom = update.GetPoolRoom(1)
					var user_name string = update.GetString(2)

					txt := fmt.Sprintf(to_whom.GetLocale().MemberConnected, user_name, from_room.GetOwnerName(), from_room.GetName())

					msg := tgbotapi.NewMessage(to_whom.GetChatID(), txt)
					msg.ParseMode = PM_HTML

					if from_room.GetOwnerID().Compare(to_whom.GetID()) == 0 {
						msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
							[]tgbotapi.InlineKeyboardButton{
								tgbotapi.NewInlineKeyboardButtonData(
									to_whom.GetLocale().CommandCloseRoom,
									TG_COMMAND_CLOSEROOM),
							})
					}

					bot.Send(msg)
				}
			case UPD_ROOM_FINISHED:
				{
					var to_whom *PoolClient = update.GetPoolClient(0)
					var from_room *PoolRoom = update.GetPoolRoom(1)

					txt := fmt.Sprintf(to_whom.GetLocale().EvtRoomFinished, from_room.GetOwnerName(), from_room.GetName())

					msg := tgbotapi.NewMessage(to_whom.GetChatID(), txt)
					msg.ParseMode = PM_HTML

					bot.Send(msg)
				}
			case UPD_ROOM_CLOSED:
				{
					var to_whom *PoolClient = update.GetPoolClient(0)
					var from_room *PoolRoom = update.GetPoolRoom(1)

					txt := fmt.Sprintf(to_whom.GetLocale().EvtRoomClosed, from_room.GetOwnerName(), from_room.GetName())

					msg := tgbotapi.NewMessage(to_whom.GetChatID(), txt)
					msg.ParseMode = PM_HTML

					bot.Send(msg)
				}
			case UPD_YOUR_TURN:
				{
					var to_whom *PoolClient = update.GetPoolClient(0)
					var room *PoolRoom = update.GetPoolRoom(1)
					var turn int64 = update.GetInt(2)

					txt := fmt.Sprintf(to_whom.GetLocale().EvtYourTurn, to_whom.GetUserName())
					hash, err := clientpool.GetHashForRoom(room)
					if err != nil {
						break
					}

					msg := tgbotapi.NewMessage(to_whom.GetChatID(), txt)
					msg.ParseMode = PM_HTML
					msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
						[]tgbotapi.InlineKeyboardButton{
							tgbotapi.NewInlineKeyboardButtonData(
								fmt.Sprintf(to_whom.GetLocale().ChooseSPS, SIGN_STONE),
								fmt.Sprintf("%s&%d&%d&%s",
									TG_COMMAND_CHOOSE, CHOOSE_STONE, turn, hash)),
						},
						[]tgbotapi.InlineKeyboardButton{
							tgbotapi.NewInlineKeyboardButtonData(
								fmt.Sprintf(to_whom.GetLocale().ChooseSPS, SIGN_SCISSORS),
								fmt.Sprintf("%s&%d&%d&%s",
									TG_COMMAND_CHOOSE, CHOOSE_SCISSORS, turn, hash)),
						},
						[]tgbotapi.InlineKeyboardButton{
							tgbotapi.NewInlineKeyboardButtonData(
								fmt.Sprintf(to_whom.GetLocale().ChooseSPS, SIGN_PAPER),
								fmt.Sprintf("%s&%d&%d&%s",
									TG_COMMAND_CHOOSE, CHOOSE_PAPER, turn, hash)),
						},
						[]tgbotapi.InlineKeyboardButton{
							tgbotapi.NewInlineKeyboardButtonData(
								to_whom.GetLocale().CommandExitRoom,
								TG_COMMAND_EXITROOM),
						})

					bot.Send(msg)
				}
			case UPD_WAIT_FOR_TURN:
				{
					var to_whom *PoolClient = update.GetPoolClient(0)
					var turn int64 = update.GetInt(1)

					txt := fmt.Sprintf(to_whom.GetLocale().EvtWaitForTurn, turn)

					msg := tgbotapi.NewMessage(to_whom.GetChatID(), txt)
					msg.ParseMode = PM_HTML
					msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
						[]tgbotapi.InlineKeyboardButton{
							tgbotapi.NewInlineKeyboardButtonData(
								to_whom.GetLocale().CommandExitRoom,
								TG_COMMAND_EXITROOM),
						})

					bot.Send(msg)
				}
			case UPD_SESSION_FINISHED:
				{
					var owner *PoolClient = update.GetPoolClient(0)
					var room *PoolRoom = update.GetPoolRoom(1)

					hash, err := clientpool.GetHashForRoom(room)
					if err != nil {
						break
					}

					msg := tgbotapi.NewMessage(owner.GetChatID(), owner.GetLocale().GameFinished)
					msg.ParseMode = PM_HTML
					msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
						[]tgbotapi.InlineKeyboardButton{
							tgbotapi.NewInlineKeyboardButtonData(
								fmt.Sprintf(owner.GetLocale().CommandRestartRoom,
									room.GetName()),
								fmt.Sprintf("%s&%s",
									TG_COMMAND_RESTARTROOM, hash)),
						})

					bot.Send(msg)
				}
			case UPD_ROUND_FINISHED:
				{
					// show statistics
					var to_whom *PoolClient = update.GetPoolClient(0)
					var room *PoolRoom = update.GetPoolRoom(1)
					var winner int64 = update.GetInt(2)
					var prev_state int64 = update.GetInt(3)

					members, err := clientpool.GetMembers(room)
					if err != nil {
						break
					}

					var title string
					if prev_state == PST_WATCHING {
						title = to_whom.GetLocale().RResRoundFinished
					} else {
						if winner == 0 {
							title = to_whom.GetLocale().RResWinNobody
						} else if to_whom.GetPlayer().State == PST_PLAYING {
							title = to_whom.GetLocale().RResYouWin
						} else {
							title = to_whom.GetLocale().RResYouLoose
						}
					}

					var b strings.Builder
					b.WriteString(title)
					b.WriteByte(0xA)
					b.WriteByte(0xA)
					for _, mem := range members {
						b.WriteString(fmt.Sprintf("<b>%s</b> (%s)\n",
							mem.GetUserName(), PSTToStr(mem.GetPlayer().State, mem.GetLocale())))

						chooses := mem.GetPlayer().Chooses
						if len(chooses) > 0 {
							for _, choose := range chooses {
								switch choose {
								case CHOOSE_STONE:
									b.WriteString(SIGN_STONE)
								case CHOOSE_SCISSORS:
									b.WriteString(SIGN_SCISSORS)
								case CHOOSE_PAPER:
									b.WriteString(SIGN_PAPER)
								}
							}
						}
						b.WriteByte(0xA)
					}

					msg := tgbotapi.NewMessage(to_whom.GetChatID(), b.String())
					msg.ParseMode = PM_HTML
					msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
						[]tgbotapi.InlineKeyboardButton{
							tgbotapi.NewInlineKeyboardButtonData(
								to_whom.GetLocale().CommandExitRoom,
								TG_COMMAND_EXITROOM),
						})

					bot.Send(msg)
				}
			case UPD_YOU_WIN:
				{
					var winner *PoolClient = update.GetPoolClient(0)
					msg := tgbotapi.NewMessage(winner.GetChatID(), winner.GetLocale().Congratulations)
					msg.ParseMode = PM_HTML

					bot.Send(msg)
				}
			}
		}
	}()

	// start TG handler
	go func() {
		//initialization
		bot.Send(PrepareInitCommands(TgUserId{0, 0}, DefaultLocale()))
		bot.Send(PrepareInitCommands(TgUserId{0, 0}, &RU_STRINGS))

		for update := range updates {
			// try to extract ids and find the corresponding client object
			actor, err := ExtractArgsFromTgUpdate(clientpool, &update)
			if actor != nil {
				// declare the error holder variable
				var error_str string
				if err != nil {
					error_str = ErrorToString(err)
				}

				if update.CallbackQuery != nil { // If we got a callback
					comm, params := ParseCommand(update.CallbackQuery.Data)
					handler := NewCommandHandler(bot, actor, &comm, params)

					switch comm {
					case TG_COMMAND_CHOOSE:
						{
							// choose_id, turn_num, room_hash
							if handler.GetParamCnt() > 2 {
								choose_id, err := handler.GetParamAsInt64(0)
								if err != nil {
									handler.ErrorStr = ErrorToString(err)
									break
								}
								turn_num, err := handler.GetParamAsInt64(1)
								if err != nil {
									handler.ErrorStr = ErrorToString(err)
									break
								}
								room_hash := handler.Params[2]
								room, err := clientpool.GetRoomWithHash(actor.GetClient(), room_hash)
								if err != nil {
									handler.ErrorStr = ErrorToString(err)
									break
								}
								clientpool.UpdateMemberChoose(actor.GetClient(), room, turn_num, int(choose_id))
							}
						}
					case TG_COMMAND_RESTARTROOM:
						{
							// room_hash
							if handler.GetParamCnt() > 0 {
								room_hash := handler.Params[0]
								room, err := clientpool.GetRoomWithHash(actor.GetClient(), room_hash)
								if err != nil {
									handler.ErrorStr = ErrorToString(err)
									break
								}
								if room.GetOwnerID().Compare(handler.Actor.GetID()) == 0 {
									err = clientpool.RestartRoom(room)
									if err != nil {
										if err == ErrNoActiveRooms {
											handler.ErrorStr = handler.GetLocale().NoActiveRooms
										} else {
											handler.ErrorStr = ErrorToString(err)
										}
									}
								}
							}
						}
					case TG_COMMAND_CLOSEROOM:
						{
							if handler.Actor.GetRoom() != nil {
								if handler.Actor.GetID().Compare(handler.Actor.GetRoom().GetOwnerID()) == 0 {
									err := clientpool.CloseRoom(handler.Actor.GetRoom())
									if err != nil {
										if err == ErrAlreadyClosed {
											handler.ErrorStr = handler.GetLocale().RoomAlreadyClosed
										} else {
											handler.ErrorStr = ErrorToString(err)
										}
									}
								}
							}
						}
					case TG_COMMAND_EXITROOM:
						{
							if handler.Actor.GetRoom() != nil {
								err := clientpool.ExitRoom(handler.Actor.GetRoom(), handler.Actor.GetClient())
								if err == ErrNoActiveRooms {
									handler.ErrorStr = handler.GetLocale().NoActiveRooms
								}
							}
						}
					}
					error_str = handler.ErrorStr

					bot.Send(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))
				} else if update.Message != nil { // If we got a message

					comm, params := ParseCommand(update.Message.Text)
					if comm == TG_COMMAND_START && len(params) > 0 {
						comm = "/" + params[0]
						if len(params) > 1 {
							params = params[1:]
						} else {
							params = nil
						}
					}
					handler := NewCommandHandler(bot, actor, &comm, params)

					// first check if this is the common command
					switch comm {
					case TG_COMMAND_START:
						{
							handler.HandleStart()
						}
					case TG_COMMAND_NEWROOM:
						{
							handler.HandleNewRoom()
						}
					case TG_COMMAND_JOINROOM:
						{
							handler.HandleJoinRoom()
						}
					case TG_COMMAND_STAT:
						{
							handler.HandleGetStat()
						}
					case TG_COMMAND_EXITROOM:
						{
							if handler.Actor.GetRoom() != nil {
								clientpool.ExitRoom(handler.Actor.GetRoom(), handler.Actor.GetClient())
							}
						}
					default:
						{
							// no. this is not the common command
							if (update.Message.ReplyToMessage != nil) &&
								(len(update.Message.ReplyToMessage.Entities) > 0) { // check if this is the reply for some editor
								// extract json obj (editor state)
								cur_cmd, params, ok := ExtractCommandFromEntities(
									update.Message.ReplyToMessage.Text,
									update.Message.ReplyToMessage.Entities)

								if ok {
									// Yes, we have a message in the editor's business logic section
									handler := NewCommandHandler(bot, actor, &cur_cmd, params)
									switch cur_cmd {
									case TG_COMMAND_NEWROOM:
										{
											handler.HandleNewRoomInput(update.Message.Text)
										}
									}
								}
							} else {
								handler.ErrorStr = actor.GetLocale().UnsupportedMsg
							}
						}
					}
					error_str = handler.ErrorStr
				}
				if len(error_str) > 0 && (actor.GetChatID() > 0) {
					bot.Send(PrepareDoLog(actor.GetChatID(),
						fmt.Sprintf(DefaultLocale().ErrorDetected, error_str)))
				}
			}
		}
		stop <- 1
	}()

	for loop := true; loop; {
		select {
		case <-stop:
			{
				loop = false
				close(stop)
				close(pool_updates)
				break
			}
		default:
			time.Sleep(250 * time.Millisecond)
		}
	}

}
