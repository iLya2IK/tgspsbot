/*===============================================================*/
/* The SPS Bot (client pool)                                     */
/*                                                               */
/* Copyright 2024 Ilya Medvedkov                                 */
/*===============================================================*/

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"reflect"
	"strings"
	"sync"
	"time"

	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

var ErrAlreadyClosed error = fmt.Errorf("already closed")
var ErrNoActiveRooms error = fmt.Errorf("no active rooms")

/* TgUserId decl */

type TgUserId struct {
	user_id int64
	chat_id int64
}

func NewUserId(uid, cid int64) TgUserId {
	return TgUserId{user_id: uid, chat_id: cid}
}

/* TgUserId impl */

func (id *TgUserId) GetChatID() int64 {
	return id.chat_id
}

func (id *TgUserId) GetUserID() int64 {
	return id.user_id
}

func (id *TgUserId) Compare(src *TgUserId) int {
	if id.user_id < src.user_id {
		return -1
	} else if id.user_id > src.user_id {
		return 1
	} else {
		if id.chat_id < src.chat_id {
			return -1
		} else if id.chat_id > src.chat_id {
			return 1
		}
	}
	return 0
}

type PoolClientStatus int

const (
	StatusWaiting     PoolClientStatus = 1
	StatusWaitNewRoom PoolClientStatus = 2
	StatusAuthorized  PoolClientStatus = 0x100
)

type PoolClientSettings struct {
}

func GenClientSettings(sett string) *PoolClientSettings {
	setts := &(PoolClientSettings{})
	json.Unmarshal([]byte(sett), setts)
	return setts
}

type PoolRoomSettings struct {
}

func GenRoomSettings(sett string) *PoolRoomSettings {
	room := &(PoolRoomSettings{})
	json.Unmarshal([]byte(sett), room)
	return room
}

type PoolPlayer struct {
	State   int   `json:"state"`
	Choose  int   `json:"choose"`
	Chooses []int `json:"chooses"`
}

func GenPoolPlayer(st string) *PoolPlayer {
	player := &(PoolPlayer{})
	json.Unmarshal([]byte(st), player)
	return player
}

/* PoolGame decl */

type PoolGame struct {
	State int `json:"state"`
	Round int `json:"round"`
}

const GST_WAITING = 0
const GST_ROOM_CLOSED_WAIT_TO_START = 1
const GST_STARTED = 2

const PST_PLAYING = 0
const PST_WATCHING = 1

func GenPoolGame(st string) *PoolGame {
	game := &(PoolGame{})
	json.Unmarshal([]byte(st), game)
	return game
}

/* PoolRoom decl */
type PoolRoom struct {
	ownername string
	ownerid   TgUserId
	name      string
	setts     *PoolRoomSettings
	state     *PoolGame
}

func ThrowRoomNotReady(local *LanguageStrings) error {
	return errors.New(local.RoomNotReady)
}

func ThrowNoRoomDetected(local *LanguageStrings) error {
	return errors.New(local.NoRoomDetected)
}

func ThrowNotValidRoom(local *LanguageStrings) error {
	return errors.New(local.NotValidRoom)
}

func ThrowRoomClosed(local *LanguageStrings) error {
	return errors.New(local.RoomClosed)
}

func NewRoom(ownername string, id TgUserId, name string) *PoolRoom {
	return &PoolRoom{ownername: ownername, ownerid: id, name: name}
}

/* PoolRoom impl */

func (room *PoolRoom) GetName() string {
	return room.name
}

func (room *PoolRoom) GetOwnerName() string {
	return room.ownername
}

func (room *PoolRoom) GetOwnerID() *TgUserId {
	return &room.ownerid
}

func (room *PoolRoom) Locate(id TgUserId, name string) bool {
	if id.Compare(&room.ownerid) == 0 &&
		strings.Compare(name, room.name) == 0 {
		return true
	}
	return false
}

func (room *PoolRoom) GetRoomSettings() *PoolRoomSettings {
	return room.setts
}

func (room *PoolRoom) GetGame() *PoolGame {
	return room.state
}

/* PoolClient decl */

type PoolClient struct {
	status PoolClientStatus

	id        TgUserId
	user_name string
	player    *PoolPlayer
	locale    *LanguageStrings
}

/* PoolClient impl */

func (c *PoolClient) GetLocale() *LanguageStrings {
	return c.locale
}

func (c *PoolClient) GetStatus() PoolClientStatus {
	return c.status
}

func (c *PoolClient) GetID() *TgUserId {
	return &c.id
}

func (c *PoolClient) GetChatID() int64 {
	return c.id.chat_id
}

func (c *PoolClient) GetUserName() string {
	return c.user_name
}

func (c *PoolClient) SetStatus(ns PoolClientStatus) {
	c.status = ns
}

func (c *PoolClient) Locate(id *TgUserId) bool {
	return (c.id.Compare(id) == 0)
}

func (c *PoolClient) GetPlayer() *PoolPlayer {
	return c.player
}

/* PoolActor decl */

type PoolActor struct {
	pool         *Pool
	client       *PoolClient
	room         *PoolRoom
	client_setts *PoolClientSettings
}

/* PoolActor impl */

func (actor *PoolActor) GetLocale() *LanguageStrings {
	if actor.client != nil {
		return actor.client.locale
	}
	return DefaultLocale()
}

func (actor *PoolActor) GetID() *TgUserId {
	if actor.client != nil {
		return actor.client.GetID()
	}
	return &TgUserId{0, 0}
}

func (actor *PoolActor) IsAuthorized() bool {
	if actor.client != nil {
		return (actor.client.GetStatus()&StatusAuthorized > 0)
	}
	return false
}

func (actor *PoolActor) GetChatID() int64 {
	if actor.client != nil {
		return actor.client.id.chat_id
	}
	return 0
}

func (actor *PoolActor) GetUserName() string {
	if actor.client != nil {
		return actor.client.user_name
	}
	return ""
}

func (actor *PoolActor) GetPool() *Pool {
	return actor.pool
}

func (actor *PoolActor) GetClient() *PoolClient {
	return actor.client
}

func (actor *PoolActor) GetRoom() *PoolRoom {
	return actor.room
}

func (actor *PoolActor) GetClientSettings() *PoolClientSettings {
	return actor.client_setts
}

func (actor *PoolActor) GetPlayer() *PoolPlayer {
	if actor.client != nil {
		return actor.client.player
	}
	return nil
}

/* StmtWrapper decl */

type StmtWrapper struct {
	stmt *sql.Stmt
}

/* StmtWrapper impl */

func PrepareStmt(db *sql.DB, sql string) (*StmtWrapper, error) {
	res := StmtWrapper{}
	var err error
	res.stmt, err = db.Prepare(sql)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (stmt *StmtWrapper) DoUpdate(bindings []any) error {

	if _, err := stmt.stmt.Exec(bindings...); err != nil {
		return err
	}

	return nil
}

type variantParam struct {
	name string
	kind reflect.Kind
}

type variantColumn struct {
	ct   *variantParam
	need bool
}

type variantScanner struct {
	dst  map[string]any
	cols []*variantColumn
	loc  int
}

func (vs *variantScanner) Scan(src any) error {
	ct := vs.cols[vs.loc]
	if ct.need {
		switch ct.ct.kind {
		case reflect.Int64, reflect.Int32, reflect.Int:
			vs.dst[ct.ct.name] = src.(int64)
		case reflect.Float32, reflect.Float64:
			vs.dst[ct.ct.name] = src.(float64)
		case reflect.String:
			vs.dst[ct.ct.name] = src.(string)
		}
	}
	vs.loc++
	return nil
}

func (stmt *StmtWrapper) doSelectRowsLimited(bindings []any, cols []variantParam, limit int) ([]map[string]any, error) {

	results := make([]map[string]any, 0)

	rows, err := stmt.stmt.Query(bindings...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	sql_cols, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	loc_cols := make([]*variantColumn, len(sql_cols))
	for i, v := range sql_cols {
		value := &variantColumn{}
		k := -1
		for j, v0 := range cols {
			if v0.name == v.Name() {
				value.ct = &cols[j]
				value.need = true
				k = j
				break
			}
		}
		if k < 0 {
			value.ct = &variantParam{name: v.Name()}
			value.need = false
		}
		loc_cols[i] = value
	}

	vS := &variantScanner{cols: loc_cols, loc: 0}
	vsArray := make([]any, len(loc_cols))
	for i := 0; i < len(loc_cols); i++ {
		vsArray[i] = vS
	}
	cnt := 0
	for rows.Next() && (cnt < limit || limit < 0) {
		result := make(map[string]any)

		vS.dst = result
		vS.loc = 0
		err := rows.Scan(vsArray...)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
		cnt++
	}

	if len(results) > 0 {
		return results, err
	}
	return nil, sql.ErrNoRows
}

func (stmt *StmtWrapper) DoSelectRow(bindings []any, cols []variantParam) (map[string]any, error) {

	results, err := stmt.doSelectRowsLimited(bindings, cols, 1)
	if err != nil {
		return nil, err
	}

	return results[0], nil
}

func (stmt *StmtWrapper) DoSelectRows(bindings []any, cols []variantParam) ([]map[string]any, error) {

	results, err := stmt.doSelectRowsLimited(bindings, cols, -1)
	if err != nil {
		return nil, err
	}

	return results, nil
}

/* Pool decl */

type PoolUpdateType int

const (
	UPD_CLIENT_DISCONNECT_ROOM PoolUpdateType = iota
	UPD_CLIENT_CONNECTED_ROOM
	UPD_ROOM_FINISHED
	UPD_ROOM_CLOSED
	UPD_ROUND_FINISHED
	UPD_YOUR_TURN
	UPD_YOU_WIN
	UPD_WAIT_FOR_TURN
	UPD_SESSION_FINISHED
	UPD_CLIENT_CLOSE_ROOM
)

type PoolUpdate struct {
	Type   PoolUpdateType
	Params []any
}

func (upd *PoolUpdate) GetPoolClient(ind int) *PoolClient {
	return upd.Params[ind].(*PoolClient)
}

func (upd *PoolUpdate) GetPoolRoom(ind int) *PoolRoom {
	return upd.Params[ind].(*PoolRoom)
}

func (upd *PoolUpdate) GetString(ind int) string {
	return upd.Params[ind].(string)
}

func (upd *PoolUpdate) GetInt(ind int) int64 {
	return upd.Params[ind].(int64)
}

type PoolUpdates chan PoolUpdate

type Pool struct {
	choose_mux sync.Mutex

	client_db *sql.DB
	// Prepares
	adduser_stmt        *StmtWrapper
	getuser_stmt        *StmtWrapper
	upduser_stmt        *StmtWrapper
	incuserstatt_stmt   *StmtWrapper
	incuserstatw_stmt   *StmtWrapper
	getuserstat_stmt    *StmtWrapper
	addroom_stmt        *StmtWrapper
	getroom_stmt        *StmtWrapper
	getroomsetts_stmt   *StmtWrapper
	getroomstate_stmt   *StmtWrapper
	updroomsetts_stmt   *StmtWrapper
	updroomstate_stmt   *StmtWrapper
	getroombyhash_stmt  *StmtWrapper
	findroombyhash_stmt *StmtWrapper
	getroombycid_stmt   *StmtWrapper
	addroomhash_stmt    *StmtWrapper
	getroomhash_stmt    *StmtWrapper
	addmember_stmt      *StmtWrapper
	rmvmember_stmt      *StmtWrapper
	clrmembers_stmt     *StmtWrapper
	resetmemberst_stmt  *StmtWrapper
	getmembers_stmt     *StmtWrapper
	getmemberids_stmt   *StmtWrapper
	getmember_stmt      *StmtWrapper
	updmemberstate_stmt *StmtWrapper

	updates PoolUpdates
}

var SETTINGS_COL = variantParam{"settings", reflect.String}
var STATE_COL = variantParam{"state", reflect.String}
var HASH_COL = variantParam{"hash", reflect.String}
var NAME_COL = variantParam{"name", reflect.String}
var EUID_COL = variantParam{"euid", reflect.Int}
var ECID_COL = variantParam{"ecid", reflect.Int}
var MUID_COL = variantParam{"muid", reflect.Int}
var MCID_COL = variantParam{"mcid", reflect.Int}
var STAT_TOTAL_COL = variantParam{"stat_total", reflect.Int}
var STAT_WON_COL = variantParam{"stat_won", reflect.Int}
var USERNAME_COL = variantParam{"user_name", reflect.String}
var ROOMNAME_COL = variantParam{"roomname", reflect.String}
var LOCALE_COL = variantParam{"locale", reflect.String}
var CNT_COL = variantParam{"cnt", reflect.Int}

/* Pool impl */

func NewPool(client_db_loc string) (*Pool, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=rwc", client_db_loc))
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("create table if not exists \"users\" (" +
		"\"user_id\" int," +
		"\"chat_id\" int," +
		"\"user_name\" text," +
		"\"locale\" text default 'en'," +
		"\"user_first_name\" text," +
		"\"user_second_name\" text," +
		"\"last_start\" text default (current_timestamp)," +
		"\"stat_total\" int default 0," +
		"\"stat_won\" int default 0," +
		"\"settings\" text default ('{}')," +
		"unique (\"user_id\", \"chat_id\"));")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("create table if not exists \"rooms\" (" +
		"\"ext_user_id\" int not null," +
		"\"ext_chat_id\" int not null," +
		"\"name\" text not null," +
		"\"last_used\" text default (current_timestamp)," +
		"\"state\" text default '{}'," +
		"\"settings\" text default ('{}')," +
		"CONSTRAINT \"rooms_fk_ext\" FOREIGN KEY (\"ext_user_id\", \"ext_chat_id\") " +
		"REFERENCES \"users\" (\"user_id\", \"chat_id\") on delete cascade," +
		"unique (\"ext_user_id\", \"ext_chat_id\", \"name\"));")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("create table if not exists \"rooms_hashes\" (" +
		"\"euid\" int not null," +
		"\"ecid\" int not null," +
		"\"roomname\" text not null," +
		"\"hash\" text not null," +
		"\"gen_at\" text default (current_timestamp)," +
		"CONSTRAINT \"rooms_hashes_fk_ext\" FOREIGN KEY (\"euid\", \"ecid\", \"roomname\") " +
		"REFERENCES \"rooms\" (\"ext_user_id\", \"ext_chat_id\", \"name\") on delete cascade," +
		"unique (\"hash\"));")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("create table if not exists \"members\" (" +
		"\"euid\" int not null," +
		"\"ecid\" int not null," +
		"\"roomname\" text not null," +
		"\"muid\" int not null," +
		"\"mcid\" int not null," +
		"\"state\" text default '{}'," +
		"CONSTRAINT \"members_fk_ext\" FOREIGN KEY (\"euid\", \"ecid\", \"roomname\") " +
		"REFERENCES \"rooms\" (\"ext_user_id\", \"ext_chat_id\", \"name\") on delete cascade," +
		"CONSTRAINT \"members_fk_ext2\" FOREIGN KEY (\"muid\", \"mcid\") " +
		"REFERENCES \"users\" (\"user_id\", \"chat_id\") on delete cascade," +
		"unique (\"muid\", \"muid\"));")
	if err != nil {
		return nil, err
	}

	pool := &(Pool{
		//terminate:  make(chan bool, 2),
		client_db: db,
		updates:   make(PoolUpdates, 128),
	})

	if pool.adduser_stmt, err = PrepareStmt(db,
		"with _ex_ as (select * from \"users\" where \"user_id\"=?1 and \"chat_id\" = ?2 limit 1)"+
			"replace into \"users\" "+
			"(\"user_id\", \"chat_id\", \"user_name\", \"locale\", \"user_first_name\", \"user_second_name\", \"last_start\", \"stat_total\", \"stat_won\", \"settings\") "+
			"values (?1, ?2, ?3, ?4, ?5, ?6, current_timestamp,"+
			"CASE WHEN EXISTS(select * from _ex_) THEN (select \"stat_total\" from _ex_) ELSE 0 end,"+
			"CASE WHEN EXISTS(select * from _ex_) THEN (select \"stat_won\" from _ex_) ELSE 0 end,"+
			"CASE WHEN EXISTS(select * from _ex_) THEN (select \"settings\" from _ex_) ELSE '{}' end);"); err != nil {
		return nil, err
	}
	if pool.upduser_stmt, err = PrepareStmt(db,
		"update \"users\" set \"settings\"=?3 where \"user_id\"=?1 and \"chat_id\"=?2;"); err != nil {
		return nil, err
	}
	if pool.incuserstatt_stmt, err = PrepareStmt(db,
		"update \"users\" set \"stat_total\"=\"stat_total\"+1 where \"user_id\"=?1 and \"chat_id\"=?2;"); err != nil {
		return nil, err
	}
	if pool.incuserstatw_stmt, err = PrepareStmt(db,
		"update \"users\" set \"stat_total\"=\"stat_total\"+1, \"stat_won\"=\"stat_won\"+1 where \"user_id\"=?1 and \"chat_id\"=?2;"); err != nil {
		return nil, err
	}
	if pool.getuser_stmt, err = PrepareStmt(db,
		"select * from \"users\" where \"user_id\"=?1 and \"chat_id\"=?2;"); err != nil {
		return nil, err
	}
	if pool.getuserstat_stmt, err = PrepareStmt(db,
		"select \"stat_total\", \"stat_won\" from \"users\" where \"user_id\"=?1 and \"chat_id\"=?2;"); err != nil {
		return nil, err
	}
	if pool.addroom_stmt, err = PrepareStmt(db,
		"with _ex_ as (select * from \"rooms\" where \"ext_user_id\"=?1 and \"ext_chat_id\" = ?2 and \"name\" = ?3 limit 1)"+
			"replace into \"rooms\" "+
			"(\"ext_user_id\", \"ext_chat_id\", \"name\", \"last_used\", \"state\", \"settings\")"+
			"values (?1, ?2, ?3, current_timestamp, "+
			"CASE WHEN EXISTS(select * from _ex_) THEN (select \"state\" from _ex_) ELSE '{}' end, "+
			"CASE WHEN EXISTS(select * from _ex_) THEN (select \"settings\" from _ex_) ELSE '{}' end);"); err != nil {
		return nil, err
	}
	if pool.getroom_stmt, err = PrepareStmt(db,
		"select \"state\", \"settings\" from \"rooms\" "+
			" where \"ext_user_id\"=?1 and \"ext_chat_id\"=?2 and \"name\"=?3;"); err != nil {
		return nil, err
	}
	if pool.updroomsetts_stmt, err = PrepareStmt(db,
		"update \"rooms\" set \"settings\"=?4 where "+
			"\"ext_user_id\"=?1 and \"ext_chat_id\"=?2 and \"name\"=?3;"); err != nil {
		return nil, err
	}
	if pool.getroomsetts_stmt, err = PrepareStmt(db,
		"select \"settings\" from \"rooms\" "+
			" where \"ext_user_id\"=?1 and \"ext_chat_id\"=?2 and \"name\"=?3;"); err != nil {
		return nil, err
	}
	if pool.updroomstate_stmt, err = PrepareStmt(db,
		"update \"rooms\" set \"state\"=?4 where "+
			"\"ext_user_id\"=?1 and \"ext_chat_id\"=?2 and \"name\"=?3;"); err != nil {
		return nil, err
	}
	if pool.getroomstate_stmt, err = PrepareStmt(db,
		"select \"state\" from \"rooms\" "+
			" where \"ext_user_id\"=?1 and \"ext_chat_id\"=?2 and \"name\"=?3;"); err != nil {
		return nil, err
	}
	if pool.addroomhash_stmt, err = PrepareStmt(db,
		"replace into \"rooms_hashes\" "+
			"(\"euid\", \"ecid\", \"roomname\", \"hash\")"+
			"values (?1, ?2, ?3, ?4);"); err != nil {
		return nil, err
	}
	if pool.getroomhash_stmt, err = PrepareStmt(db,
		"select \"hash\" from \"rooms_hashes\" "+
			"where \"euid\"==?1 and \"ecid\"==?2 and \"roomname\"==?3;"); err != nil {
		return nil, err
	}
	if pool.getroombyhash_stmt, err = PrepareStmt(db,
		"select \"roomname\", \"user_name\", \"euid\", \"ecid\" from \"rooms_hashes\" inner join \"users\" on "+
			"\"user_id\"==\"euid\" and \"chat_id\"==\"ecid\" where \"hash\" == ?1;"); err != nil {
		return nil, err
	}
	if pool.findroombyhash_stmt, err = PrepareStmt(db,
		"select count(*) as \"cnt\" from \"rooms_hashes\" where \"hash\" == ?1;"); err != nil {
		return nil, err
	}
	if pool.getroombycid_stmt, err = PrepareStmt(db,
		"select \"roomname\", \"user_name\", \"euid\", \"ecid\" from \"members\" inner join \"users\" on "+
			"\"user_id\"==\"euid\" and \"chat_id\"==\"ecid\" where \"muid\"==?1 and \"mcid\"==?2;"); err != nil {
		return nil, err
	}
	if pool.addmember_stmt, err = PrepareStmt(db,
		"replace into \"members\" "+
			"(\"euid\", \"ecid\", \"roomname\", \"muid\", \"mcid\")"+
			"values (?1, ?2, ?3, ?4, ?5);"); err != nil {
		return nil, err
	}
	if pool.rmvmember_stmt, err = PrepareStmt(db,
		"delete from \"members\" where "+
			"\"euid\"==?1 and \"ecid\"==?2 and \"roomname\"==?3 and \"muid\"==?4 and \"mcid\"==?5;"); err != nil {
		return nil, err
	}
	if pool.clrmembers_stmt, err = PrepareStmt(db,
		"delete from \"members\" where "+
			"\"euid\"==?1 and \"ecid\"==?2 and \"roomname\"==?3;"); err != nil {
		return nil, err
	}
	if pool.getmembers_stmt, err = PrepareStmt(db,
		"select \"muid\", \"mcid\", \"user_name\", \"locale\", \"state\" from \"members\" "+
			"inner join \"users\" on \"muid\"==\"user_id\" and \"mcid\" == \"chat_id\" "+
			"where \"euid\"==?1 and \"ecid\"==?2 and \"roomname\"==?3 order by \"user_name\" asc, \"mcid\" desc;"); err != nil {
		return nil, err
	}
	if pool.getmemberids_stmt, err = PrepareStmt(db,
		"select \"muid\", \"mcid\", \"locale\" from \"members\" "+
			"inner join \"users\" on \"muid\"==\"user_id\" and \"mcid\" == \"chat_id\" "+
			"where \"euid\"==?1 and \"ecid\"==?2 and \"roomname\"==?3;"); err != nil {
		return nil, err
	}
	if pool.getmember_stmt, err = PrepareStmt(db,
		"select \"user_name\", \"locale\", \"state\" from \"members\" "+
			"inner join \"users\" on \"muid\"==\"user_id\" and \"mcid\" == \"chat_id\" "+
			"where \"euid\"==?1 and \"ecid\"==?2 and \"roomname\"==?3 and \"muid\"==?4 and \"mcid\" == ?5;"); err != nil {
		return nil, err
	}
	if pool.updmemberstate_stmt, err = PrepareStmt(db,
		"update \"members\" "+
			"set \"state\"=?6 "+
			"where \"euid\"==?1 and \"ecid\"==?2 and \"roomname\"==?3 and \"muid\"==?4 and \"mcid\" == ?5;"); err != nil {
		return nil, err
	}
	if pool.resetmemberst_stmt, err = PrepareStmt(db,
		"update \"members\" "+
			"set \"state\"='{}' "+
			"where \"euid\"==?1 and \"ecid\"==?2 and \"roomname\"==?3;"); err != nil {
		return nil, err
	}

	return pool, nil
}

func (pool *Pool) NewPoolClient(id TgUserId, un string, locale *LanguageStrings) (*PoolClient, error) {
	new_pool_client := &(PoolClient{id: id, user_name: un, locale: locale})

	return new_pool_client, nil
}

func (pool *Pool) NewPoolRoom(client *PoolClient, name string) (*PoolRoom, error) {
	new_pool_room := NewRoom(client.user_name, client.id, name)

	return new_pool_room, nil
}

func (pool *Pool) GetPoolUpdates() PoolUpdates {
	/*go func() {
		for {
			// some idle work

			time.Sleep(time.Second * 5)
		}
	}()*/

	return pool.updates
}

func (pool *Pool) dbAddCID(id TgUserId, un, ietf, fn, ln string) (*PoolClientSettings, error) {
	cols, err := pool.getuser_stmt.DoSelectRow(
		[]any{id.user_id, id.chat_id},
		[]variantParam{SETTINGS_COL})
	if err != nil && (err != sql.ErrNoRows) {
		return nil, err
	}

	var sett_str string
	if err == sql.ErrNoRows {
		sett_str = "{}"
	} else {
		sett_str = cols[SETTINGS_COL.name].(string)
	}
	sett := GenClientSettings(sett_str)

	err = pool.adduser_stmt.DoUpdate(
		[]any{
			id.user_id,
			id.chat_id,
			un,
			ietf,
			fn,
			ln})
	return sett, err
}

func (pool *Pool) dbGetRoom(id TgUserId, name string, doupdate bool) (*PoolRoomSettings, *PoolGame, error) {
	cols, err := pool.getroom_stmt.DoSelectRow(
		[]any{id.user_id, id.chat_id, name},
		[]variantParam{
			SETTINGS_COL,
			STATE_COL})
	if err != nil && (err != sql.ErrNoRows) {
		return nil, nil, err
	}

	var sett_str string = "{}"
	var state_str string = "{}"

	if err == nil {
		sett_str = cols[SETTINGS_COL.name].(string)
		state_str = cols[STATE_COL.name].(string)
	}

	sett := GenRoomSettings(sett_str)
	state := GenPoolGame(state_str)

	if doupdate {
		err = pool.addroom_stmt.DoUpdate(
			[]any{
				id.user_id,
				id.chat_id,
				name})
		if err != nil {
			return nil, nil, err
		}
	}
	return sett, state, err
}

func (pool *Pool) GetRoomForClient(client *PoolClient) (*PoolRoom, error) {
	cols, err := pool.getroombycid_stmt.DoSelectRow(
		[]any{
			client.id.user_id,
			client.id.chat_id},
		[]variantParam{USERNAME_COL, ROOMNAME_COL, EUID_COL, ECID_COL})
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	euid := cols[EUID_COL.name].(int64)
	ecid := cols[ECID_COL.name].(int64)
	tgid := TgUserId{user_id: euid, chat_id: ecid}

	room, err := pool.GenRoom(
		&PoolClient{id: tgid, user_name: cols[USERNAME_COL.name].(string)},
		cols[ROOMNAME_COL.name].(string), false, client.locale)
	if err != nil {
		return nil, err
	}
	return room, nil
}

func (pool *Pool) GenRoom(client *PoolClient, name string, doupdate bool, locale *LanguageStrings) (*PoolRoom, error) {
	sett, state, err := pool.dbGetRoom(client.id, name, doupdate)
	if err != nil {
		return nil, err
	}

	room, err := pool.NewPoolRoom(client, name)
	if err != nil {
		return nil, err
	}
	room.setts = sett
	room.state = state
	return room, nil
}

func (pool *Pool) GenCID(id TgUserId, un, fn, ln string, locale *LanguageStrings) (*PoolActor, error) {

	sett, err := pool.dbAddCID(id, un, locale.IETFCode, fn, ln)
	if err != nil {
		return nil, err
	}

	client, err := pool.NewPoolClient(id, un, locale)
	if err != nil {
		return nil, err
	}

	// check if we are already in some room

	room, err := pool.GetRoomForClient(client)
	if err != nil {
		return nil, err
	}

	actor := &(PoolActor{pool: pool, client: client, client_setts: sett, room: room})

	return actor, nil
}

func (pool *Pool) GetUser(id TgUserId) (*PoolClient, error) {
	cols, err := pool.getuser_stmt.DoSelectRow(
		[]any{id.user_id, id.chat_id},
		[]variantParam{USERNAME_COL, LOCALE_COL})
	if err != nil {
		return nil, err
	}

	return pool.NewPoolClient(id,
		cols[USERNAME_COL.name].(string),
		GetLocale(cols[LOCALE_COL.name].(string)))
}

func (pool *Pool) GetUserStat(id *TgUserId) (int, int, error) {
	cols, err := pool.getuserstat_stmt.DoSelectRow(
		[]any{id.user_id, id.chat_id},
		[]variantParam{STAT_TOTAL_COL, STAT_WON_COL})
	if err != nil {
		return 0, 0, err
	}

	return int(cols[STAT_TOTAL_COL.name].(int64)), int(cols[STAT_WON_COL.name].(int64)), nil
}

func (pool *Pool) GetMembers(room *PoolRoom) ([]*PoolClient, error) {
	ids, err := pool.getmembers_stmt.DoSelectRows(
		[]any{
			room.ownerid.user_id,
			room.ownerid.chat_id,
			room.GetName()},
		[]variantParam{MUID_COL, MCID_COL, USERNAME_COL, LOCALE_COL, STATE_COL})

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	result := make([]*PoolClient, 0)
	for _, id := range ids {
		result = append(result, &PoolClient{
			id: TgUserId{
				id[MUID_COL.name].(int64),
				id[MCID_COL.name].(int64)},
			user_name: id[USERNAME_COL.name].(string),
			player:    GenPoolPlayer(id[STATE_COL.name].(string)),
			locale:    GetLocale(id[LOCALE_COL.name].(string)),
		})
	}
	return result, nil
}

func (pool *Pool) GetMemberIds(room *PoolRoom) ([]*PoolClient, error) {
	ids, err := pool.getmemberids_stmt.DoSelectRows(
		[]any{
			room.ownerid.user_id,
			room.ownerid.chat_id,
			room.GetName()},
		[]variantParam{MUID_COL, MCID_COL, LOCALE_COL})

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	result := make([]*PoolClient, 0)
	for _, id := range ids {
		result = append(result,
			&PoolClient{
				id:     TgUserId{id[MUID_COL.name].(int64), id[MCID_COL.name].(int64)},
				locale: GetLocale(id[LOCALE_COL.name].(string))})
	}
	return result, nil
}

type DoWithMember = func(room *PoolRoom, mem *PoolClient, params []any) error

func (pool *Pool) forEachMemberDo(room *PoolRoom, do_what DoWithMember, params []any) error {
	members, err := pool.GetMemberIds(room)

	if err != nil {
		return err
	}

	for _, id := range members {
		err := do_what(room, id, params)
		if err != nil {
			return err
		}
	}
	return nil
}

func (pool *Pool) AddMember(joinroom *PoolRoom, client *PoolClient) error {
	// check if we remove member
	curroom, err := pool.GetRoomForClient(client)
	if curroom != nil && err != nil &&
		((curroom.ownerid.Compare(joinroom.GetOwnerID())) != 0) {
		err := pool.ExitRoom(curroom, client)
		if err != nil {
			return err
		}
	}

	// add member to room
	err = pool.addmember_stmt.DoUpdate(
		[]any{
			joinroom.ownerid.user_id,
			joinroom.ownerid.chat_id,
			joinroom.name,
			client.id.user_id,
			client.id.chat_id})

	if err != nil {
		return err
	}

	// send connected event
	err = pool.forEachMemberDo(joinroom, func(room_ *PoolRoom, mem_ *PoolClient, params_ []any) error {
		upd := PoolUpdate{
			Type:   UPD_CLIENT_CONNECTED_ROOM,
			Params: []any{mem_, NewRoom(room_.ownername, room_.ownerid, room_.name), params_[0].(string)}}
		pool.updates <- upd
		return nil
	}, []any{client.user_name})

	return err
}

func (pool *Pool) GetRoomWithHash(client *PoolClient, hash string) (*PoolRoom, error) {

	if len(hash) == 0 {
		return nil, ThrowNotValidRoom(client.GetLocale())
	}

	cols, err := pool.getroombyhash_stmt.DoSelectRow(
		[]any{
			hash},
		[]variantParam{ROOMNAME_COL, USERNAME_COL, EUID_COL, ECID_COL})

	if err == sql.ErrNoRows {
		return nil, ThrowNotValidRoom(client.GetLocale())
	}

	if err != nil {
		return nil, err
	}

	euid := cols[EUID_COL.name].(int64)
	ecid := cols[ECID_COL.name].(int64)
	tgid := TgUserId{user_id: euid, chat_id: ecid}

	room, err := pool.GenRoom(
		&PoolClient{id: tgid, user_name: cols[USERNAME_COL.name].(string)},
		cols[ROOMNAME_COL.name].(string), true, client.locale)
	if err != nil {
		return nil, err
	}
	return room, nil
}

func (pool *Pool) AuthorizeWithHash(client *PoolClient, hash string) (*PoolRoom, error) {
	client.SetStatus(StatusWaiting)

	room, err := pool.GetRoomWithHash(client, hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ThrowNotValidRoom(client.GetLocale())
		}
		return nil, err
	}

	if room.GetGame().State != int(GST_WAITING) {
		return room, ThrowRoomClosed(client.GetLocale())
	}

	err = pool.AddMember(room, client)
	if err != nil {
		return room, err
	}
	client.SetStatus(StatusAuthorized)

	return room, nil
}

func (pool *Pool) AuthorizeToRoom(owner *PoolClient, room_name string, client *PoolClient) (*PoolRoom, error) {
	client.SetStatus(StatusWaiting)

	if len(room_name) == 0 {
		return nil, ThrowNoRoomDetected(client.GetLocale())
	}

	room, err := pool.GenRoom(owner, room_name, true, client.locale)
	if err != nil {
		return nil, err
	}
	err = pool.AddMember(room, client)
	if err != nil {
		return room, err
	}
	client.SetStatus(StatusAuthorized)
	return room, nil
}

func (pool *Pool) AuthorizeWithRoomName(client *PoolClient, name string) (*PoolRoom, error) {
	client.SetStatus(StatusWaiting)

	if len(name) == 0 {
		return nil, ThrowNoRoomDetected(client.GetLocale())
	}

	room, err := pool.GenRoom(client, name, true, client.locale)
	if err != nil {
		return nil, err
	}
	err = pool.AddMember(room, client)
	if err != nil {
		return room, err
	}
	client.SetStatus(StatusAuthorized)
	return room, nil
}

func (pool *Pool) GetHashForRoom(room *PoolRoom) (string, error) {
	cols, err := pool.getroomhash_stmt.DoSelectRow(
		[]any{
			room.ownerid.user_id,
			room.ownerid.chat_id,
			room.GetName()},
		[]variantParam{HASH_COL})
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	if err == sql.ErrNoRows {
		hashv, err := pool.GenRoomHash(room)
		if err != nil {
			return "", err
		}
		return hashv, nil
	}
	return cols[HASH_COL.name].(string), nil
}

func (pool *Pool) GenRoomHash(room *PoolRoom) (string, error) {
	var xor_id int64 = (room.ownerid.user_id ^ room.ownerid.chat_id) | room.ownerid.user_id

	var hash_value string
	for {
		bytew := bytes.NewBuffer(make([]byte, 0, 9))
		binary.Write(bytew, binary.LittleEndian, int32(xor_id&0xffffffff))
		binary.Write(bytew, binary.LittleEndian, int16((xor_id>>32)&0xffff))
		binary.Write(bytew, binary.LittleEndian, int16(time.Now().UnixMilli()%100000))
		binary.Write(bytew, binary.LittleEndian, byte(rand.IntN(256)))
		hash_value = base64.URLEncoding.EncodeToString(bytew.Bytes())
		hash_value = strings.ReplaceAll(hash_value, "-", "AA")
		hash_value = strings.ReplaceAll(hash_value, "_", "bb")
		cols, err := pool.findroombyhash_stmt.DoSelectRow(
			[]any{
				hash_value},
			[]variantParam{CNT_COL})

		if (err == sql.ErrNoRows) || (cols[CNT_COL.name].(int64) == 0) {
			break
		}
	}

	err := pool.addroomhash_stmt.DoUpdate(
		[]any{
			room.ownerid.user_id,
			room.ownerid.chat_id,
			room.name,
			hash_value,
		})
	return hash_value, err
}

func (pool *Pool) ExitRoom(room *PoolRoom, client *PoolClient) error {
	if room != nil && client != nil {
		members, err := pool.GetMemberIds(room)

		if err != nil {
			return err
		}
		if room.ownerid.Compare(client.GetID()) == 0 {
			err := pool.clrmembers_stmt.DoUpdate(
				[]any{
					room.ownerid.user_id,
					room.ownerid.chat_id,
					room.name,
				})
			if err != nil {
				return err
			}
			err = pool.UpdateRoomState(room, &PoolGame{})
			if err != nil {
				return err
			}
			// send "room finished" event
			for _, id := range members {
				upd := PoolUpdate{
					Type:   UPD_ROOM_FINISHED,
					Params: []any{id, room}}
				pool.updates <- upd
			}
		} else {
			err := pool.rmvmember_stmt.DoUpdate(
				[]any{
					room.ownerid.user_id,
					room.ownerid.chat_id,
					room.name,
					client.id.user_id,
					client.id.chat_id,
				})
			if err != nil {
				return err
			}
			// send disconnection event
			for _, id := range members {
				upd := PoolUpdate{
					Type:   UPD_CLIENT_DISCONNECT_ROOM,
					Params: []any{id, NewRoom(room.ownername, room.ownerid, room.name), client.user_name}}
				pool.updates <- upd
			}
		}
	}
	return nil
}

func (pool *Pool) RestartRoom(room *PoolRoom) error {
	if room != nil {
		state := &PoolGame{State: GST_ROOM_CLOSED_WAIT_TO_START}

		err := pool.UpdateRoomState(room, state)
		if err != nil {
			return err
		}
		err = pool.ResetMembersState(room)
		if err != nil {
			return err
		}
		// send "room closed" event
		err = pool.forEachMemberDo(room, func(room_ *PoolRoom, mem_ *PoolClient, params_ []any) error {
			upd := PoolUpdate{
				Type:   UPD_ROOM_CLOSED,
				Params: []any{mem_, room_}}
			pool.updates <- upd
			return nil
		}, []any{})
		if err == sql.ErrNoRows {
			return ErrNoActiveRooms
		}
		if err != nil {
			return err
		}
		// init game
		// give turn to the first player
		state, err = pool.GetRoomState(room)
		if err != nil {
			return err
		}
		state.Round = 0
		err = pool.UpdateRoomState(room, state)
		if err != nil {
			return err
		}
		return pool.NextRound(room)
	}
	return nil
}

func (pool *Pool) CloseRoom(room *PoolRoom) error {
	if room != nil {
		state, err := pool.GetRoomState(room)
		if err != nil {
			return err
		}

		if state.State != GST_WAITING {
			return ErrAlreadyClosed
		}

		return pool.RestartRoom(room)
	}
	return nil
}

func (pool *Pool) NotifyOwnerFinishedGame(room *PoolRoom) error {
	owner, err := pool.GetUser(room.ownerid)
	if err != nil {
		return err
	}
	upd := PoolUpdate{
		Type:   UPD_SESSION_FINISHED,
		Params: []any{owner, room}}
	pool.updates <- upd
	return nil
}

func (pool *Pool) NextRound(room *PoolRoom) error {
	members, err := pool.GetMembers(room)
	if err != nil {
		return err
	}

	state, err := pool.GetRoomState(room)
	if err != nil {
		return err
	}

	state.State = GST_STARTED
	state.Round++

	err = pool.UpdateRoomState(room, state)
	if err != nil {
		return err
	}

	for _, mem := range members {
		mem.player.Choose = 0
		err := pool.UpdateMemberState(room, mem, mem.player)
		if err != nil {
			return err
		}
		if mem.player.State == PST_PLAYING {
			upd := PoolUpdate{
				Type:   UPD_YOUR_TURN,
				Params: []any{mem, room, int64(state.Round)}}
			pool.updates <- upd
		} else {
			upd := PoolUpdate{
				Type:   UPD_WAIT_FOR_TURN,
				Params: []any{mem, int64(state.Round)}}
			pool.updates <- upd
		}
	}

	return nil
}

/* important! do not call recursively */
func (pool *Pool) UpdateMemberChoose(
	client *PoolClient,
	room *PoolRoom,
	round int64,
	choose int) error {

	pool.choose_mux.Lock()
	defer pool.choose_mux.Unlock()

	state, err := pool.GetRoomState(room)
	if err != nil {
		return err
	}

	if state.State != GST_STARTED {
		return ThrowRoomNotReady(client.GetLocale())
	}

	if (round < 1) || (round > 255) {
		return pool.ExitRoom(room, client)
	}

	mem_state, err := pool.GetMemberState(room, client)
	if err != nil {
		return err
	}

	mem_state.Choose = choose
	chooses := make([]int, round)
	if mem_state.Chooses != nil && len(mem_state.Chooses) > 0 {
		copy(chooses, mem_state.Chooses)
	}
	chooses[round-1] = choose
	mem_state.Chooses = chooses

	err = pool.UpdateMemberState(room, client, mem_state)
	if err != nil {
		return err
	}

	members, err := pool.GetMembers(room)
	if err != nil {
		return err
	}

	// check is round finished (all voted)
	var all_finish bool = true
	var vote int64 = 0

	for _, mem := range members {
		if mem.player.State == PST_PLAYING {
			if mem.player.Choose == 0 {
				all_finish = false
			}
			vote |= int64(mem.player.Choose)
		}
	}

	if all_finish {
		state, err := pool.GetRoomState(room)
		if err != nil {
			return err
		}
		state.State = GST_ROOM_CLOSED_WAIT_TO_START
		err = pool.UpdateRoomState(room, state)
		if err != nil {
			return err
		}

		var winner int64

		switch vote {
		case 3:
			winner = 1
		case 5:
			winner = 4
		case 6:
			winner = 2
		default:
			winner = 0 // nobody wins
		}

		var playing_now int = 0
		var winner_mem *PoolClient = nil

		for _, mem := range members {
			var a_state int64 = int64(mem.player.State)

			if a_state == PST_PLAYING {
				if mem.player.Choose != int(winner) && winner != 0 {
					mem.player.State = PST_WATCHING
					err = pool.UpdateMemberState(room, mem, mem.player)
					if err != nil {
						return err
					}
					err = pool.incClientStat(mem, loss)
					if err != nil {
						return err
					}
				}
			}

			if mem.player.State == PST_PLAYING {
				playing_now++
				winner_mem = mem
			}

			upd := PoolUpdate{
				Type:   UPD_ROUND_FINISHED,
				Params: []any{mem, room, winner, a_state}}
			pool.updates <- upd
		}

		if playing_now <= 1 {
			if len(members) > 1 {
				if winner_mem != nil {
					err = pool.incClientStat(winner_mem, won)
					if err != nil {
						return err
					}
					upd := PoolUpdate{
						Type:   UPD_YOU_WIN,
						Params: []any{winner_mem, room}}
					pool.updates <- upd
				}
			}

			return pool.NotifyOwnerFinishedGame(room)
		} else {
			return pool.NextRound(room)
		}
	}

	return nil
}

func (pool *Pool) setClientSettingsValue(client *PoolClient, sett *PoolClientSettings, value string) error {
	if len(value) == 0 {
		value = "some default value"
	}
	// sett.SomeValue = value
	return pool.updateClientSettings(client, sett)
}

func (pool *Pool) updateClientSettings(client *PoolClient, sett *PoolClientSettings) error {
	json_str, err := json.Marshal(*sett)
	if err != nil {
		return err
	}

	err = pool.upduser_stmt.DoUpdate(
		[]any{
			client.id.user_id,
			client.id.chat_id,
			string(json_str)})
	return err
}

type gameResult int

const (
	won gameResult = iota
	loss
)

func (pool *Pool) incClientStat(client *PoolClient, res gameResult) error {
	switch res {
	case won:
		{
			err := pool.incuserstatw_stmt.DoUpdate(
				[]any{
					client.id.user_id,
					client.id.chat_id})
			return err
		}
	case loss:
		{
			err := pool.incuserstatt_stmt.DoUpdate(
				[]any{
					client.id.user_id,
					client.id.chat_id})
			return err
		}
	}
	return nil
}

func (pool *Pool) GetMemberState(room *PoolRoom, client *PoolClient) (*PoolPlayer, error) {
	cols, err := pool.getmember_stmt.DoSelectRow(
		[]any{
			room.ownerid.user_id,
			room.ownerid.chat_id,
			room.GetName(),
			client.id.user_id,
			client.id.chat_id},
		[]variantParam{STATE_COL})

	if err != nil {
		return GenPoolPlayer("{}"), err
	}

	return GenPoolPlayer(cols[STATE_COL.name].(string)), nil
}

func (pool *Pool) UpdateMemberState(room *PoolRoom, client *PoolClient, state *PoolPlayer) error {
	if room == nil {
		return ThrowNoRoomDetected(client.GetLocale())
	}

	json_str, err := json.Marshal(*state)
	if err != nil {
		return err
	}

	err = pool.updmemberstate_stmt.DoUpdate(
		[]any{
			room.ownerid.user_id,
			room.ownerid.chat_id,
			room.GetName(),
			client.id.user_id,
			client.id.chat_id,
			string(json_str)})
	return err
}

func (pool *Pool) getClientRoomSettings(client *PoolClient, name string) (string, error) {
	cols, err := pool.getroomsetts_stmt.DoSelectRow(
		[]any{
			client.id.user_id,
			client.id.chat_id,
			name},
		[]variantParam{SETTINGS_COL})

	if err != nil {
		return "", err
	}

	return cols[SETTINGS_COL.name].(string), nil
}

func (pool *Pool) updateClientRoomSettings(client *PoolClient, room string, sett *PoolRoomSettings) error {
	if len(room) == 0 {
		return ThrowNoRoomDetected(client.GetLocale())
	}

	json_str, err := json.Marshal(*sett)
	if err != nil {
		return err
	}

	err = pool.updroomsetts_stmt.DoUpdate(
		[]any{
			client.id.user_id,
			client.id.chat_id,
			room,
			string(json_str)})
	return err
}

func (pool *Pool) GetRoomState(room *PoolRoom) (*PoolGame, error) {
	cols, err := pool.getroomstate_stmt.DoSelectRow(
		[]any{
			room.ownerid.user_id,
			room.ownerid.chat_id,
			room.GetName()},
		[]variantParam{STATE_COL})

	if err != nil {
		return GenPoolGame("{}"), err
	}

	state := GenPoolGame(cols[STATE_COL.name].(string))
	return state, nil
}

func (pool *Pool) UpdateRoomState(room *PoolRoom, state *PoolGame) error {

	json_str, err := json.Marshal(*state)
	if err != nil {
		return err
	}

	err = pool.updroomstate_stmt.DoUpdate(
		[]any{
			room.ownerid.user_id,
			room.ownerid.chat_id,
			room.name,
			string(json_str)})
	return err
}

func (pool *Pool) ResetMembersState(room *PoolRoom) error {
	err := pool.resetmemberst_stmt.DoUpdate(
		[]any{
			room.ownerid.user_id,
			room.ownerid.chat_id,
			room.name})
	return err
}

type BufferReader struct {
	name string
	id   int64
	data *bytes.Buffer
}

func (reader *BufferReader) IsEmpty() bool {
	if reader.data == nil {
		return true
	}
	return reader.data.Len() == 0
}

func (reader *BufferReader) GetId() int64 {
	return reader.id
}

// NeedsUpload shows if the file needs to be uploaded.
func (reader *BufferReader) NeedsUpload() bool {
	return true
}

// UploadData gets the file name and an `io.Reader` for the file to be uploaded. This
// must only be called when the file needs to be uploaded.
func (reader *BufferReader) UploadData() (string, io.Reader, error) {
	return reader.name, reader.data, nil
}

// SendData gets the file data to send when a file does not need to be uploaded. This
// must only be called when the file does not need to be uploaded.
func (reader *BufferReader) SendData() string {
	return fmt.Sprintf("Cant upload %s", reader.name)
}
