package zoraxy_plugin

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type PluginUiRouter struct {
	PluginID       string    //The ID of the plugin
	TargetFs       *embed.FS //The embed.FS where the UI files are stored
	TargetFsPrefix string    //The prefix of the embed.FS where the UI files are stored, e.g. /web
	HandlerPrefix  string    //The prefix of the handler used to route this router, e.g. /ui
}

// NewPluginEmbedUIRouter creates a new PluginUiRouter with embed.FS
// The targetFsPrefix is the prefix of the embed.FS where the UI files are stored
// The targetFsPrefix should be relative to the root of the embed.FS
// The targetFsPrefix should start with a slash (e.g. /web) that corresponds to the root folder of the embed.FS
// The handlerPrefix is the prefix of the handler used to route this router
// The handlerPrefix should start with a slash (e.g. /ui) that matches the http.Handle path
// All prefix should not end with a slash
func NewPluginEmbedUIRouter(pluginID string, targetFs *embed.FS, targetFsPrefix string, handlerPrefix string) *PluginUiRouter {
	//Make sure all prefix are in /prefix format
	if !strings.HasPrefix(targetFsPrefix, "/") {
		targetFsPrefix = "/" + targetFsPrefix
	}
	targetFsPrefix = strings.TrimSuffix(targetFsPrefix, "/")

	if !strings.HasPrefix(handlerPrefix, "/") {
		handlerPrefix = "/" + handlerPrefix
	}
	handlerPrefix = strings.TrimSuffix(handlerPrefix, "/")

	//Return the PluginUiRouter
	return &PluginUiRouter{
		PluginID:       pluginID,
		TargetFs:       targetFs,
		TargetFsPrefix: targetFsPrefix,
		HandlerPrefix:  handlerPrefix,
	}
}

func (p *PluginUiRouter) populateCSRFToken(r *http.Request, fsHandler http.Handler) http.Handler {
	//Get the CSRF token from header
	csrfToken := r.Header.Get("X-Zoraxy-Csrf")
	if csrfToken == "" {
		csrfToken = "missing-csrf-token"
	}

	//Return the middleware
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the request is for an HTML file
		if strings.HasSuffix(r.URL.Path, "/") {
			// Redirect to the index.html
			http.Redirect(w, r, r.URL.Path+"index.html", http.StatusFound)
			return
		}
		if strings.HasSuffix(r.URL.Path, ".html") {
			//Read the target file from embed.FS
			targetFilePath := strings.TrimPrefix(r.URL.Path, "/")
			targetFilePath = p.TargetFsPrefix + "/" + targetFilePath
			targetFilePath = strings.TrimPrefix(targetFilePath, "/")
			targetFileContent, err := fs.ReadFile(*p.TargetFs, targetFilePath)
			if err != nil {
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}
			body := string(targetFileContent)
			body = strings.ReplaceAll(body, "{{.csrfToken}}", csrfToken)
			http.ServeContent(w, r, r.URL.Path, time.Now(), strings.NewReader(body))
			return
		}

		//Call the next handler
		fsHandler.ServeHTTP(w, r)
	})

}

// GetHttpHandler returns the http.Handler for the PluginUiRouter
func (p *PluginUiRouter) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//Remove the plugin UI handler path prefix
		rewrittenURL := r.RequestURI
		rewrittenURL = strings.TrimPrefix(rewrittenURL, p.HandlerPrefix)
		rewrittenURL = strings.ReplaceAll(rewrittenURL, "//", "/")
		r.URL, _ = url.Parse(rewrittenURL)
		r.RequestURI = rewrittenURL

		//Serve the file from the embed.FS
		subFS, err := fs.Sub(*p.TargetFs, strings.TrimPrefix(p.TargetFsPrefix, "/"))
		if err != nil {
			fmt.Println(err.Error())
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Replace {{csrf_token}} with the actual CSRF token and serve the file
		p.populateCSRFToken(r, http.FileServer(http.FS(subFS))).ServeHTTP(w, r)
	})
}
