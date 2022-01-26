package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func (app *Application) flushWal(ctx context.Context) error {
	conn := app.db.Get(ctx)
	if conn == nil {
		return errors.New("could not get db connection")
	}
	defer app.db.Put(conn)
	if err := sqlitex.ExecTransient(conn, "PRAGMA wal_checkpoint(TRUNCATE);", nil); err != nil {
		return err
	}
	return nil
}

func initTables(s, s2 string) error {

	conn, err := sqlite.OpenConn("./game.db", 0)

	if err != nil {
		return errors.New("could not get db connection")
	}
	defer conn.Close()

	if err := sqlitex.ExecTransient(conn, "CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, username TEXT NOT NULL, name TEXT NOT NULL, pw TEXT NOT NULL, role TEXT NOT NULL, points INTEGER NOT NULL)", nil); err != nil {
		return err
	}
	isInit := false
	stmt := conn.Prep("SELECT * FROM users ORDER BY id ASC LIMIT 1")

	hasRow, err := stmt.Step()
	if err != nil {
		return err
	} else if !hasRow {
		isInit = true
	} else if hasRow {
		fmt.Println("Tables are already initialized.")
	}
	stmt.Reset()
	if isInit {
		fmt.Println("Creating database tables...")
		if err = sqlitex.ExecTransient(conn, "INSERT INTO users(username, name, pw, role, points) VALUES(?, ?, ?, ?, ?) ", nil, "olafsabatschus", "Olaf Sabatschus", s, "admin", 0); err != nil {
			return err
		}
		if err = sqlitex.ExecTransient(conn, "INSERT INTO users(username, name, pw, role, points) VALUES(?, ?, ?, ?, ?) ", nil, "player1", "Player 1", s2, "player", 0); err != nil {
			return err
		}
		if err = sqlitex.ExecTransient(conn, "INSERT INTO users(username, name, pw, role, points) VALUES(?, ?, ?, ?, ?) ", nil, "player2", "Player 2", s2, "player", 0); err != nil {
			return err
		}
		if err = sqlitex.ExecTransient(conn, "INSERT INTO users(username, name, pw, role, points) VALUES(?, ?, ?, ?, ?) ", nil, "player3", "Player 2", s2, "player", 0); err != nil {
			return err
		}

		if err = sqlitex.ExecTransient(conn, "CREATE TABLE IF NOT EXISTS games (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL, num_players INTEGER NOT NULL, txt TEXT, datum TEXT)", nil); err != nil {
			return err
		}
	}
	return nil
}

func (app *Application) getUserForUsername(username string, ctx context.Context) (*User, error) {
	conn := app.db.Get(ctx)
	if conn == nil {
		return nil, errors.New("could not get db connection")
	}
	defer app.db.Put(conn)

	app.wg.Add(1)
	defer app.wg.Done()

	var u User
	fmt.Println("got " + username)
	stmt := conn.Prep("SELECT id, name, username, role, pw, points FROM users WHERE username = $nm;")
	stmt.SetText("$nm", username)
	defer stmt.Reset()
	hasRow, err := stmt.Step()
	if err != nil {
		return nil, err
	} else if hasRow {

		u.Id = stmt.ColumnInt64(0)
		u.Name = stmt.ColumnText(1)
		u.Username = stmt.ColumnText(2)
		u.Role = stmt.ColumnText(3)
		u.Pw = stmt.ColumnText(4)
		u.Points = stmt.ColumnInt64(5)
		return &u, nil
	} else {
		return nil, errors.New("not found")
	}
}

func (app *Application) loadAllWords(ctx context.Context) ([]string, error) {
	conn := app.db.Get(ctx)
	if conn == nil {
		return nil, errors.New("could not get db connection")
	}
	defer app.db.Put(conn)
	x := make([]string, 0)
	stmt := conn.Prep("SELECT word FROM words LIMIT 2200000")
	for {
		hasRow, err := stmt.Step()
		if err != nil {
			return nil, err
		} else if !hasRow {
			return x, nil
		}
		str := stmt.ColumnText(0)
		x = append(x, str)
	}
}

func (app *Application) addWords(a *AddWord, ctx context.Context) error {
	conn := app.db.Get(ctx)
	if conn == nil {
		return errors.New("could not get db connection")
	}
	defer app.db.Put(conn)
	app.wg.Add(1)
	defer app.wg.Done()
	for _, w := range a.Words {
		v := strings.TrimSpace(w)
		if err := sqlitex.ExecTransient(conn, "INSERT INTO words (word) VALUES(?)", nil, v); err != nil {
			return err
		}
	}
	return nil
}

func (app *Application) removeWords(a *AddWord, ctx context.Context) error {
	conn := app.db.Get(ctx)
	if conn == nil {
		return errors.New("could not get db connection")
	}
	defer app.db.Put(conn)
	app.wg.Add(1)
	defer app.wg.Done()
	for _, w := range a.Words {
		if err := sqlitex.ExecTransient(conn, "DELETE FROM words where word = ?", nil, w); err != nil {
			return err
		}
	}
	return nil
}

func (app *Application) findWord(search string, ctx context.Context) (*Word, error) {
	conn := app.db.Get(ctx)
	if conn == nil {
		return nil, errors.New("could not get db connection")
	}
	defer app.db.Put(conn)
	app.wg.Add(1)
	defer app.wg.Done()
	fmt.Println("Suche " + search)
	search = strings.ToLower(search)
	stmt := conn.Prep("SELECT id, word FROM words WHERE word = $w")
	stmt.SetText("$w", search)
	defer stmt.Reset()
	var x *Word
	if hasRow, err := stmt.Step(); err != nil {
		return nil, err
	} else if !hasRow {
		return nil, errors.New("...Wort nicht in der Datenbank gefunden")
	}
	//fmt.Println("Wort " + search + " gefunden")
	x = &Word{
		Id:   stmt.ColumnInt64(0),
		Word: stmt.ColumnText(1),
	}
	return x, nil
}

func (app *Application) getGameForId(id int64, ctx context.Context) ([]*Game, error) {
	conn := app.db.Get(ctx)
	if conn == nil {
		return nil, errors.New("could not get db connection")
	}
	defer app.db.Put(conn)

	app.wg.Add(1)
	defer app.wg.Done()

	var x []*Game
	stmt := conn.Prep("SELECT id, txt, datum FROM games WHERE id = $id")
	stmt.SetInt64("$id", id)
	defer stmt.Reset()
	for {
		if hasRow, err := stmt.Step(); err != nil {
			return nil, err
		} else if !hasRow {
			break
		}
		x = append(x, &Game{
			Id:    stmt.ColumnInt64(0),
			Text:  stmt.ColumnText(1),
			Datum: stmt.ColumnText(2),
		})
	}
	return x, nil
}
