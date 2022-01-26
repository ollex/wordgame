package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	csrf "github.com/utrack/gin-csrf"

	"zombiezen.com/go/sqlite/sqlitex"
)

var app *Application
var game *GoGame
var Points map[rune]int
var event *Event

func (stream *Event) listen() {
	for {
		select {
		// Add new available client
		case client := <-stream.NewClients:
			stream.TotalClients[client] = true
			log.Printf("Client added. %d registered clients", len(stream.TotalClients))

		// Remove closed client
		case client := <-stream.ClosedClients:
			delete(stream.TotalClients, client)
			log.Printf("Removed client. %d registered clients", len(stream.TotalClients))
		// check if client is closed before sending event msg works like this... if the chan is closed it goes into case... else into default
		// without default it would block, always include default statement
		case eventMsg := <-stream.Message:
			for clientMessageChan := range stream.TotalClients {
				select {
				case <-clientMessageChan:
				default:
					clientMessageChan <- eventMsg
				}
			}
		}
	}
}

func backupProtect() gin.HandlerFunc {
	return func(c *gin.Context) {
		if atomic.LoadInt32(&app.isBackupRunning) == 1 {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"Message": "Maintenance Mode - Please come back soon, this should not last longer than a minute or two!",
			})
			c.Abort()
		} else {
			c.Next()
		}
	}
}

func (app *Application) backup() bool {
	if atomic.CompareAndSwapInt32(&app.isBackupRunning, 0, 1) {
		fmt.Println("backup already running")
		return false
	}

	fmt.Println("app.wg.Wait()...")
	// wait until all queries have finished

	app.wg.Wait()
	fmt.Println("app.wg.Wait is done")
	ctx := context.Background()
	cctx, cancel := context.WithTimeout(ctx, time.Second*15)
	err := app.flushWal(cctx)
	cancel()
	if err != nil {
		fmt.Println(err.Error())
		atomic.StoreInt32(&app.isBackupRunning, 0)
		return false
	}
	err = app.db.Close()
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	// cp database - we don't hold locks supposedly here anymore, so POSIX advisory file lock bug should not affect us
	src, err := os.Open("./game.db")
	if err != nil {
		return false
	}
	defer src.Close()

	p, err := os.Getwd()
	if err != nil {
		return false
	}

	out, err := os.Create(p + "/data_" + time.Now().UTC().Format(time.RFC3339) + ".db")
	if err != nil {
		return false
	}
	defer func() {
		cerr := out.Close()
		err = cerr
	}()

	if _, err = io.Copy(out, src); err != nil {
		return false
	}
	err = out.Sync()
	// do errors ever really happen in Sync() or can we ignore it?
	return true
}

func seedIt() {
	rand.Seed(time.Now().UnixNano())
}

func (app *Application) swapLetters(r []rune) []rune {
	app.game.Mu.Lock()
	l := len(app.game.Letters)
	for _, rn := range r {
		for i := 0; i < l; i++ {
			if app.game.Letters[i].C == rn {
				app.game.Letters[i].Exist++
				break
			}
		}
	}
	app.game.Mu.Unlock()
	sr := make([]rune, len(r))
	for i := 0; i < len(r); i++ {
		if r[i] != '|' {
			lt := app.getRandChar()
			sr[i] = lt
		} else {
			sr[i] = '|'
		}
	}
	return sr
}

func (app *Application) getRandChar() rune {
	app.game.Mu.Lock()
	defer app.game.Mu.Unlock()
	var b []rune
	k := len(app.game.Letters)
	for i := 0; i < k; i++ {
		if app.game.Letters[i].Exist > 0 {
			b = append(b, app.game.Letters[i].C)
		}
	}
	if len(b) > 0 {
		idx := rand.Intn(len(b))
		r := b[idx]
		for i := 0; i < k; i++ {
			if app.game.Letters[i].C == r {
				app.game.Letters[i].Exist--
				break
			}
		}
		return r
	} else {
		return '|'
	}
}

func (app *Application) nextPlayer() int {
	app.game.Mu.Lock()
	defer app.game.Mu.Unlock()
	if app.game.CurPlayer < (len(app.game.Players) - 1) {
		app.game.CurPlayer++
	} else {
		app.game.CurPlayer = 0
	}
	app.game.Counter++
	return app.game.CurPlayer
}

// to do: re-use the existing game when a new game is constructed after first use
func newGame() *GoGame {
	seedIt()
	game := &GoGame{
		GameField:    []PlayField{},
		Players:      map[string]*Player{},
		PlayedFields: []PlayedField{},
		CurPlayer:    0,
		FirstPlayer:  0,
		IsRunning:    false,
		IsLastRound:  false,
		StepsLeft:    0,
		Counter:      0,
		Letters: []Letter{
			{C: 'Q', Points: 10, Exist: 1},
			{C: 'Y', Points: 10, Exist: 1},
			{C: 'Ö', Points: 8, Exist: 1},
			{C: 'X', Points: 8, Exist: 1},
			{C: 'Ä', Points: 6, Exist: 1},
			{C: 'J', Points: 6, Exist: 1},
			{C: 'Ü', Points: 6, Exist: 1},
			{C: 'V', Points: 6, Exist: 1},
			{C: 'P', Points: 4, Exist: 1},
			{C: 'C', Points: 4, Exist: 2},
			{C: 'F', Points: 4, Exist: 2},
			{C: 'K', Points: 4, Exist: 2},
			{C: 'B', Points: 3, Exist: 2},
			{C: 'M', Points: 3, Exist: 4},
			{C: 'W', Points: 3, Exist: 1},
			{C: 'Z', Points: 3, Exist: 1},
			{C: 'H', Points: 2, Exist: 4},
			{C: 'G', Points: 2, Exist: 3},
			{C: 'L', Points: 2, Exist: 3},
			{C: 'O', Points: 2, Exist: 3},
			{C: 'N', Points: 1, Exist: 9},
			{C: 'E', Points: 1, Exist: 15},
			{C: 'S', Points: 1, Exist: 7},
			{C: 'I', Points: 1, Exist: 6},
			{C: 'R', Points: 1, Exist: 6},
			{C: 'T', Points: 1, Exist: 6},
			{C: 'U', Points: 1, Exist: 6},
			{C: 'A', Points: 1, Exist: 5},
			{C: 'D', Points: 1, Exist: 4},
			{C: '*', Points: 0, Exist: 2},
		},
	}

	for i := 0; i < 225; i++ {
		game.GameField = append(game.GameField, PlayField{F: uint8(i), Fac: 1, Word: false})
	}
	game.GameField[0].Fac = 3
	game.GameField[0].Word = true
	game.GameField[7].Fac = 3
	game.GameField[7].Word = true
	game.GameField[14].Fac = 3
	game.GameField[14].Word = true
	game.GameField[105].Fac = 3
	game.GameField[105].Word = true
	game.GameField[112].Fac = 3
	game.GameField[112].Word = true
	game.GameField[119].Fac = 3
	game.GameField[119].Word = true
	game.GameField[210].Fac = 3
	game.GameField[210].Word = true
	game.GameField[217].Fac = 3
	game.GameField[217].Word = true
	game.GameField[224].Fac = 3
	game.GameField[224].Word = true
	game.GameField[16].Fac = 2
	game.GameField[16].Word = true
	game.GameField[28].Fac = 2
	game.GameField[28].Word = true
	game.GameField[32].Fac = 2
	game.GameField[32].Word = true
	game.GameField[42].Fac = 2
	game.GameField[42].Word = true
	game.GameField[48].Fac = 2
	game.GameField[48].Word = true
	game.GameField[56].Fac = 2
	game.GameField[56].Word = true
	game.GameField[154].Fac = 2
	game.GameField[154].Word = true
	game.GameField[160].Fac = 2
	game.GameField[160].Word = true
	game.GameField[168].Fac = 2
	game.GameField[168].Word = true
	game.GameField[176].Fac = 2
	game.GameField[176].Word = true
	game.GameField[182].Fac = 2
	game.GameField[182].Word = true
	game.GameField[192].Fac = 2
	game.GameField[192].Word = true
	game.GameField[196].Fac = 2
	game.GameField[196].Word = true
	game.GameField[208].Fac = 2
	game.GameField[208].Word = true
	game.GameField[3].Fac = 2
	game.GameField[11].Fac = 2
	game.GameField[36].Fac = 2
	game.GameField[38].Fac = 2
	game.GameField[45].Fac = 2
	game.GameField[52].Fac = 2
	game.GameField[59].Fac = 2
	game.GameField[92].Fac = 2
	game.GameField[96].Fac = 2
	game.GameField[98].Fac = 2
	game.GameField[102].Fac = 2
	game.GameField[108].Fac = 2
	game.GameField[116].Fac = 2
	game.GameField[122].Fac = 2
	game.GameField[126].Fac = 2
	game.GameField[128].Fac = 2
	game.GameField[132].Fac = 2
	game.GameField[165].Fac = 2
	game.GameField[172].Fac = 2
	game.GameField[179].Fac = 2
	game.GameField[186].Fac = 2
	game.GameField[188].Fac = 2
	game.GameField[213].Fac = 2
	game.GameField[221].Fac = 2
	game.GameField[20].Fac = 3
	game.GameField[24].Fac = 3
	game.GameField[76].Fac = 3
	game.GameField[80].Fac = 3
	game.GameField[84].Fac = 3
	game.GameField[88].Fac = 3
	game.GameField[136].Fac = 3
	game.GameField[140].Fac = 3
	game.GameField[144].Fac = 3
	game.GameField[148].Fac = 3
	game.GameField[200].Fac = 3
	game.GameField[204].Fac = 3
	game.Mu = sync.Mutex{}
	return game
}

func main() {
	Points = map[rune]int{}
	Points['Q'] = 10
	Points['Y'] = 10
	Points['Ö'] = 8
	Points['X'] = 8
	Points['Ä'] = 6
	Points['J'] = 6
	Points['Ü'] = 6
	Points['V'] = 6
	Points['P'] = 4
	Points['C'] = 4
	Points['F'] = 4
	Points['K'] = 4
	Points['B'] = 3
	Points['M'] = 3
	Points['W'] = 3
	Points['Z'] = 3
	Points['H'] = 2
	Points['G'] = 2
	Points['L'] = 2
	Points['O'] = 2
	Points['N'] = 1
	Points['E'] = 1
	Points['S'] = 1
	Points['I'] = 1
	Points['R'] = 1
	Points['T'] = 1
	Points['U'] = 1
	Points['A'] = 1
	Points['D'] = 1

	event = &Event{
		Message:       make(chan string),
		NewClients:    make(chan chan string),
		ClosedClients: make(chan chan string),
		TotalClients:  make(map[chan string]bool),
	}

	go event.listen()
	var s, s2 string
	var err error
	if len(os.Args) >= 2 {
		s, err = HashIt(os.Args[1])
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		s, err = HashIt("somebigsecret")
		if err != nil {
			log.Fatal(err.Error())
		}
	}
	if len(os.Args) >= 3 {
		s2, err = HashIt(os.Args[2])
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		s2, err = HashIt("someotherbigsecret")
		if err != nil {
			log.Fatal(err.Error())
		}
	}

	err = initTables(s, s2)
	if err != nil {
		log.Fatal(err.Error())
	}

	db, err := sqlitex.Open("./game.db", 0, 10)
	if err != nil {
		log.Print(err.Error())
	}
	defer db.Close()

	game = newGame()

	app = &Application{
		db:              db,
		isBackupRunning: 0,
		wg:              sync.WaitGroup{},
		game:            game,
		Words:           "",
	}
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.SetTrustedProxies([]string{"127.0.0.1"})

	r.LoadHTMLGlob("templates/*")

	r.Use(backupProtect())

	store := cookie.NewStore([]byte("someZsdf7_haasdfZZZasd$$"))
	r.Use(sessions.Sessions("session", store))

	r.Use(csrf.Middleware(csrf.Options{
		Secret: "secret123",
		ErrorFunc: func(c *gin.Context) {
			c.String(400, "CSRF token mismatch")
			c.Abort()
		},
	}))

	r.StaticFile("dragula.min.js", "public/dragula/dragula.min.js")
	r.StaticFile("dragula.min.css", "public/dragula/dragula.min.css")
	r.StaticFile("gowd.js", "public/gowd.js")

	setupRoutes(r, app)

	r.POST("/game/play", func(c *gin.Context) {
		_, _, un, err := RoleCheck(c, []string{"player"})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": err.Error(),
			})
			return
		}
		app.game.Mu.Lock()
		i := app.game.CurPlayer
		runs := app.game.IsRunning
		app.game.Mu.Unlock()
		if !runs {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Das Spiel ist beendet - es kann nicht mehr gezogen werden!",
			})
			return
		}
		str := "player" + strconv.Itoa(i+1)
		if un != str {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Du bist gerade nicht an der Reihe!",
			})
			return
		}
		rec := new([]UserPlayedField)
		if err := c.ShouldBind(rec); err != nil {
			log.Println(err.Error())
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Die gesendeten Spieldaten konnten nicht korrekt interpretiert werden.",
			})
			return
		}
		xr, _ := json.Marshal(rec)
		fmt.Println(string(xr))
		l := len(*rec)
		if l == 0 {
			// ok we have nothing to do... just go to next player... !
			idx := app.nextPlayer()
			c.JSON(200, gin.H{
				"msg": "ok wait for next player",
			})
			event.Message <- "{\"for\":\"player|all\",\"type\":\"next\",\"player\":\"player" + strconv.Itoa(idx+1) + "\"}"
			return
		}
		retWords := make([]string, 0)
		copyPf := make([]PlayedField, len(app.game.PlayedFields))
		// lock
		copy(copyPf, app.game.PlayedFields)
		//playedRunes := make([]rune, 0)
		recoverRunes := make([]rune, 0)
		checkLtrs := map[uint8]bool{}
		checkPrevLtrs := map[uint8]bool{}
		playedPos := make([]uint8, 0)
		for _, pf := range *rec {
			copyPf = append(copyPf, PlayedField{
				Pf:     pf.Position,
				Player: un,
				Char:   []rune(pf.Str)[0],
			})
			//playedRunes = append(playedRunes, []rune(pf.Str)[0])
			if pf.IsJoker {
				recoverRunes = append(recoverRunes, '*')
			} else {
				recoverRunes = append(recoverRunes, []rune(pf.Str)[0])
			}
			checkLtrs[pf.Position] = true
			playedPos = append(playedPos, pf.Position)
		}

		for _, cf := range app.game.PlayedFields {
			checkLtrs[cf.Pf] = true
			checkPrevLtrs[cf.Pf] = true
		}
		lc := len(copyPf)
		ar := make([]WordLine, 0)
		for _, pf := range *rec {
			f := false
			w := make([]rune, 0)
			limStart := uint8(math.Floor((float64(pf.Position/15) * 15))) // first in row
			line := uint8(math.Floor(float64(pf.Position / 15)))
			limEnd := limStart + 14
			minH := pf.Position
			maxH := pf.Position
			isConH := false
			isConHEx := false
			isConV := false
			isConVEx := false
			// go left
			for k := pf.Position - 1; k >= limStart; k-- {
				f = false
				for i := 0; i < lc; i++ {
					if copyPf[i].Pf == k {
						w = append(w, copyPf[i].Char)
						f = true
						minH = k
						if ok := checkLtrs[k]; ok {
							isConH = true
						}
						if ok := checkPrevLtrs[k]; ok {
							isConHEx = true
						}
						break
					}
				}
				if !f {
					break
				}
			}
			// reverse it
			for i2, j := 0, len(w)-1; i2 < j; i2, j = i2+1, j-1 {
				w[i2], w[j] = w[j], w[i2]
			}
			//w = append(w, '*')
			w = append(w, []rune(pf.Str)[0])

			if ok := checkLtrs[pf.Position]; ok {
				isConH = true
			}
			if ok := checkPrevLtrs[pf.Position]; ok {
				isConHEx = true
			}
			// accounting for fields, where a letter bonus is given, and for the word factor
			ltrFacPoints := 0
			if !pf.IsJoker && !app.game.GameField[pf.Position].Word && app.game.GameField[pf.Position].Fac > 1 {
				ltrFacPoints = (int(app.game.GameField[pf.Position].Fac - 1)) * Points[[]rune(pf.Str)[0]]
			}
			wrdFac := 1
			if app.game.GameField[pf.Position].Word {
				wrdFac = int(app.game.GameField[pf.Position].Fac)
			}
			for k := pf.Position + 1; k <= limEnd; k++ {
				f = false
				for i := 0; i < lc; i++ {
					if copyPf[i].Pf == k {
						w = append(w, copyPf[i].Char)
						f = true
						maxH = k
						if ok := checkLtrs[k]; ok {
							isConH = true
						}
						if ok := checkPrevLtrs[k]; ok {
							isConHEx = true
						}
						break
					}
				}
				if !f {
					break
				}
			}
			if len(w) > 1 {
				curL := len(ar)
				isFound := false
				for idx := 0; idx < curL; idx++ {
					if ar[idx].Dir == Direction(X) && ar[idx].Line == line && ar[idx].Min <= maxH && ar[idx].Max >= minH {
						isFound = true
						if uint8(wrdFac) > ar[idx].Fac {
							ar[idx].Fac = uint8(wrdFac)
							var p int = 0
							for ki := 0; ki < len(ar[idx].Runes); ki++ {
								if ar[idx].Runes[ki] != '*' {
									p += Points[ar[idx].Runes[ki]]
								}
							}
							p += ltrFacPoints
							p *= int(wrdFac)
							ar[idx].Points = p
						}
						break
					}
				}
				if !isFound {
					_, err := app.findWord(string(w), c)
					if err != nil {
						fmt.Println(err.Error())
						c.JSON(200, gin.H{
							"error": "Das Wort " + string(w) + " konnte nicht in der Datenbank gefunden werden. Nochmal (anders) legen, bitte!",
						})
						return
					}
					p := 0
					for ki := 0; ki < len(w); ki++ {
						// this check is now unnecessary
						if w[ki] != '*' {
							p += Points[w[ki]]
						}
					}
					p += ltrFacPoints
					p *= int(wrdFac)

					ar = append(ar, WordLine{
						Dir:    Direction(X),
						Min:    minH,
						Max:    maxH,
						Line:   line,
						Runes:  w,
						Points: p,
						Fac:    uint8(wrdFac),
						Con:    isConH,
						ConEx:  isConHEx,
					})
					retWords = append(retWords, string(w))
				}
			}

			wV := make([]rune, 0)
			limStartV := pf.Position - uint8(math.Floor(float64(pf.Position/15)*15))
			line = pf.Position % 15
			limEndV := 210 + limStartV
			minHV := pf.Position
			maxHV := pf.Position
			if pf.Position != limStartV {
				for k := pf.Position - 15; k >= limStartV; k -= 15 {
					f = false
					for i := 0; i < lc; i++ {
						if copyPf[i].Pf == k {
							wV = append(wV, copyPf[i].Char)
							f = true
							minHV = k
							if ok := checkLtrs[k]; ok {
								isConV = true
							}
							if ok := checkPrevLtrs[k]; ok {
								isConVEx = true
							}
							break
						}
					}
					if !f {
						break
					}
				}
			}
			for i2, j := 0, len(wV)-1; i2 < j; i2, j = i2+1, j-1 {
				wV[i2], wV[j] = wV[j], wV[i2]
			}
			wV = append(wV, []rune(pf.Str)[0])
			if ok := checkLtrs[pf.Position]; ok {
				isConV = true
			}
			if ok := checkPrevLtrs[pf.Position]; ok {
				isConVEx = true
			}
			if pf.Position != limEndV {
				for k := pf.Position + 15; k <= limEndV; k += 15 {
					f = false
					for i := 0; i < lc; i++ {
						if copyPf[i].Pf == k {
							wV = append(wV, copyPf[i].Char)
							f = true
							maxHV = k
							if ok := checkLtrs[k]; ok {
								isConV = true
							}
							if ok := checkPrevLtrs[k]; ok {
								isConVEx = true
							}
							break
						}
					}
					if !f {
						break
					}
				}
			}
			if len(wV) > 1 {
				curL := len(ar)
				isFound := false
				for idx := 0; idx < curL; idx++ {
					if ar[idx].Dir == Direction(Y) && ar[idx].Line == line && ar[idx].Min <= maxHV && ar[idx].Max >= minHV {
						isFound = true
						if uint8(wrdFac) > ar[idx].Fac {
							ar[idx].Fac = uint8(wrdFac)
							var p int = 0
							for ki := 0; ki < len(ar[idx].Runes); ki++ {
								// check is now unnecessary
								if ar[idx].Runes[ki] != '*' {
									p += Points[ar[idx].Runes[ki]]
								}
							}
							p += ltrFacPoints
							p *= int(wrdFac)
							ar[idx].Points = p
						}
						break
					}
				}
				if !isFound {
					_, err := app.findWord(string(wV), c)
					if err != nil {
						fmt.Println(err.Error())
						c.JSON(200, gin.H{
							"error": "Das Wort " + string(wV) + " konnte nicht in der Datenbank gefunden werden. Nochmal (anders) legen, bitte!",
						})
						return
					}
					p := 0
					for ki := 0; ki < len(wV); ki++ {
						// check is now unnecessary
						if wV[ki] != '*' {
							p += Points[wV[ki]]
						}
					}
					p += ltrFacPoints
					p *= int(wrdFac)

					ar = append(ar, WordLine{
						Dir:    Direction(Y),
						Min:    minHV,
						Max:    maxHV,
						Line:   line,
						Points: p,
						Fac:    uint8(wrdFac),
						Con:    isConV,
						ConEx:  isConVEx,
					})
					retWords = append(retWords, string(wV))
				}
			}
		}
		if l > 0 && len(retWords) == 0 {
			c.JSON(200, gin.H{
				"error": "Leider kein gültiges Wort gefunden, versuche es noch einmal!",
			})
			return
		}
		if len(app.game.PlayedFields) == 0 {
			hasCentral := false
			for z := 0; z < len(playedPos); z++ {
				if playedPos[z] == 112 {
					hasCentral = true
					break
				}
			}
			if !hasCentral {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "Das Mittelfeld muss am Anfang des Spiels belegt werden. Bitte nochmals versuchen!",
				})
				return
			}
		}
		// first loop over all words, if one of them is connected to "old" plus all are connected to "new" ok, else not ok
		isOneCon := false
		for i := 0; i < len(ar); i++ {
			if ar[i].ConEx {
				isOneCon = true
				break
			}
		}
		areAllCon := true
		if isOneCon {
			for i := 0; i < len(ar); i++ {
				if !ar[i].Con {
					areAllCon = false
					break
				}
			}
			if !areAllCon {
				c.JSON(200, gin.H{
					"error": "Es gibt unverbundene Steine!",
				})
				return
			}
		} else {
			if len(app.game.PlayedFields) > 0 {
				c.JSON(200, gin.H{
					"error": "Es gibt unverbundene Steine!",
				})
				return
			}
		}
		app.game.Mu.Lock()
		rem := len(app.game.Players[un].Runes)
		app.game.Mu.Unlock()
		retStr := ""
		replRunes := make([]rune, l)
		haveLtrs := true
		for i = 0; i < l; i++ {
			ltr := app.getRandChar()
			replRunes[i] = ltr
			if ltr != '|' {
				retStr += string(ltr)
			} else {
				haveLtrs = false
			}
		}
		k := 0
		// get rid of played runes
		app.game.Mu.Lock()
		for _, rn := range recoverRunes {
			for i := 0; i < 7; i++ {
				if app.game.Players[un].Runes[i] == rn {
					app.game.Players[un].Runes[i] = replRunes[k]
					k++
					break
				}
			}
		}
		app.game.Mu.Unlock()
		tp := 0
		// bonus for all letters on hand played
		if rem-l == 0 {
			tp += 50
		}
		war, _ := json.Marshal(retWords)
		for _, r := range ar {
			tp += r.Points
		}
		app.game.Mu.Lock()
		app.game.Players[un].Points += tp
		pts := app.game.Players[un].Points
		if len(retWords) > 0 {
			for _, pf := range *rec {
				app.game.PlayedFields = append(app.game.PlayedFields, PlayedField{
					Pf:     pf.Position,
					Player: un,
					Char:   []rune(pf.Str)[0],
				})
			}
		}
		app.game.Mu.Unlock()
		c.JSON(200, gin.H{
			"words":   string(war),
			"letters": retStr,
			"points":  pts,
		})
		cidx := app.game.CurPlayer + 1
		idx := app.nextPlayer()

		fields, err := json.Marshal(rec)
		if err != nil {
			fmt.Println("json.Marshal error in line: 634 -> " + err.Error())
		}
		sf := strings.TrimPrefix(string(fields), "\"")
		sf = strings.TrimSuffix(sf, "\"")

		if !haveLtrs && !app.game.IsLastRound && un == "player"+strconv.Itoa(cidx) {
			app.game.IsLastRound = true
			app.game.StepsLeft = len(app.game.Players) - 1
			event.Message <- "{\"for\":\"player|all\",\"type\":\"next\",\"cp\":" + strconv.Itoa(cidx) + ",\"points\":" + strconv.Itoa(pts) + ",\"words\":" + string(war) + ",\"fields\":" + sf + ",\"player\":\"player" + strconv.Itoa(idx+1) + "\"}"
			fmt.Println("entering last round")
		} else if app.game.IsLastRound {
			app.game.StepsLeft--
			if app.game.StepsLeft == 0 {
				app.game.IsRunning = false
				event.Message <- "{\"for\":\"player|all\",\"type\":\"end\",\"cp\":" + strconv.Itoa(cidx) + ",\"points\":" + strconv.Itoa(pts) + ",\"words\":" + string(war) + ",\"fields\":" + sf + ",\"player\":\"player" + strconv.Itoa(idx+1) + "\"}"
				fmt.Println("End condition reached")
			}
		} else {
			fmt.Println("next round due")
			event.Message <- "{\"for\":\"player|all\",\"type\":\"next\",\"cp\":" + strconv.Itoa(cidx) + ",\"points\":" + strconv.Itoa(pts) + ",\"words\":" + string(war) + ",\"fields\":" + sf + ",\"player\":\"player" + strconv.Itoa(idx+1) + "\"}"
		}
	})

	r.GET("/game/start", func(c *gin.Context) {
		_, _, un, err := RoleCheck(c, []string{"player"})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": err.Error(),
			})
			return
		}
		if un != "player1" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "du hast keine Befugnis, ein neues Spiel zu starten.",
			})
			return
		}
		app.game.Mu.Lock()
		if app.game.IsRunning {
			app.game.Mu.Unlock()
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "das Spiel ist schon gestartet.",
			})
			return
		}
		app.game.IsRunning = true
		l := len(app.game.Players)
		ended := app.game.IsLastRound
		app.game.Mu.Unlock()
		/*if l < 2 {
			c.JSON(200, gin.H{
				"error": "not enough players," + strconv.Itoa(l) + " online",
			})
			return
		}*/
		if ended {
			// we need a new game, here. Copy all current users into a new game
			pl := make(map[string]*Player, len(app.game.Players))
			for _, p := range app.game.Players {
				pl[p.Name] = &Player{
					Name:   p.Name,
					Runes:  nil,
					Points: p.Points,
				}
			}
			app.game = newGame()
			app.game.Players = pl
		}
		ok := true
		for i := 1; i <= l; i++ {
			if _, exists := app.game.Players["player"+strconv.Itoa(i)]; !exists {
				ok = false
				break
			}
		}
		if !ok {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Es muss eine lückenlos aufsteigende Anzahl an Spielern eingeloggt sein, um ein Spiel beginnen zu können (Player1, Player2...)",
			})
			return
		}
		for _, p := range app.game.Players {
			ra := ""
			p.Runes = make([]rune, 7)
			for i := 0; i < 7; i++ {
				// no check for | which would be "run out of letters" necessary here, we have enough letters at start of game
				r := app.getRandChar()
				if r != '|' {
					app.game.Mu.Lock()
					p.Runes[i] = r
					ra += string(r)
					app.game.Mu.Unlock()
				}
			}
			event.Message <- "{\"for\":\"player|" + p.Name + "\",\"type\":\"runes\",\"runes\":\"" + ra + "\"}"
		}
		time.Sleep(time.Millisecond * 200)
		idx := rand.Intn(l)
		app.game.Mu.Lock()
		app.game.CurPlayer = idx
		app.game.FirstPlayer = idx
		n := app.game.Players["player"+strconv.Itoa(idx+1)].Name
		app.game.Mu.Unlock()
		event.Message <- "{\"for\":\"player|all\",\"type\":\"start\",\"p\":\"" + n + "\"}"
		c.JSON(200, gin.H{
			"msg": "game started!",
		})
	})

	r.GET("/sse", func(c *gin.Context) {
		_, _, un, err := RoleCheck(c, []string{"player"})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Transfer-Encoding", "chunked")

		clientChan := make(ClientChan)
		// Send new connection to event server
		event.NewClients <- clientChan

		defer func() {
			// Send closed connection to event server
			event.ClosedClients <- clientChan
		}()

		go func() {
			// Send connection that is closed by client to event server
			<-c.Request.Context().Done()
			event.ClosedClients <- clientChan
			fmt.Println("Client closed sse") // this actually also happens on reload and so on. Maybe we should have a timeout check before removing the player from app.game
		}()
		app.game.Mu.Lock()

		flds := make([]UserPlayedField, len(app.game.PlayedFields))
		for i := range app.game.PlayedFields {
			flds[i] = UserPlayedField{
				Position: app.game.PlayedFields[i].Pf,
				Str:      string(app.game.PlayedFields[i].Char),
				IsJoker:  app.game.PlayedFields[i].IsJoker,
			}
		}

		fls, err := json.Marshal(flds)
		plyr := "player" + strconv.Itoa(app.game.CurPlayer+1)
		rs := string(app.game.Players[un].Runes)
		pts := app.game.Players[un].Points
		isRun := app.game.IsRunning
		app.game.Mu.Unlock()
		if err != nil {
			fmt.Println(err.Error())
		}
		fl := string(fls)
		if len(fl) < 1 {
			fl = ""
		}
		myLetters := ""
		myPoints := 0
		st := "false"
		if isRun {
			myLetters = rs
			myPoints = pts
			st = "true"
		}

		// send a message with the games' status for the case this customer was disconnected. A move counter will tell the client if
		// he is right or wrong
		go func() {
			time.Sleep(time.Millisecond * 1000)
			event.Message <- "{\"for\":\"player|" + un + "\",\"type\":\"stat\",\"started\":" + st + ",\"curp\":\"" + plyr + "\",\"points\":" + strconv.Itoa(myPoints) + ",\"letters\":\"" + myLetters + "\",\"fields\":" + fl + "}"
		}()
		// cannot do anything after this in a handler
		c.Stream(func(w io.Writer) bool {
			if msg, ok := <-clientChan; ok {
				if strings.Contains(msg, "player|"+un) || strings.Contains(msg, "player|all") {
					// analyze message, to customize event type, makes frontend easier!
					var d map[string]interface{}
					var t string
					err := json.Unmarshal([]byte(msg), &d)
					if err != nil {
						log.Println("json.Unmarshal of event msg failed: " + err.Error())
						log.Println("message is " + msg)
						c.SSEvent("message", msg)
					} else {
						if v, ok := d["type"]; ok {
							t = v.(string)
						} else {
							t = "message"
						}
						c.SSEvent(t, msg)
					}
				}
				return true
			}
			return false
		})
	})

	r.Run("0.0.0.0:8080")

}
