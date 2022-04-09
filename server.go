package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"

	"pppordle/cert"
	"pppordle/check"
	"pppordle/server/level"
)

const (
	sessionPort = 1337
)

var (
	domain = "pppordle.chal.pwni.ng"
	dev    = os.Getenv("PPPORDLE_ENV")
	pemCA  *cert.PemCertPair
)

func main() {
	f, err := os.OpenFile("pppordle.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	check.Fatal("could not open log file", err)
	defer f.Close()
	log.SetOutput(f)

	certName := "ca"
	if dev == "dev" {
		log.Println("starting server in development mode")
		domain = "localhost"
		certName = "dev_ca"
	}
	caCert, err := os.ReadFile(fmt.Sprintf("certs/%s.pem", certName))
	check.Fatal("unable to read ca cert", err)
	caKey, err := os.ReadFile(fmt.Sprintf("certs/%s.key", certName))
	check.Fatal("unable to read ca key", err)
	pemCA = &cert.PemCertPair{
		Cert: caCert,
		Key:  caKey,
	}

	pemServer, err := cert.MakeCerts(cert.CertConfig{
		Parent:     pemCA,
		IsServer:   true,
		IsClient:   false,
		Serial:     big.NewInt(1),
		CommonName: domain,
		DNSNames:   []string{domain, "*.session"},
		SecsValid:  60 * 60 * 24 * 365,
	})
	check.Fatal("unable to generate server certificate pair", err)

	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(pemCA.Cert)
	if !ok {
		log.Fatalf("failed to add ca cert to pool")
	}

	var levels []LevelServer

	levels = append(levels, LevelServer{
		Level:      level.Level1(),
		Entrypoint: true,
	})

	levels = append(levels, LevelServer{
		Level: level.Level2(),
	})

	levels = append(levels, LevelServer{
		Level: level.Level3(),
	})

	levels = append(levels, LevelServer{
		Level: level.Level4(),
	})

	serverCert, err := tls.X509KeyPair(pemServer.Cert, pemServer.Key)
	check.Fatal("unable to load server certificate pair", err)

	var wg sync.WaitGroup
	wg.Add(len(levels))
	for i, l := range levels {
		l.Port = sessionPort + i + 1
		l.Config = &tls.Config{
			Certificates:       []tls.Certificate{serverCert},
			MinVersion:         tls.VersionTLS13,
			GetConfigForClient: getSessionFromHello,
		}

		if !l.Entrypoint {
			l.Config.ClientAuth = tls.RequireAndVerifyClientCert
			l.Config.ClientCAs = caCertPool
			l.Config.VerifyPeerCertificate = getLevelValidator(caCertPool, l.Level.Number)
		}

		fmt.Printf("Starting level %d listener\n", l.Level.Number)
		go func(levelServer LevelServer) {
			levelServer.Host()
			wg.Done()
		}(l)
	}

	fmt.Println("Starting session listener")
	handleSessions(sessionPort, &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		MinVersion:   tls.VersionTLS13,
	}, len(levels))

	wg.Wait()
}

func getLevelValidator(caCertPool *x509.CertPool, levelNumber int) func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if len(verifiedChains) == 0 || len(verifiedChains[0]) == 0 {
			return errors.New("No verified chains")
		}

		opts := x509.VerifyOptions{
			DNSName:   fmt.Sprint(levelNumber),
			Roots:     caCertPool,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}
		_, err := verifiedChains[0][0].Verify(opts)
		return err
	}
}

func getSessionFromHello(hello *tls.ClientHelloInfo) (*tls.Config, error) {
	sessionID, err := parseSessionID(hello.ServerName)
	if err != nil {
		return nil, nil
	}

	sessionConn := Conn{
		LocalAddr:  hello.Conn.LocalAddr(),
		RemoteAddr: hello.Conn.RemoteAddr(),
	}

	RequestMutex.Lock()
	Requests[sessionConn] = sessionID
	RequestMutex.Unlock()

	return nil, nil
}

func parseSessionID(domain string) (*uuid.UUID, error) {
	splitString := strings.Split(domain, ".")

	if len(splitString) != 2 {
		return nil, fmt.Errorf("session parsing failed: %w", errors.New("improperly formatted server name"))
	}

	sessionID, err := uuid.Parse(splitString[0])
	if err != nil {
		return nil, fmt.Errorf("session parsing failed: %w", err)
	}

	return &sessionID, nil
}
