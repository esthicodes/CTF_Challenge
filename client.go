package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"time"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/rivo/tview"

	"pppordle/game"
)

const (
	sessionPort = 1337
)

var (
	domain = "pppordle.chal.pwni.ng"
	dev = os.Getenv("PPPORDLE_ENV")
)

func main() {
	log.SetOutput(io.Discard)

	if dev == "dev" {
		domain = "localhost"
	}

	startUI()
}

func startSession(level int, loadingText *tview.TextView) (net.Conn, error) {
	sessionServer := net.JoinHostPort(domain, fmt.Sprint(sessionPort))
	levelServer := net.JoinHostPort(domain, fmt.Sprint(sessionPort+level))

	certName := "certs/ca.pem"
	if dev == "dev" {
		certName = "certs/dev_ca.pem"
	}

	ca, err := os.ReadFile(certName)
	if err != nil {
		return nil, fmt.Errorf("failed to open CA cert file: %w", err)
	}
	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(ca)
	if !ok {
		return nil, errors.New("Failed to add ca cert to pool")
	}

	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}

	conn, err := tls.Dial("tcp", sessionServer, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to session server: %w", err)
	}

	initResult, err := makeRequest[*game.InitResult](conn, game.Request{Type: game.RequestInit})
	if err != nil {
		return nil, fmt.Errorf("failed to get session init result: %w", err)
	}
	log.Printf("received init result: %+v", initResult)

	sessionID := initResult.SessionID
	tlsConfig.ServerName = fmt.Sprintf("%s.session", sessionID.String())

	if level > 1 {
		clientPem, err := os.ReadFile(fmt.Sprintf("certs/level%d.pem", level))
		if err != nil {
			return nil, fmt.Errorf("unable to read client cert for this level: %w", err)
		}
		clientKey, err := os.ReadFile(fmt.Sprintf("certs/level%d.key", level))
		if err != nil {
			return nil, fmt.Errorf("unable to read client key for this level: %w", err)
		}

		clientCert, err := tls.X509KeyPair(clientPem, clientKey)
		if err != nil {
			return nil, fmt.Errorf("unable to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{clientCert}
	}

	authConn, err := tls.Dial("tcp", levelServer, tlsConfig)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to connnect to level %d server: %w", level, err)
	}

	defer authConn.Close()

	_, err = io.Copy(loadingText, authConn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read authentication response: %w", err)
	}

	time.Sleep(time.Second)

	return conn, nil
}

func makeRequest[R game.Result](conn net.Conn, req game.Request) (res R, err error) {
	e := json.NewEncoder(conn)
	d := json.NewDecoder(conn)

	if req.Type == game.RequestGuess {
		err = e.Encode(req)
		if err != nil {
			return nil, err
		}
	}

	err = d.Decode(&res)
	if err != nil {
		return nil, err
	}

	return res, nil
}
