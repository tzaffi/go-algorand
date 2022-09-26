// Trace is handy for keeping breadcrumbs of client request
package daemon

/* TODO: will this interface be of any use?
type Traceable interface {
	StartTrace(nameFmt string, parts ...any)
	Tracing() bool
	Trace() *Trace
}
*/

type TraceComparison int64

const (
	Equality TraceComparison = iota // default to comparing using ==
	SetEquality // only look at the contents of collection regardless of order
	ByLength // only compare using len()
	Incomparable // don't attempt to compare
)

func (trace *Trace) WithRespComparator(comparator TraceComparison) *Trace{
	if trace != nil {
		trace.ResponseComparator = comparator
	}
	return trace
}

func (trace *Trace) WithReqComparator(comparator TraceComparison) *Trace{
	if trace != nil {
		trace.RequestComparator = comparator
	}
	return trace
}

// and for creating SDK fixtures from the results
type Trace struct {
	Daemon 	 string `json:"daemon"`
	Name 	 string `json:"name"`
	Volatile bool 	`json:"volatile"`

	Path     string `json:"path"`
	Resource string `json:"resource"`
	Method   string `json:"method"`

	RequestType 	string 		`json:"requestGoType"`
	RequestObject   interface{} `json:"requestObject"`
	RequestBytesB64 *string 	`json:"requestBytesB64"`

	// for the non-raw bytes endpoints:
	Params *map[string][]string `json:"params"`

	EncodeJSON  bool    `json:"encodeJSON"`
	DecodeJSON  bool    `json:"decodeJSON"`
	StatusCode  int     `json:"statusCode"`
	ResponseErr *string `json:"responseErr"`

	Response *string 	`json:"response"`
	ResponseB64 *string `json:"responseB64"`

	// for DecodeJSON responses only:
	ParsedResponseType string      `json:"parsedResponseGoType"`
	ParsedResponse     interface{} `json:"parsedResponse"`

	RequestComparator TraceComparison `json:"requestComparator"`
	ResponseComparator TraceComparison `json:"responseComparator"`
}
