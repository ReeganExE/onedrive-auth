package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/urfave/cli/v2"
)

//go:embed index.html
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
				Name:     "org-id",
				Required: true,
				EnvVars:  []string{"OD_ORG_ID"},
			},
			&cli.StringFlag{
				Name:     "client-id",
				Required: true,
				EnvVars:  []string{"OD_CLIENT_ID"},
			},
			&cli.StringFlag{
				Name:     "client-secret",
				Required: true,
				EnvVars:  []string{"OD_CLIENT_SECRET"},
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

	authURL := "https://login.microsoftonline.com/" +
		conf.OrgID + "/oauth2/v2.0/authorize?client_id=" +
		conf.ClientID + "&scope=" + url.QueryEscape(conf.Scope) +
		"&response_type=code&redirect_uri=" + conf.RedirectURI
	ux, _ := url.Parse(authURL)
	authURL = ux.String()
	log.Println(authURL)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(200)
			code := r.URL.Query().Get("code")
			if code == "" {
				http.Error(w, "Invalid authenticate", 400)
			} else {
				w.Write(getToken(code, conf))
			}
		} else {
			http.NotFound(w, r)
		}
	})
	exec.Command("open", authURL).Start()
	http.ListenAndServe("127.0.0.1:"+conf.CallbackPort, nil)
	return nil
}

func getToken(code string, conf *args) []byte {
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
		return renderHTML(v)
	}
	body, _ := ioutil.ReadAll(res.Body)

	return body
}

func renderHTML(v *AccessToken) []byte {
	tmpl, err := template.ParseFS(tpl, "index.html")
	if err != nil {
		panic(err)
	}

	w := bytes.NewBuffer(nil)

	if err := tmpl.Execute(w, v); err != nil {
		panic(err)
	}

	return w.Bytes()
}

type AccessToken struct {
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int64  `json:"expires_in"`
	EXTExpiresIn int64  `json:"ext_expires_in"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
