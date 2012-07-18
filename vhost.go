// vhost is a simple HTTP proxy server front-end for virtual host backends.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

var errorTemplate = template.Must(template.New("error").Parse(`<!DOCTYPE html>
<html>
<head>
<title>{{.StatusCode}} {{.StatusText}}</title>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
</head>
<body>
<h1>{{.StatusText}}</h1>
<pre>
{{.Message}}
</pre>
</body>
</html>
`))

var errorMessages = map[int]string{
	http.StatusNotFound: `Hmmm, there's nothing here matching that URL :-(

Maybe a typo in the URL?`,
	http.StatusInternalServerError: `Dang it, something broke :-(

Our hackers are working on it...

Please try again later`,
}

func writeDefaultError(w http.ResponseWriter, statusCode int) {
	w.WriteHeader(statusCode)
	errorTemplate.Execute(w, map[string]interface{}{
		"StatusCode": statusCode,
		"StatusText": http.StatusText(statusCode),
		"Message":    errorMessages[statusCode],
	})
}

// File is used for canned error responses
type File struct {
	Name       string `json:"file"`
	StatusCode int    // HTTP status code
}

func writeResponse(w http.ResponseWriter, statusCode int, f *File) {
	if f != nil {
		b, err := ioutil.ReadFile(f.Name)
		if err == nil {
			if f.StatusCode != 0 {
				statusCode = f.StatusCode
			}
			w.WriteHeader(statusCode)
			w.Write(b)
			return
		}
		log.Printf("Failed to read `%s`: %v", f.Name, err)
	}
	// send built-in response
	writeDefaultError(w, statusCode)
}

// A Server is a backend handling the forwarded request from Proxy
type Server struct {
	URL           string
	Backend       string
	InternalError *File
}

// Vhost is an http.Handler whose ServeHTTP forwards the request to 
// backend Servers according to the incoming request URL
type Vhost struct {
	config   Config
	servers  map[string]*Server
	handlers map[*Server]http.Handler
}

func (v *Vhost) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	originalUrl := r.Host + r.URL.Path
	server := v.servers[r.Host]
	if server == nil {
		log.Printf("Not Found: `%s`", originalUrl)
		writeResponse(w, http.StatusNotFound, v.config.NotFound)
		return
	}
	defer func() {
		if val := recover(); val != nil {
			log.Printf("Error proxying request `%s` to `%s`: %v", originalUrl, r.URL, val)
			er := server.InternalError
			if er == nil {
				er = v.config.InternalError
			}
			writeResponse(w, http.StatusInternalServerError, er)
		}
	}()
	v.handlers[server].ServeHTTP(w, r)
}

// PanicyRoundTripper decorates an http.RoundTripper and panics if 
// RoundTrip fails with an error
type PanicyRoundTripper struct {
	Transport http.RoundTripper
}

func (rt *PanicyRoundTripper) RoundTrip(r *http.Request) (res *http.Response, err error) {
	res, err = rt.Transport.RoundTrip(r)
	if err != nil {
		panic("proxy: " + err.Error())
	}
	return
}

var configFile = flag.String("config_file", "", "JSON config file. Required")

type Config struct {
	Port          int
	NotFound      *File
	InternalError *File
	Proxies       []*Server `json:"proxy"`
}

func applyConfig() *Vhost {
	file, err := os.Open(*configFile)
	if err != nil {
		log.Fatalf("Failed to open config file `%s`: %v\n", *configFile, err)
	}
	config := Config{}
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		log.Fatalf("Failed to decode JSON config file `%s`: %v\n", *configFile, err)
	}

	servers := make(map[string]*Server)
	handlers := make(map[*Server]http.Handler)
	for _, s := range config.Proxies {
		servers[s.URL] = s
		url, err := url.Parse(s.Backend)
		if err != nil {
			log.Fatalf("Failed to parse URL `%s`: %v\n", s.Backend, err)
		}
		rp := httputil.NewSingleHostReverseProxy(url)
		rp.Transport = &PanicyRoundTripper{http.DefaultTransport}
		handlers[s] = rp
	}

	return &Vhost{config, servers, handlers}
}

func main() {
	flag.Parse()
	if *configFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	vhost := applyConfig()

	err := http.ListenAndServe(fmt.Sprintf(":%d", vhost.config.Port), vhost)
	if err != nil {
		log.Fatalf("Unable to listen on port %d: %v\n", vhost.config.Port, err)
	}
}
