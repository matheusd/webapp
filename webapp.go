package webapp

import (	
	"fmt"
	"net/http"
	"encoding/json"	
)

// Error is the structure of a response that encodes a processing error
type Error struct {
	Code int
	ErrorID string
	OrigError error
	Extra interface{}
}

// Error matches the error interface and returns a simple string message.
func (err Error) Error() string {
	orig := "nil"
	if err.OrigError != nil {
		orig = err.OrigError.Error()
	}
	return fmt.Sprintf("REST Error %d: %s (original: %s)",
		err.Code, err.ErrorID, orig)
}

type ErrorIntf interface {
	error
	WebAppError() (int, string)
}

// Response of a request that should use a specific http return code
type Response struct {
	Code int
	Payload interface{}
}

// DoneResponse is a dummy response class that indicates the previous handler
// has already written the response
var DoneResponse Response = Response{
	Code: 666,
}

// HandlerFunc is an adapter for handlers that return a response
type HandlerFunc func (http.ResponseWriter, *http.Request) interface{}

// ServeWebApp executes the function and returns its response
func (f HandlerFunc) ServeWebApp(w http.ResponseWriter, r *http.Request) interface{} {
	return f(w, r)
}

// Handler is the interface that represents a handler of a web app request
type Handler interface{
	ServeWebApp(http.ResponseWriter, *http.Request) interface{}
}

// Validatable is an interface for a struct that can be validated
type Validatable interface {
	Validate() error
}


// DecodeRequest decodes an http request according to the Content-Type header
// (right now only supports json)
func DecodeRequest(req *http.Request, reqData interface{}) error {
	// TODO: Support more than just json depending on Content-Type header
	
	decoder := json.NewDecoder(req.Body)
    defer req.Body.Close()
    if err := decoder.Decode(reqData); err != nil {		
        return Error{
			Code: http.StatusBadRequest,
			ErrorID: "INVALIDREQJSON",
			OrigError: err,
		}
	}

	if validatable, ok := reqData.(Validatable) ; ok {
		if err := validatable.Validate() ; err != nil {
			return Error{
				Code: http.StatusBadRequest,
				ErrorID: "VALIDATIONERROR",
				OrigError: err,
			}			
		}
	}
	
	return nil
}

// EncodeResponse encodes a response object with the preferred encoding specified
// on the Accepts http header. Right now supports only json.
func EncodeResponse(w http.ResponseWriter, req *http.Request, respData interface{}) {
	var (
		toMarshalData interface{}
		responseCode int
		marshalErr error
		response []byte
	)

	if respData == DoneResponse { return }

	switch resp:= respData.(type) {
		case Response:
			toMarshalData = resp.Payload
			responseCode = resp.Code
		case Error:
			toMarshalData = resp
			responseCode = resp.Code
		case ErrorIntf:
			code, id := resp.WebAppError()
			responseCode = code
			toMarshalData = Error{
				Code: code,
				OrigError: resp,
				ErrorID: id,
			}
		case error:
			// unhandled error
			responseCode = http.StatusInternalServerError
			toMarshalData = Error{
				Code: responseCode,
				OrigError: resp,
				ErrorID: "NONWEBAPPERROR",
			}
		default:
			toMarshalData = respData
			responseCode = http.StatusOK
	}

	// TODO: Support more than just json depending on the Accepts Header
	w.Header().Set("Content-Type", "application/json")

	response, marshalErr = json.Marshal(toMarshalData)
	if marshalErr != nil {
		responseCode = http.StatusInternalServerError
		response = []byte(`{"Code": 500, "ErrorId": "RESPONSEMARSHALERROR"}`)
	}

	w.WriteHeader(responseCode)
	w.Write(response)
}

// HandleFunc is a helper method that converts a webapp.HandlerFunc into an
// http.HandlerFunc. This works as a middleware/filter, getting the response
// from the WebApp function and encoding it to the client.
func HandleFunc(handler HandlerFunc) http.HandlerFunc {
	return func (w http.ResponseWriter, req *http.Request) {
		respData := handler.ServeWebApp(w, req)
		EncodeResponse(w, req, respData)
	}
}

// NewBadRequestError returns an error with code of http.StatusBadRequest. Meant
// as a shortcut helper function.
func NewBadRequestError(errorID string) Error {
	return Error{
		Code: http.StatusBadRequest,
		ErrorID: errorID,
	}
}
