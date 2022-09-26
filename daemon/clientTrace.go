// Trace is handy for keeping breadcrumbs of client request
package daemon

/* TODO: will this interface be of any use?
type Traceable interface {
	StartTrace(nameFmt string, parts ...any)
	Tracing() bool
	Trace() *Trace
}
*/

type ResponseComparison int64

const (
	Equality ResponseComparison = iota // default to comparing using ==
	SetEquality // only look at the contents of collection regardless of order
	ByLength // only compare using len()
	Incomparable // don't attempt to compare
)

func (trace *Trace) WithComparator(comparator ResponseComparison) *Trace{
	if trace != nil {
		trace.Comparator = comparator
	}
	return trace
}

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

	Comparator ResponseComparison `json:"comparator"`
}
