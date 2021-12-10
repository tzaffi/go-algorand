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
	// Delete a given participation key by id
	// (DELETE /v2/participation/{participation-id})
	DeleteParticipationKeyByID(ctx echo.Context, participationId string) error
	// Get participation key info by id
	// (GET /v2/participation/{participation-id})
	GetParticipationKeyByID(ctx echo.Context, participationId string) error

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
	router.POST("/v2/shutdown", wrapper.ShutdownNode, m...)

}

// Base64 encoded, gzipped, json marshaled Swagger object
var swaggerSpec = []string{

	"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",
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
