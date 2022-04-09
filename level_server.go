package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"pppordle/check"
	"pppordle/server/level"

	"github.com/google/uuid"
)

var (
	Requests     = make(map[Conn]*uuid.UUID)
	RequestMutex sync.Mutex
)

type LevelServer struct {
	Port       int
	Config     *tls.Config
	Level      *level.Level
	Entrypoint bool
}

func (ls *LevelServer) Host() {
	listener, err := tls.Listen("tcp", fmt.Sprintf(":%d", ls.Port), ls.Config)
	check.Fatal(fmt.Sprintf("level %d listener failed", ls.Level.Number), err)

	for {
		conn, err := listener.Accept()
		if err != nil {
			check.Print("error accepting connection", err)
			continue
		}

		go ls.HandleAuthenticatedRequest(conn)
	}
}

func (ls *LevelServer) HandleAuthenticatedRequest(conn net.Conn) {
	sessionConn := Conn{
		LocalAddr:  conn.LocalAddr(),
		RemoteAddr: conn.RemoteAddr(),
	}

	defer func() {
		RequestMutex.Lock()
		delete(Requests, sessionConn)
		RequestMutex.Unlock()
		conn.Close()
	}()

	connErr := make(chan error, 1)
	sessionErr := make(chan error, 1)

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		ls.sessionSearch(sessionConn, connErr, sessionErr)
		wg.Done()
	}()
	feedbackWriter(conn, connErr, sessionErr)

	wg.Wait()
	return
}

func feedbackWriter(conn net.Conn, connErr chan error, sessionErr chan error) {
	_, err := conn.Write([]byte("Searching for session..."))
	if err != nil {
		connErr <- err
		return
	}

	progressTimer := time.NewTicker(200 * time.Millisecond)

sessionSearchLoop:
	for {
		select {
		case err = <-sessionErr:
			break sessionSearchLoop
		case <-progressTimer.C:
			_, err := conn.Write([]byte("."))
			if err != nil {
				connErr <- err
				return
			}
		}
	}

	connErr <- nil

	if err != nil {
		conn.Write([]byte("\nError finding session:\n" + err.Error()))
	} else {
		conn.Write([]byte("\nSession found\n"))
	}
}

func (ls *LevelServer) sessionSearch(sessionConn Conn, connErr chan error, sessionErr chan error) {
	var err error
	bruteForcePrevention(1000)

	RequestMutex.Lock()
	sessionID, ok := Requests[sessionConn]
	RequestMutex.Unlock()
	if !ok {
		sessionErr <- errors.New("No session provided")
		return
	}

	session, ok := Sessions[*sessionID]
	if !ok {
		sessionErr <- errors.New("Could not find session")
		return
	}

	sessionErr <- nil
	select {
	case err = <-connErr:
	default:
	}
	if err != nil {
		return
	}

	session.GameChan <- ls.Level.GenerateGame()
}

func bruteForcePrevention(ms time.Duration) {
	time.Sleep(ms * time.Millisecond)
}
