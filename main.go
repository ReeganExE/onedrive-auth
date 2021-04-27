package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/urfave/cli/v2"
)

//go:embed result.html index.html
var tpl embed.FS

type args struct {
	OrgID        string
	ClientID     string
	ClientSecret string
	Scope        string
	RedirectURI  string
	CallbackPort string
}

func main() {
	log.SetOutput(os.Stdout)
	app := &cli.App{
		Action: handle,
		Name:   "OneDrive Auth Util",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "org-id",
				EnvVars: []string{"OD_ORG_ID"},
			},
			&cli.StringFlag{
				Name:    "client-id",
				EnvVars: []string{"OD_CLIENT_ID"},
			},
			&cli.StringFlag{
				Name:    "client-secret",
				EnvVars: []string{"OD_CLIENT_SECRET"},
			},
			&cli.StringFlag{
				Name:    "scope",
				EnvVars: []string{"OD_SCOPE"},
				Value:   "Files.ReadWrite offline_access",
			},
			&cli.StringFlag{
				Name:    "redirect-uri",
				EnvVars: []string{"OD_REDIRECT_URI"},
				Value:   "http://localhost:6789",
			},
			&cli.IntFlag{
				Name:  "port",
				Value: 6789,
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func handle(context *cli.Context) error {
	conf := &args{
		OrgID:        context.String("org-id"),
		ClientID:     context.String("client-id"),
		ClientSecret: context.String("client-secret"),
		Scope:        context.String("scope"),
		RedirectURI:  context.String("redirect-uri"),
		CallbackPort: context.String("port"),
	}

	// Handle auth callback
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			code := r.URL.Query().Get("code")
			if code == "" {
				httpError(w, 400, "Invalid code")
			} else {
				getToken(w, code, conf)
			}
		} else {
			http.NotFound(w, r)
		}
	})

	// Handle submitsion
	http.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

		params := url.Values{}
		params.Add("client_id", r.Form.Get("ClientID"))
		params.Add("scope", conf.Scope)
		params.Add("response_type", "code")
		params.Add("redirect_uri", conf.RedirectURI)

		conf.OrgID = r.Form.Get("OrgID")
		conf.ClientID = r.Form.Get("ClientID")
		conf.ClientSecret = r.Form.Get("ClientSecret")

		authURL := "https://login.microsoftonline.com/" + r.Form.Get("OrgID") + "/oauth2/v2.0/authorize?" + params.Encode()
		http.Redirect(w, r, authURL, http.StatusMovedPermanently)
	})

	// Starter
	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFS(tpl, "index.html")
		if err != nil {
			panic(err)
		}
		if err := tmpl.Execute(w, conf); err != nil {
			panic(err)
		}
	})

	// Listener
	n, err := net.Listen("tcp", "127.0.0.1:"+conf.CallbackPort)
	if err != nil {
		log.Fatal(err)
	}
	openBrowser("http://localhost:" + conf.CallbackPort + "/start")

	return http.Serve(n, nil)
}

func httpError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	w.Header().Set("content-type", "text/html")
	fmt.Fprintf(w, `<!doctype html><html>`+msg+`.<br><a href="/start">Start again</a></html>`)
}

func getToken(w http.ResponseWriter, code string, conf *args) {
	u := "https://login.microsoftonline.com/" + conf.OrgID + "/oauth2/v2.0/token"

	// Query params
	params := url.Values{}
	params.Add("client_id", conf.ClientID)
	params.Add("scope", conf.Scope)
	params.Add("code", code)
	params.Add("redirect_uri", conf.RedirectURI)
	params.Add("grant_type", "authorization_code")
	params.Add("client_secret", conf.ClientSecret)

	payload := strings.NewReader(params.Encode())

	req, _ := http.NewRequest("POST", u, payload)

	req.Header.Add("user-agent", "okhttp/3.6.0")
	req.Header.Add("content-type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		var v *AccessToken
		json.NewDecoder(res.Body).Decode(&v)
		renderResultHTML(w, v)
	} else {
		d, _ := io.ReadAll(res.Body)
		fmt.Println(res.Status, string(d))
		httpError(w, 500, "Something went wrong")
	}
}

func renderResultHTML(w io.Writer, v *AccessToken) {
	tmpl, err := template.ParseFS(tpl, "result.html")
	if err != nil {
		panic(err)
	}
	if err := tmpl.Execute(w, v); err != nil {
		panic(err)
	}
}

func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}
}

type AccessToken struct {
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int64  `json:"expires_in"`
	EXTExpiresIn int64  `json:"ext_expires_in"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
