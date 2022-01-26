package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	csrf "github.com/utrack/gin-csrf"
	"zombiezen.com/go/sqlite/sqlitex"
)

func RoleCheck(c *gin.Context, roles []string) (int64, string, string, error) {
	session := sessions.Default(c)
	r := session.Get("role").(string)
	ur := session.Get("username").(string)
	f := false
	for _, rs := range roles {
		if rs == r {
			f = true
			break
		}
	}
	if f {
		id := session.Get("id").(int64)
		return id, r, ur, nil
	}
	return 0, r, ur, errors.New("access denied")
}

func setupRoutes(r *gin.Engine, app *Application) {

	rate, err := limiter.NewRateFromFormatted("10-M")
	if err != nil {
		log.Fatal(err)
		return
	}

	store := memory.NewStore()

	middleware := mgin.NewMiddleware(limiter.New(store, rate))

	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/login")
	})

	r.GET("/login", func(c *gin.Context) {
		c.HTML(200, "index.tmpl", gin.H{
			"msg":  "Please enter your credentials:",
			"tpe":  "primary",
			"csrf": csrf.GetToken(c),
		})
	})

	r.GET("/logout", func(c *gin.Context) {
		session := sessions.Default(c)
		n := session.Get("username").(string)
		session.Clear()
		session.Set("dummy", true)
		session.Save()
		c.Redirect(302, "/")
		app.game.Mu.Lock()
		delete(app.game.Players, n)
		l := len(app.game.Players)
		app.game.Mu.Unlock()
		event.Message <- "{\"for\":\"player|all\",\"left\":\"" + n + "\"}"
		if l == 0 {
			app.game = newGame()
		}
	})

	r.GET("/words/all", func(c *gin.Context) {
		_, _, _, err := RoleCheck(c, []string{"player", "admin"})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		t := time.Now()
		str, err := app.loadAllWords(c)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			//j, _ := json.Marshal(str)

			s := "\n" + strconv.Itoa(len(str))
			x := time.Since(t).String()
			s += "\n" + x
			c.JSON(200, gin.H{
				"words": s,
			})
		}
	})

	r.POST("/chat/msg", func(c *gin.Context) {
		_, _, un, err := RoleCheck(c, []string{"player", "admin"})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		m := new(ChatMsg)
		if c.ShouldBind(m) == nil {
			if len(m.Msg) > 0 {
				event.Message <- "{\"for\":\"player|all\",\"type\":\"chat\",\"from\":\"" + un + "\",\"txt\":\"" + m.Msg + "\"}"
				c.JSON(200, gin.H{
					"msg": "ok",
				})
			}
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Anfrage konnte nicht richtig interpretiert werden",
			})
		}
	})

	r.POST("/word/add", func(c *gin.Context) {
		_, _, _, err := RoleCheck(c, []string{"player", "admin"})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		w := new(AddWord)
		if c.ShouldBind(w) == nil {
			err := app.addWords(w, c)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(200, gin.H{
				"msg": "ok",
			})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Wort konnte nicht in Anfrage gefunden werden",
			})
		}
	})

	r.POST("/letter/swap", func(c *gin.Context) {
		_, _, un, err := RoleCheck(c, []string{"player", "admin"})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		app.game.Mu.Lock()
		cp := app.game.CurPlayer
		runs := app.game.IsRunning
		app.game.Mu.Unlock()
		cp++
		if !runs {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Das Spiel ist schon beendet - es können keine Buchstaben mehr getauscht werden!",
			})
			return
		}
		i := "player" + strconv.Itoa(cp)
		if i != un {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Du bist nicht an der Reihe..!",
			})
			return
		}
		app.game.Mu.Lock()
		s := make([]rune, 7)
		copy(s, app.game.Players[un].Runes)
		app.game.Mu.Unlock()
		bs := app.swapLetters(s)
		app.game.Mu.Lock()
		app.game.Players[un].Runes = bs
		app.game.Mu.Unlock()
		c.JSON(200, gin.H{
			"letters": string(bs),
		})
		idx := app.nextPlayer()
		event.Message <- "{\"for\":\"player|all\",\"type\":\"next\",\"player\":\"player" + strconv.Itoa(idx+1) + "\"}"
	})

	r.POST("/word/remove", func(c *gin.Context) {
		_, _, _, err := RoleCheck(c, []string{"player", "admin"})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		w := new(AddWord)
		if c.ShouldBind(w) == nil {
			err := app.removeWords(w, c)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(200, gin.H{
				"msg": "ok",
			})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Wort konnte nicht in Anfrage gefunden werden",
			})
		}
	})

	r.POST("/login", middleware, func(c *gin.Context) {
		session := sessions.Default(c)
		session.Clear()
		session.Save()
		app.game.Mu.Lock()
		run := app.game.IsRunning
		app.game.Mu.Unlock()
		if run {
			c.HTML(200, "index.tmpl", gin.H{
				"msg":  "Game in progress - come back later!",
				"tpe":  "error",
				"csrf": csrf.GetToken(c),
			})
			return
		}
		p := new(Person)
		if c.ShouldBind(p) == nil {
			u, err := app.getUserForUsername(p.Name, c)

			if err != nil {
				log.Println(err.Error())
				c.HTML(http.StatusBadRequest, "index.tmpl", gin.H{
					"msg":  "Auth Failed",
					"tpe":  "error",
					"csrf": csrf.GetToken(c),
				})
				return
			}

			if ok := CheckIt(p.Pw, u.Pw); !ok {
				c.HTML(http.StatusBadRequest, "index.tmpl", gin.H{
					"msg":  "Auth Failed",
					"tpe":  "error",
					"csrf": csrf.GetToken(c),
				})
				return
			}

			if u.Role != "admin" && u.Role != "player" {
				c.HTML(403, "index.tmpl", gin.H{
					"msg":  "Authentication Failed.",
					"tpe":  "error",
					"csrf": csrf.GetToken(c),
				})
				return
			}
			// login only if this user is not yet logged in!
			app.game.Mu.Lock()
			if _, ok := app.game.Players[u.Username]; ok {
				app.game.Mu.Unlock()
				c.HTML(403, "index.tmpl", gin.H{
					"msg":  u.Username + " is already logged in.",
					"tpe":  "error",
					"csrf": csrf.GetToken(c),
				})
				return
			}
			app.game.Players[u.Username] = &Player{
				Name:   u.Username,
				Points: int(u.Points),
				Runes:  make([]rune, 7),
			}
			app.game.Mu.Unlock()

			session.Options(sessions.Options{
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
				MaxAge:   0,
			})
			session.Set("id", u.Id)
			session.Set("name", u.Name)
			session.Set("role", u.Role)
			session.Set("username", u.Username)
			session.Save()

			if u.Role == "player" {
				c.Redirect(302, "/dashboard")
			} else if u.Role == "admin" {
				c.Redirect(302, "/admin")
			}

		} else {
			c.HTML(http.StatusBadRequest, "index.tmpl", gin.H{
				"msg":  "Auth Failed",
				"tpe":  "error",
				"csrf": csrf.GetToken(c),
			})
		}
	})

	r.GET("/cmd/backup", func(c *gin.Context) {
		_, _, _, err := RoleCheck(c, []string{"coach"})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		ok := app.backup()
		fmt.Printf("backup = %v", ok)
		if atomic.LoadInt32(&app.isBackupRunning) == 1 {
			// this means we have to open db in app again
			app.db, err = sqlitex.Open("./workout.db", 0, 10)
			if err != nil {
				os.Exit(1)
			}
			atomic.StoreInt32(&app.isBackupRunning, 0)
		}
		if !ok {
			c.JSON(200, gin.H{
				"error": "Backup failed",
			})
			return
		}

		c.JSON(200, gin.H{
			"msg": "Backup successful",
		})
	})

	r.GET("/dashboard", func(c *gin.Context) {
		_, _, un, err := RoleCheck(c, []string{"player"})
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.HTML(200, "dashboard.tmpl", gin.H{
			"csrf":   csrf.GetToken(c),
			"player": un,
		})
	})

	r.GET("/admin", func(c *gin.Context) {
		_, _, un, err := RoleCheck(c, []string{"admin"})
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.HTML(200, "admin.tmpl", gin.H{
			"csrf": csrf.GetToken(c),
			"name": un,
		})
	})

}
