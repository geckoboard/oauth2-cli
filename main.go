package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"golang.org/x/oauth2"
)

const configDefaults = "/etc/oauth2-cli.json"

type config struct {
	Interface    string `json:"interface"`
	Port         int    `json:"port"`
	Callback     string `json:"callback"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	AuthURL      string `json:"auth_url"`
	TokenURL     string `json:"token_url"`
	CodeParam    string `json:"code_param"`
	Scope        string `json:"scopes"`
	OIDCNonce    bool   `json:"nonce"`
	Verbose      bool   `json:"verbose"`
}

func loadConfig() config {
	conf := config{
		Interface: "127.0.0.1",
		Port:      8081,
		Callback:  "/oauth/callback",
		CodeParam: "code",
	}

	defaultsFile, err := os.Open(configDefaults)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Fatalf("failed to open %q: %s\n", configDefaults, err)
		}
	} else {
		if err := json.NewDecoder(defaultsFile).Decode(&conf); err != nil {
			log.Fatalf("failed to parse %q: %s", configDefaults, err)
		}
	}

	flag.StringVar(&conf.Interface, "interface", conf.Interface, "Listening interface")
	flag.IntVar(&conf.Port, "port", conf.Port, "Listening port")
	flag.StringVar(&conf.Callback, "callback", conf.Callback, "Callback URL")
	flag.StringVar(&conf.ClientID, "id", conf.ClientID, "Client ID")
	flag.StringVar(&conf.ClientSecret, "secret", conf.ClientSecret, "Client Secret")
	flag.StringVar(&conf.AuthURL, "auth", conf.AuthURL, "Provider auth URL")
	flag.StringVar(&conf.TokenURL, "token", conf.AuthURL, "Provider token URL")
	flag.StringVar(&conf.CodeParam, "code", conf.CodeParam, "Query param to read code from")
	flag.StringVar(&conf.Scope, "scope", conf.Scope, "oAuth scope to authorize")
	flag.BoolVar(&conf.OIDCNonce, "oidc-nonce", conf.OIDCNonce, "include and then validate the OIDC nonce param")
	flag.BoolVar(&conf.Verbose, "verbose", conf.Verbose, "enable verbose logging")
	flag.Parse()

	required("auth", conf.AuthURL)
	required("token", conf.TokenURL)
	required("id", conf.ClientID)
	required("secret", conf.ClientSecret)

	return conf
}

func main() {
	conf := loadConfig()

	callbackURL, err := url.Parse(conf.Callback)
	if err != nil {
		log.Fatalln(err)
	}
	if callbackURL.Scheme == "" {
		callbackURL.Scheme = "http"
	}
	if callbackURL.Host == "" {
		callbackURL.Host = fmt.Sprintf("%s:%d", conf.Interface, conf.Port)
	}

	config := &oauth2.Config{
		ClientID:     conf.ClientID,
		ClientSecret: conf.ClientSecret,
		Scopes:       []string{conf.Scope},
		RedirectURL:  callbackURL.String(),
		Endpoint: oauth2.Endpoint{
			AuthURL:  conf.AuthURL,
			TokenURL: conf.TokenURL,
		},
	}

	var nonce string
	opts := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline}
	if conf.OIDCNonce {
		nonce = randString()
		opts = append(opts, oauth2.SetAuthURLParam("nonce", nonce))
	}

	state := randString()
	visitURL := config.AuthCodeURL(state, opts...)
	log.Printf("Visit this URL in your browser:\n%s\n\n", visitURL)

	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(1)

	http.HandleFunc(callbackURL.Path, func(w http.ResponseWriter, r *http.Request) {
		defer wg.Done()

		if conf.Verbose {
			log.Printf("Got callback: %s\n", r.URL.RequestURI())
			http.DefaultTransport = loggingTransport{Transport: http.DefaultTransport}
		}

		query := r.URL.Query()

		if s := query.Get("state"); s != state {
			http.Error(w, fmt.Sprintf("Invalid state: %s", s), http.StatusUnauthorized)
			return
		}

		code := query.Get(conf.CodeParam)
		token, err := config.Exchange(ctx, code)
		if err != nil {
			http.Error(w, fmt.Sprintf("Exchange error: %s", err), http.StatusServiceUnavailable)
			return
		}

		if nonce != "" {
			if err := checkNonce(nonce, token); err != nil {
				http.Error(w, fmt.Sprintf("OIDC nonce error: %s", err), http.StatusUnauthorized)
				return
			}
		}

		tokenJSON, err := json.MarshalIndent(token, "", "  ")
		if err != nil {
			http.Error(w, fmt.Sprintf("Token parse error: %s", err), http.StatusServiceUnavailable)
			return
		}

		log.Printf("result:\n%s\n", tokenJSON)

		_, _ = w.Write(tokenJSON)
	})

	server := http.Server{
		Addr: fmt.Sprintf("%s:%d", conf.Interface, conf.Port),
	}

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalln(err)
		}
	}()

	wg.Wait()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalln(err)
	}
}

func checkNonce(nonce string, token *oauth2.Token) error {
	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		return fmt.Errorf("missing OIDC id_token")
	}
	splitToken := strings.SplitN(idToken, ".", 3)
	log.Printf("%q", splitToken[1])
	payload, err := base64.RawURLEncoding.DecodeString(splitToken[1])
	if err != nil {
		return fmt.Errorf("id_token payload decode: %w", err)
	}
	var decodeToken struct {
		Nonce string
	}
	if err := json.Unmarshal(payload, &decodeToken); err != nil {
		return fmt.Errorf("id_token payload decode: %w", err)
	}
	if decodeToken.Nonce != nonce {
		return fmt.Errorf("%q != %q", decodeToken.Nonce, nonce)
	}
	return nil
}

func randString() string {
	buf := make([]byte, 32)
	rand.Read(buf)
	return base64.StdEncoding.EncodeToString(buf)
}

func required(flag string, value string) {
	if value == "" {
		log.Fatalf("-%s is a required flag\n", flag)
	}
}
