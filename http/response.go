package http

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"log"
	nh "net/http"
	"path/filepath"
	"reflect"
	"strings"

	texttemplate "text/template"

	"github.com/ugorji/go/codec"
)

// Respond is used to return data to the client with a custom http code
// The returned content-type will respect the Accept header for application/json,
// application/cbor and application/xml (and application/html and text/html if enabled
// with SetHTMLTemplatePaths).
func Respond(w nh.ResponseWriter, r *nh.Request, statusCode int, response interface{}) {
	contentType := decideAccept(r.Header) // request accept is response content-type

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(statusCode) // commit point. contentType and statusCode are now on the wire
	var err error
	switch contentType {
	case "text/plain":
		if response != nil {
			fmt.Fprintf(w, "%v", response)
		}
	case "text/html":
		// Return html as a complete page.
		enc := successEncoderForHTML{htmlTemplatePath: responseTemplatePaths.TextHTML, w: w}
		err = enc.Encode(response)
	case "application/html":
		// Return html as a fragment for embedding in another page.
		enc := successEncoderForHTML{htmlTemplatePath: responseTemplatePaths.ApplicationHTML, w: w}
		err = enc.Encode(response)
	case "application/cbor":
		cbor := &codec.CborHandle{}
		enc := codec.NewEncoder(w, cbor)
		err = enc.Encode(response)
	case "application/json":
		json := &codec.JsonHandle{}
		json.Canonical = true
		enc := codec.NewEncoder(w, json)
		err = enc.Encode(response)
	case "application/xml":
		var bytes []byte
		bytes, err = xml.Marshal(response)
		if err == nil {
			_, err = w.Write(bytes)
		}
	default:
		panic(fmt.Sprintf("[RespondOk] unexpected Accept header: %s", contentType)) // decideAccept must ensure that this never happens
	}
	if err != nil {
		log.Println("[RespondOk] Encode Error:", contentType, err)
		// There is no point in calling RespondError() because calling w.WriteHeader(...) again
		// will have no effect on the returned status - w.WriteHeader(...) has already been called
		// once, implicitly, by the first w.Write(...).
		// We could encode the response to our own buffer before calling w.WriteHeader(...) but then we
		// lose all the performance advantages of streaming the response.
		// So instead we prevoke the http server into breaking the connection prematurely which will
		// result in nginx returning 502 to the caller.
		panic(fmt.Sprintf("[RespondOk] failed to send response. Content-Type: %s. Error: %s", contentType, err.Error()))
	}
}

// RespondOk is used to return data to the client with a 200 http code
// The returned content-type will respect the Accept header for application/json,
// application/cbor and application/xml (and application/html and text/html if enabled
// with SetHTMLTemplatePaths).
func RespondOk(w nh.ResponseWriter, r *nh.Request, response interface{}) {
	Respond(w, r, nh.StatusOK, response)
}

// RespondError is used to return errors to the client
// The returned content-type will respect the Accept header for application/json,
// application/cbor and application/xml (and application/html and text/html if enabled
// with SetHTMLTemplatePaths).
// Be careful in here not to recurse (by calling RespondError() again) when there is an error.
// For the detail parameter, only error, string and RespondErrorDetail types are useful.
func RespondError(w nh.ResponseWriter, r *nh.Request, statusCode int, detail ...interface{}) {
	contentType := decideAccept(r.Header) // request accept is response content-type

	response := ResponseError{
		StatusCode: statusCode,
		Message:    nh.StatusText(statusCode),
	}

	for _, v := range detail {
		if err, ok := v.(error); ok {
			response.Message = err.Error()
		} else if str, ok := v.(string); ok {
			if len(str) > 0 && str[0] == '#' {
				response.Documentation = str[1:]
			} else {
				response.Message = str
			}
		} else if d, ok := v.(RespondErrorDetail); ok {
			response.Details = append(response.Details, string(d))
		}
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(response.StatusCode) // commit point. contentType and StatusCode are now on the wire

	var err error
	switch contentType {
	case "text/plain":
		// Return plain text
		fmt.Fprintf(w, "%v", response.Message)
	case "text/html":
		// Return html, as a complete page.
		enc := errorEncoderForHTML{htmlTemplateDef: sErrorTemplateTextHTML, w: w}
		err = enc.Encode(response)
	case "application/html":
		// Return html as a fragment for embedding in another page.
		enc := errorEncoderForHTML{htmlTemplateDef: sErrorTemplateApplicationHTML, w: w}
		err = enc.Encode(response)
	case "application/cbor":
		cbor := &codec.CborHandle{}
		enc := codec.NewEncoder(w, cbor)
		err = enc.Encode(response)
	case "application/json":
		json := &codec.JsonHandle{}
		json.Canonical = true
		enc := codec.NewEncoder(w, json)
		err = enc.Encode(response)
	case "application/xml":
		var bytes []byte
		bytes, err = xml.Marshal(response)
		if err == nil {
			_, err = w.Write(bytes)
		}
	default:
		panic(fmt.Sprintf("[RespondError] unexpected Accept header: %s", contentType)) // decideAccept must guarantee that this never happens
	}
	if err != nil {
		log.Println("[RespondError] Encode Error:", contentType, response.Message, err)
		// There is no point in calling RespondError() again because calling w.WriteHeader(...) again
		// will have no effect on the returned status.
		// We could encode the response to our own buffer before calling w.WriteHeader(...) but then we
		// lose all the performance advantages of streaming the response.
		// So instead we prevoke the http server into breaking the connection prematurely which will
		// result in nginx returning 502 to the caller.
		panic(fmt.Sprintf("[RespondError] failed to send response. Content-Type: %s Error: %s", contentType, err.Error()))
	}
}

// RespondErrorDetail allows a RespondError detail parameter to end up in the Details array
type RespondErrorDetail string

//////////////////////////////////////////////////////////////////////////
// Implementation

// responseTemplatePaths contains the paths to HTML template files for formatting html responses.
// Usually only used for testing.
var responseTemplatePaths struct {
	ApplicationHTML string
	TextHTML        string
}

func cleanSentenceJoin(l, r string) string {
	if l == "" {
		return r
	}
	if r == "" {
		return l
	}
	return strings.TrimRight(l, ". \t\r\n") + ". " + r
}

// ResponseError structure holds standard fields for errors.
// Public so that when two of our own applications communicate; one can parse the error received from the other.
type ResponseError struct {
	StatusCode    int      `json:"status_code"`
	Message       string   `json:"message"`
	Documentation string   `json:"documentation"`
	Details       []string `json:"details,omitempty"`
}

// decideContentType provides consistent handling of the content-type header value.
func decideContentType(requestHeader nh.Header) string {
	return decideBodyType(requestHeader.Get("Content-Type"), false)
}

// decideAccept provides consistent handling of the accept header value.
func decideAccept(requestHeader nh.Header) string {
	return decideBodyType(requestHeader.Get("Accept"), true)
}

// decideBodyType provides consistent handling of content-type and accept header values.
// Also consistently handles accept types that depend on HTML templates (which may not exist).
// Also consistently handles FHIR aliasing - we treat fhir-foo as foo.
func decideBodyType(headerValue string, isResponse bool) string {

	// Can only return HTML if the relevant template exists.
	// We want the same response type choice to occur for errors as for success.
	switch headerValue {
	case "text/plain":
		if isResponse {
			return headerValue
		}
	case "text/html":
		if isResponse && responseTemplatePaths.TextHTML != "" {
			return headerValue
		}
	case "application/html":
		if isResponse && responseTemplatePaths.ApplicationHTML != "" {
			return headerValue
		}
	case "application/cbor":
		return headerValue
	case "", "application/json", "application/fhir+json":
		return "application/json"
	case "application/xml", "application/fhir+xml":
		return "application/xml"
	case "application/fail": // for testing panic
		return headerValue
	}

	// Force default behaviour
	return "application/json"
}

type successEncoderForHTML struct {
	htmlTemplatePath string
	w                nh.ResponseWriter
}

func (encoder successEncoderForHTML) Encode(response interface{}) error {
	_, filename := filepath.Split(encoder.htmlTemplatePath)

	// Use text.template to avoid escaping the response which assumed to already be html
	t, err := texttemplate.New(filename).Funcs(texttemplate.FuncMap{
		"fieldExists": fieldExists,
	}).ParseFiles(encoder.htmlTemplatePath)
	if err != nil {
		return err
	}

	var out bytes.Buffer
	err = t.Execute(&out, &response)
	if err != nil {
		return err
	}

	encoder.w.Write([]byte(out.String()))
	return nil
}

func fieldExists(name string, data interface{}) bool {
	// From https://stackoverflow.com/questions/44675087/golang-template-variable-isset,
	// with the addition of converting from reflect.Interface
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false
	}
	return v.FieldByName(name).IsValid()
}

type errorEncoderForHTML struct {
	htmlTemplateDef string
	w               nh.ResponseWriter
}

func (encoder errorEncoderForHTML) Encode(response ResponseError) error {
	// Use text.template to avoid escaping the response which is already html
	// Be careful in here not to recurse (by calling RespondError() again) when there is an error.
	t, err := texttemplate.New("ErrorOccured").Parse(encoder.htmlTemplateDef)
	if err != nil {
		return err
	}

	response.Message = prepareStringAsHTML(response.Message)
	response.Documentation = prepareStringAsHTML(response.Documentation)

	var out bytes.Buffer
	err = t.Execute(&out, &response)
	if err != nil {
		return err
	}

	encoder.w.Write([]byte(out.String()))
	return nil
}

func prepareStringAsHTML(s string) string {
	return strings.Replace(strings.Replace(strings.TrimSpace(s), "\n", "<br/>", -1), "\r", "", -1)
}

const sErrorTemplateApplicationHTML = `<table>
<tr><td>StatusCode:</td><td id="errorcode">{{.StatusCode}}</td></tr>
<tr><td>Message:</td><td id="errormessage">{{.Message}}</td></tr>
<tr><td>Documentation:</td><td id="errordocumentation">{{.Documentation}}</td></tr>
</table>`

const sErrorTemplateTextHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<title>An error occured</title>
<style>
table, tr, td {
	border: 1px solid black;
	border-collapse: collapse;
}
#errormessage {
	font-weight: bold;
}
</style>
</head>
<body>` + sErrorTemplateApplicationHTML + `
</body>
</html>
`

// SetHTMLTemplatePaths allows the application to specify the paths to HTML tempate
// files that will be used for responses with HTML content-types.
// If a template path is empty, it will not be used and the default content-type
// (application/json) will be used instead.
// Template paths are empty by default.
func SetHTMLTemplatePaths(applicationHTML, textHTML string) {
	responseTemplatePaths.ApplicationHTML = applicationHTML
	responseTemplatePaths.TextHTML = textHTML
}
