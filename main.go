package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"text/template"

	"github.com/9ssi7/turnstile"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// TODO store requests that could not be sent

// handle request
// redirect to receiver, based on sport entered
// config: redirect url, mail (server, port, user, password, subject prefix), receiver email, turnstile private token
// reply-to address (email address listed)
// define, which sport to which email

var (
	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Gray    = "\033[37m"
	White   = "\033[97m"
) // ANSI colors

type Req struct {
	Name    string
	Email   string
	Dept    string
	Comment string
}

func (req *Req) display() string {
	d, err := json.Marshal(req)
	if err != nil {
		return ""
	}
	return fmt.Sprint(string(d))
}

func fromHttpReq(req *http.Request, config *Config) (*Req, error) {
	name := req.FormValue("name")
	if name == "" {
		return nil, errors.New("no name provided")
	}
	email := req.FormValue("email")
	if email == "" {
		return nil, errors.New("no email provided")
	}

	sport := req.FormValue("sport")
	dpt, prs := config.Departments[sport]
	if !prs {
		return nil, errors.New("no valid department provided")
	}
	dptName := dpt.Display
	comment := req.FormValue("comment")
	return &Req{
		Name:    name,
		Email:   email,
		Dept:    dptName,
		Comment: comment,
	}, nil
}

var (
	errInvalidToken = errors.New("invalid turnstile token")
	logger          = log.Default()

	config       *Config
	turnstileSrv turnstile.Service
	mailTmpl     *template.Template
)

func main() {
	go handleSignals()

	// get config
	logger.Println("reading config location from CONFIG_FILE environment variable...")
	configPath := os.Getenv("CONFIG_FILE")
	config, err := getConfig(configPath)
	if err != nil {
		errExit("there's no config file at location '%s'\n", configPath)
	}

	// init turnstile
	logger.Println("initializing Cloudflare Turnstile service...")
	turnstileSrv = turnstile.New(turnstile.Config{
		Secret: config.Turnstile.Secret,
	})

	// init template
	logger.Println("reading config location from MAIL_TEMPLATE_FILE environment variable...")
	tmplPath := os.Getenv("MAIL_TEMPLATE_FILE")
	mailTmpl, err = template.New(path.Base(tmplPath)).ParseFiles(tmplPath)
	if err != nil {
		errExit("failed to parse mail template: %s", err)
	}

	// register request handler
	logger.Println("registering HTTP handler")
	http.HandleFunc("/submit", handleReq(config, &turnstileSrv, mailTmpl))
	logger.Printf("listening to %s\n", config.ListenAddress)
	http.ListenAndServe(config.ListenAddress, nil)
}

func handleReq(config *Config, turnstileSrv *turnstile.Service, mailTmpl *template.Template) func(http.ResponseWriter, *http.Request) {
	return func(httpWr http.ResponseWriter, httpReq *http.Request) {
		if config.Turnstile.EnforceValidation {
			err := validateReq(httpReq, config, turnstileSrv)
			if err == errInvalidToken {
				// w.WriteHeader(http.StatusUnauthorized)
				// w.Write([]byte("401 - Turnstile token invalid"))
				logger.Printf(colorMsg("an error occurred for a request by %s (%s): invalid Turnstile token\n", Yellow), httpReq.FormValue("name"), httpReq.FormValue("email"))
				http.Redirect(httpWr, httpReq, config.Redirect.Error.Turnstile, http.StatusTemporaryRedirect)
				return
			} else if err != nil {
				// w.WriteHeader(http.StatusInternalServerError)
				// w.Write([]byte(fmt.Sprintf("500 - Something terrible happened: %s", err)))
				logger.Printf(colorMsg("an error occurred for a request by %s (%s): %s\n", Yellow), httpReq.FormValue("name"), httpReq.FormValue("email"), err)
				http.Redirect(httpWr, httpReq, config.Redirect.Error.Generic, http.StatusTemporaryRedirect)
				return
			}
		}

		// generate a request and send it
		req, err := fromHttpReq(httpReq, config)
		if err != nil {
			http.Redirect(httpWr, httpReq, config.Redirect.Error.Generic, http.StatusTemporaryRedirect)
			return
		}
		recv := config.Departments[httpReq.FormValue("sport")].Email
		logger.Printf("trying to send request %s to %s\n", req.display(), recv)
		err = sendReq(req, mailTmpl, recv, config)
		if err != nil {
			// w.WriteHeader(http.StatusInternalServerError)
			// w.Write([]byte(fmt.Sprintf("500 - Something terrible happened: %s", err)))
			logger.Printf(Yellow+"an error occurred sending request %s: %s\n", req.display(), err)
			http.Redirect(httpWr, httpReq, config.Redirect.Error.Mail, http.StatusTemporaryRedirect)
			return
		}

		// temporary redirect on success
		logger.Printf("request by %s delivered successfully to %s\n", req.Name, recv)
		http.Redirect(httpWr, httpReq, config.Redirect.Success, http.StatusTemporaryRedirect)
	}
}

func sendReq(req *Req, template *template.Template, recv string, config *Config) error {
	mailConfig := config.Mail
	var buf bytes.Buffer
	if err := template.Execute(&buf, req); err != nil {
		return err
	}
	// client, err := smtp.DialTLS(fmt.Sprintf("%s:%d", config.Server, config.Port), nil)
	// if err != nil {
	// 	return err
	// }
	auth := sasl.NewPlainClient("", mailConfig.User, mailConfig.Password)
	to := []string{recv}
	msg := strings.NewReader(fmt.Sprintf("From: %s via Formular <%s>\r\nReply-To: %s <%s>\r\nTo: %s\r\nSubject: %s%s\r\nMIME-version: 1.0;\r\nContent-Type: text/html; charset=\"UTF-8\";\r\n\r\n%s\r\n",
		req.Name, mailConfig.User, req.Name, req.Email, recv, mailConfig.Prefix, req.Name, buf.String()))
	err := smtp.SendMailTLS(fmt.Sprintf("%s:%d", mailConfig.Server, mailConfig.Port), auth, mailConfig.User, to, msg)
	if err != nil {
		return err
	}
	return nil
}

func validateReq(req *http.Request, config *Config, turnstileSrv *turnstile.Service) error {
	token := req.FormValue("cf-turnstile-response")
	ipAddr := req.RemoteAddr
	if xForwardedFor := req.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		ipAddr = xForwardedFor
	}

	ok, err := (*turnstileSrv).Verify(req.Context(), token, ipAddr)
	if err != nil {
		return err
	}
	if !ok {
		return errInvalidToken
	}
	return nil
}

func colorMsg(msg string, color string) string {
	return color + msg + Reset
}

func errExit(msg string, args ...any) {
	logger.Fatalf(colorMsg(msg+"\n", Red), args...)
	os.Exit(1)
}

func handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	for sig := range sigChan {
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			logger.Println(colorMsg("received SIGHUP, exiting", Yellow))
			os.Exit(0)
			return
		case syscall.SIGHUP:
			// reload service
			newConfig, err := getConfig(os.Getenv("CONFIG_FILE"))
			if err != nil {
				logger.Fatalf(colorMsg("failed to reload config: %s\n", Red), err)
				return
			}
			config = newConfig

			turnstileSrv = turnstile.New(turnstile.Config{
				Secret: config.Turnstile.Secret,
			})

			mailTmplPath := os.Getenv("MAIL_TEMPLATE_FILE")
			mailTmpl, err = template.New(mailTmplPath).ParseFiles(mailTmplPath)
			if err != nil {
				logger.Fatalf(colorMsg("failed to mail template: %s\n", Red), err)
				return
			}

			logger.Println(colorMsg("reload complete", Green))
		}
	}
}
