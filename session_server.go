package main

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net"
	"pppordle/cert"
	"pppordle/check"
	"pppordle/game"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Session struct {
	Conn     Conn
	GameChan chan *game.Game
}

type Conn struct {
	LocalAddr  net.Addr
	RemoteAddr net.Addr
}

var (
	Sessions     = make(map[uuid.UUID]Session)
	SessionMutex sync.Mutex
)

var (
	timeout     = 1 * time.Minute
	authTimeout = 3 * time.Second
)

func handleSessions(port int, tlsConfig *tls.Config, levelCount int) {
	listener, err := tls.Listen("tcp", fmt.Sprintf(":%d", port), tlsConfig)
	check.Fatal("session listener failed", err)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting connection:", err)
			continue
		}

		sessionId := uuid.New()
		session := Session{
			Conn: Conn{
				LocalAddr:  conn.LocalAddr(),
				RemoteAddr: conn.RemoteAddr(),
			},
			GameChan: make(chan *game.Game),
		}
		SessionMutex.Lock()
		Sessions[sessionId] = session
		SessionMutex.Unlock()
		log.Printf("new session from %v: %v", conn.RemoteAddr(), sessionId)

		go func() {
			handleSession(conn, session, sessionId, levelCount)
			SessionMutex.Lock()
			delete(Sessions, sessionId)
			SessionMutex.Unlock()
		}()
	}
}

func handleSession(conn net.Conn, session Session, id uuid.UUID, levelCount int) {
	var req game.Request
	var g *game.Game

	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	err := conn.SetDeadline(time.Now().Add(timeout))
	if err != nil {
		log.Println("Failed to set deadline:", err)
		return
	}

	initMessage := &game.InitResult{
		SessionID:  id,
		LevelCount: levelCount,
	}
	err = encoder.Encode(initMessage)
	if err != nil {
		log.Println("Error sending result of game information request:", err)
		return
	}

	authTimer := time.NewTimer(authTimeout)
	select {
	case g = <-session.GameChan:
		log.Printf("session %v: level %d authentication successful", id, g.Level)
		log.Println(string(g.Word))
	case <-authTimer.C:
		log.Printf("session %v: authentication timeout", id)
		return
	}

	guesses := g.Guesses

	infoMessage := &game.InfoResult{
		Length:     len(g.Word),
		Level:      g.Level,
		Guesses:    guesses,
		Candidates: g.Candidates,
	}
	err = encoder.Encode(infoMessage)
	if err != nil {
		log.Println("Error sending result of game information request:", err)
		return
	}

	for {
		err := decoder.Decode(&req)
		if err != nil {
			return
		}

		if req.Type != game.RequestGuess {
			return
		}

		result := g.ProcessGuess([]rune(req.Data))
		if len(result.Indicators) > 0 {
			guesses -= 1
		}

		if result.Complete && g.Level < levelCount {
			completionCert, err := generateCompletionCert(g.Level + 1)
			if err != nil {
				log.Println("Failed to generate client certificate:", err)
				return
			}
			result.ClientCert = *completionCert
			result.CompleteMessage = g.CompleteMessage
		}

		result.RemainingGuesses = guesses

		err = encoder.Encode(result)
		if err != nil {
			return
		}

		if result.Complete {
			log.Printf("session %v: level %d complete", id, g.Level)
			return
		}

		if guesses == 0 {
			log.Printf("session %v: no more guesses", id)
			return
		}
	}
}

func generateCompletionCert(level int) (*cert.PemCertPair, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	pemClient, err := cert.MakeCerts(cert.CertConfig{
		Parent:     pemCA,
		IsServer:   false,
		IsClient:   true,
		Serial:     serialNumber,
		CommonName: fmt.Sprint(level),
		DNSNames:   []string{fmt.Sprint(level)},
		SecsValid:  60 * 60 * 24,
	})
	if err != nil {
		return nil, err
	}

	return pemClient, nil
}
