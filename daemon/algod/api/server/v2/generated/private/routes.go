// Package private provides primitives to interact the openapi HTTP API.
//
// Code generated by github.com/algorand/oapi-codegen DO NOT EDIT.
package private

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"github.com/algorand/oapi-codegen/pkg/runtime"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
	"net/http"
	"strings"
)

// ServerInterface represents all server handlers.
type ServerInterface interface {
	// Aborts a catchpoint catchup.
	// (DELETE /v2/catchup/{catchpoint})
	AbortCatchup(ctx echo.Context, catchpoint string) error
	// Starts a catchpoint catchup.
	// (POST /v2/catchup/{catchpoint})
	StartCatchup(ctx echo.Context, catchpoint string) error
	// Return a list of participation keys
	// (GET /v2/participation)
	GetParticipationKeys(ctx echo.Context) error
	// Add a participation key to the node
	// (POST /v2/participation)
	AddParticipationKey(ctx echo.Context) error
	// Delete a given participation key by ID
	// (DELETE /v2/participation/{participation-id})
	DeleteParticipationKeyByID(ctx echo.Context, participationId string) error
	// Get participation key info given a participation ID
	// (GET /v2/participation/{participation-id})
	GetParticipationKeyByID(ctx echo.Context, participationId string) error
	// Append state proof keys to a participation key
	// (POST /v2/participation/{participation-id})
	AppendKeys(ctx echo.Context, participationId string) error

	// (POST /v2/shutdown)
	ShutdownNode(ctx echo.Context, params ShutdownNodeParams) error
}

// ServerInterfaceWrapper converts echo contexts to parameters.
type ServerInterfaceWrapper struct {
	Handler ServerInterface
}

// AbortCatchup converts echo context to params.
func (w *ServerInterfaceWrapper) AbortCatchup(ctx echo.Context) error {

	validQueryParams := map[string]bool{
		"pretty": true,
	}

	// Check for unknown query parameters.
	for name, _ := range ctx.QueryParams() {
		if _, ok := validQueryParams[name]; !ok {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unknown parameter detected: %s", name))
		}
	}

	var err error
	// ------------- Path parameter "catchpoint" -------------
	var catchpoint string

	err = runtime.BindStyledParameter("simple", false, "catchpoint", ctx.Param("catchpoint"), &catchpoint)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter catchpoint: %s", err))
	}

	ctx.Set("api_key.Scopes", []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.AbortCatchup(ctx, catchpoint)
	return err
}

// StartCatchup converts echo context to params.
func (w *ServerInterfaceWrapper) StartCatchup(ctx echo.Context) error {

	validQueryParams := map[string]bool{
		"pretty": true,
	}

	// Check for unknown query parameters.
	for name, _ := range ctx.QueryParams() {
		if _, ok := validQueryParams[name]; !ok {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unknown parameter detected: %s", name))
		}
	}

	var err error
	// ------------- Path parameter "catchpoint" -------------
	var catchpoint string

	err = runtime.BindStyledParameter("simple", false, "catchpoint", ctx.Param("catchpoint"), &catchpoint)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter catchpoint: %s", err))
	}

	ctx.Set("api_key.Scopes", []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.StartCatchup(ctx, catchpoint)
	return err
}

// GetParticipationKeys converts echo context to params.
func (w *ServerInterfaceWrapper) GetParticipationKeys(ctx echo.Context) error {

	validQueryParams := map[string]bool{
		"pretty": true,
	}

	// Check for unknown query parameters.
	for name, _ := range ctx.QueryParams() {
		if _, ok := validQueryParams[name]; !ok {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unknown parameter detected: %s", name))
		}
	}

	var err error

	ctx.Set("api_key.Scopes", []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.GetParticipationKeys(ctx)
	return err
}

// AddParticipationKey converts echo context to params.
func (w *ServerInterfaceWrapper) AddParticipationKey(ctx echo.Context) error {

	validQueryParams := map[string]bool{
		"pretty": true,
	}

	// Check for unknown query parameters.
	for name, _ := range ctx.QueryParams() {
		if _, ok := validQueryParams[name]; !ok {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unknown parameter detected: %s", name))
		}
	}

	var err error

	ctx.Set("api_key.Scopes", []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.AddParticipationKey(ctx)
	return err
}

// DeleteParticipationKeyByID converts echo context to params.
func (w *ServerInterfaceWrapper) DeleteParticipationKeyByID(ctx echo.Context) error {

	validQueryParams := map[string]bool{
		"pretty": true,
	}

	// Check for unknown query parameters.
	for name, _ := range ctx.QueryParams() {
		if _, ok := validQueryParams[name]; !ok {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unknown parameter detected: %s", name))
		}
	}

	var err error
	// ------------- Path parameter "participation-id" -------------
	var participationId string

	err = runtime.BindStyledParameter("simple", false, "participation-id", ctx.Param("participation-id"), &participationId)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter participation-id: %s", err))
	}

	ctx.Set("api_key.Scopes", []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.DeleteParticipationKeyByID(ctx, participationId)
	return err
}

// GetParticipationKeyByID converts echo context to params.
func (w *ServerInterfaceWrapper) GetParticipationKeyByID(ctx echo.Context) error {

	validQueryParams := map[string]bool{
		"pretty": true,
	}

	// Check for unknown query parameters.
	for name, _ := range ctx.QueryParams() {
		if _, ok := validQueryParams[name]; !ok {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unknown parameter detected: %s", name))
		}
	}

	var err error
	// ------------- Path parameter "participation-id" -------------
	var participationId string

	err = runtime.BindStyledParameter("simple", false, "participation-id", ctx.Param("participation-id"), &participationId)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter participation-id: %s", err))
	}

	ctx.Set("api_key.Scopes", []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.GetParticipationKeyByID(ctx, participationId)
	return err
}

// AppendKeys converts echo context to params.
func (w *ServerInterfaceWrapper) AppendKeys(ctx echo.Context) error {

	validQueryParams := map[string]bool{
		"pretty": true,
	}

	// Check for unknown query parameters.
	for name, _ := range ctx.QueryParams() {
		if _, ok := validQueryParams[name]; !ok {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unknown parameter detected: %s", name))
		}
	}

	var err error
	// ------------- Path parameter "participation-id" -------------
	var participationId string

	err = runtime.BindStyledParameter("simple", false, "participation-id", ctx.Param("participation-id"), &participationId)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter participation-id: %s", err))
	}

	ctx.Set("api_key.Scopes", []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.AppendKeys(ctx, participationId)
	return err
}

// ShutdownNode converts echo context to params.
func (w *ServerInterfaceWrapper) ShutdownNode(ctx echo.Context) error {

	validQueryParams := map[string]bool{
		"pretty":  true,
		"timeout": true,
	}

	// Check for unknown query parameters.
	for name, _ := range ctx.QueryParams() {
		if _, ok := validQueryParams[name]; !ok {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unknown parameter detected: %s", name))
		}
	}

	var err error

	ctx.Set("api_key.Scopes", []string{""})

	// Parameter object where we will unmarshal all parameters from the context
	var params ShutdownNodeParams
	// ------------- Optional query parameter "timeout" -------------
	if paramValue := ctx.QueryParam("timeout"); paramValue != "" {

	}

	err = runtime.BindQueryParameter("form", true, false, "timeout", ctx.QueryParams(), &params.Timeout)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter timeout: %s", err))
	}

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.ShutdownNode(ctx, params)
	return err
}

// RegisterHandlers adds each server route to the EchoRouter.
func RegisterHandlers(router interface {
	CONNECT(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	DELETE(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	GET(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	HEAD(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	OPTIONS(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	PATCH(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	POST(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	PUT(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	TRACE(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
}, si ServerInterface, m ...echo.MiddlewareFunc) {

	wrapper := ServerInterfaceWrapper{
		Handler: si,
	}

	router.DELETE("/v2/catchup/:catchpoint", wrapper.AbortCatchup, m...)
	router.POST("/v2/catchup/:catchpoint", wrapper.StartCatchup, m...)
	router.GET("/v2/participation", wrapper.GetParticipationKeys, m...)
	router.POST("/v2/participation", wrapper.AddParticipationKey, m...)
	router.DELETE("/v2/participation/:participation-id", wrapper.DeleteParticipationKeyByID, m...)
	router.GET("/v2/participation/:participation-id", wrapper.GetParticipationKeyByID, m...)
	router.POST("/v2/participation/:participation-id", wrapper.AppendKeys, m...)
	router.POST("/v2/shutdown", wrapper.ShutdownNode, m...)

}

// Base64 encoded, gzipped, json marshaled Swagger object
var swaggerSpec = []string{

	"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",
	"VGP/qr553S5cf7r+/wEAAP//VQaTuKWzAAA=",
}

// GetSwagger returns the Swagger specification corresponding to the generated code
// in this file.
func GetSwagger() (*openapi3.Swagger, error) {
	zipped, err := base64.StdEncoding.DecodeString(strings.Join(swaggerSpec, ""))
	if err != nil {
		return nil, fmt.Errorf("error base64 decoding spec: %s", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %s", err)
	}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(zr)
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %s", err)
	}

	swagger, err := openapi3.NewSwaggerLoader().LoadSwaggerFromData(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("error loading Swagger: %s", err)
	}
	return swagger, nil
}
