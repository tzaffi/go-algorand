// Trace is handy for keeping breadcrumbs of client request
package daemon

// type Traceable interface {
// 	StartTrace(nameFmt string, parts ...any)
// 	Tracing() bool
// 	Trace() *Trace
// }

// and for creating SDK fixtures from the results
type Trace struct {
	Daemon string `json:"daemon"`
	Name string   `json:"name"`

	Path     string `json:"path"`
	Resource string `json:"resource"`
	Method   string `json:"method"`

	// only for endpoints receiving raw bytes. cf. `rawRequestPaths`:
	BytesB64 *string `json:"bytesB64"`

	// for the non-raw bytes endpoints:
	Params *map[string][]string `json:"params"`

	EncodeJSON  bool    `json:"encodeJSON"`
	DecodeJSON  bool    `json:"decodeJSON"`
	StatusCode  int     `json:"statusCode"`
	ResponseErr *string `json:"responseErr"`

	// raw response (TOOD: and/or) b64 dencoded
	Response *string 	`json:"response"`
	ResponseB64 *string `json:"responseB64"`

	// for DecodeJSON responses only:
	ParsedResponseType string      `json:"parsedResponseGoType"`
	ParsedResponse     interface{} `json:"parsedResponse"`
}
