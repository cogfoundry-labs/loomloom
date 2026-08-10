package authflow

import (
	"bytes"
	"html/template"
	"net/http"
)

type CallbackPageVariant string

const (
	CallbackPageGeneric      CallbackPageVariant = ""
	CallbackPageCogFoundry   CallbackPageVariant = "cogfoundry"
	CallbackPageShengSuanYun CallbackPageVariant = "shengsuanyun"

	callbackPageCSP = "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
)

type callbackPageView struct {
	Status      string
	Role        string
	Title       string
	Kicker      string
	Heading     string
	Description string
	Footer      string
	ShowCommand bool
}

type callbackPageRenderer struct {
	template *template.Template
	language string
	success  callbackPageView
	failure  callbackPageView
}

func writeCallbackPage(w http.ResponseWriter, variant CallbackPageVariant, success bool) {
	renderer := callbackPageRendererFor(variant)
	view := renderer.failure
	status := http.StatusBadRequest
	if success {
		view = renderer.success
		status = http.StatusOK
	}

	var page bytes.Buffer
	if err := renderer.template.Execute(&page, view); err != nil {
		setCallbackPageSecurityHeaders(w.Header())
		http.Error(w, "Unable to render authorization result", http.StatusInternalServerError)
		return
	}

	setCallbackPageSecurityHeaders(w.Header())
	w.Header().Set("Content-Language", renderer.language)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = page.WriteTo(w)
}

func callbackPageRendererFor(variant CallbackPageVariant) callbackPageRenderer {
	switch variant {
	case CallbackPageCogFoundry:
		return cogFoundryCallbackPageRenderer
	case CallbackPageShengSuanYun:
		return shengSuanYunCallbackPageRenderer
	default:
		return genericCallbackPageRenderer
	}
}

func mustParseCallbackPage(name, source string) *template.Template {
	return template.Must(template.New(name).Parse(source))
}

func setCallbackPageSecurityHeaders(header http.Header) {
	header.Set("Cache-Control", "no-store")
	header.Set("Content-Security-Policy", callbackPageCSP)
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
}
