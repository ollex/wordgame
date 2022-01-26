package main

import (
	"sync"

	"zombiezen.com/go/sqlite/sqlitex"
)

type Event struct {
	// Events are pushed to this channel by the main events-gathering routine
	Message chan string

	// New client connections
	NewClients chan chan string

	// Closed client connections
	ClosedClients chan chan string

	// Total client connections
	TotalClients map[chan string]bool
}

type ClientChan chan string

type Application struct {
	db              *sqlitex.Pool
	isBackupRunning int32
	wg              sync.WaitGroup
	game            *GoGame
	Words           string
}

type Person struct {
	Name string `form:"username"`
	Pw   string `form:"pw"`
}

type User struct {
	Name     string
	Pw       string
	Id       int64
	Username string
	Role     string
	Points   int64
}

type ChatMsg struct {
	Msg string `json:"msg"`
}

type AddWord struct {
	Words []string `json:"words"`
}

type Word struct {
	Id   int64
	Word string
}

type Letter struct {
	C      rune  `json:"l"`
	Points uint8 `json:"p"`
	Exist  uint8 `json:"-"`
}

type Game struct {
	Id    int64
	Text  string
	Datum string
}

type PlayField struct {
	F    uint8
	Fac  uint8
	Word bool
}

type PlayedField struct {
	Pf      uint8  `json:"position"`
	Player  string `json:"-"`
	Char    rune   `json:"rune"`
	IsJoker bool
}

type UserPlayedField struct {
	Position uint8  `json:"position"`
	Str      string `json:"rune"`
	IsJoker  bool   `json:"joker"`
}

type Player struct {
	Name   string `json:"name"`
	Points int    `json:"points"`
	Runes  []rune `json:"-"`
}

type Direction int

const (
	X Direction = iota
	Y
)

type WordLine struct {
	Dir    Direction
	Min    uint8
	Max    uint8
	Line   uint8
	Runes  []rune
	Points int
	Fac    uint8
	Con    bool
	ConEx  bool
}

type GameState int

const (
	Idle GameState = iota
	Voting
	Playing
)

type GoGame struct {
	GameField    []PlayField        `json:"-"`
	Players      map[string]*Player `json:"players"`
	PlayedFields []PlayedField      `json:"pf"`
	VoteFields   []PlayedField      `json:"vf"`
	Letters      []Letter           `json:"-"`
	CurPlayer    int                `json:"cur"`
	FirstPlayer  int                `json:"-"`
	IsLastRound  bool               `json:"last"`
	CurState     GameState          `json:"-"`
	IsRunning    bool               `json:"running"`
	Counter      int                `json:"counter"`
	StepsLeft    int                `json:"-"`
	Mu           sync.Mutex         `json:"-"`
}
