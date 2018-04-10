//go:generate swagger -q generate server -f swagger.json --exclude-main
package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"

	"github.com/autarch/metagodoc/api/handlers"
	"github.com/autarch/metagodoc/api/restapi"
	"github.com/autarch/metagodoc/api/restapi/operations"
	"github.com/autarch/metagodoc/elc"
	"github.com/autarch/metagodoc/env"
	"github.com/autarch/metagodoc/logger"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"
	flags "github.com/jessevdk/go-flags"
	"github.com/olivere/elastic"
	"github.com/tylerb/graceful"
)

func main() {
	swaggerSpec, err := loads.Embedded(restapi.SwaggerJSON, restapi.FlatSwaggerJSON)
	if err != nil {
		log.Fatalln(err)
	}

	l, err := logger.New(logger.NewParams{IsProd: env.IsProd()})
	if err != nil {
		log.Fatal(err)
	}

	api := operations.NewMetaGodocAPI(swaggerSpec)
	server := restapi.NewServer(api)
	defer server.Shutdown()

	parser := flags.NewParser(server, flags.Default)
	parser.ShortDescription = "MetaGodoc REST API"
	parser.LongDescription = swaggerSpec.Spec().Info.Description

	server.ConfigureFlags()
	for _, optsGroup := range api.CommandLineOptionsGroups {
		_, err := parser.AddGroup(optsGroup.ShortDescription, optsGroup.LongDescription, optsGroup.Options)
		if err != nil {
			log.Fatalln(err)
		}
	}

	if _, err := parser.Parse(); err != nil {
		code := 1
		if fe, ok := err.(*flags.Error); ok {
			if fe.Type == flags.ErrHelp {
				code = 0
			}
		}
		os.Exit(code)
	}

	el, err := elc.NewClient(env.TraceElastic(), l)
	if err != nil {
		log.Fatalln(err)
	}

	configureAPI(api, l, el)

	if err := server.Serve(); err != nil {
		log.Fatalln(err)
	}
}

func configureAPI(api *operations.MetaGodocAPI, l *logger.Logger, el *elastic.Client) http.Handler {
	// configure the api here
	api.ServeError = errors.ServeError

	// Set your custom logger if needed. Default one is log.Printf
	// Expected interface func(string, ...interface{})
	//
	// Example:
	// api.Logger = log.Printf

	api.JSONConsumer = runtime.JSONConsumer()

	api.JSONProducer = runtime.JSONProducer()

	h := handlers.New(l, el)
	api.GetRepositoryRepositoryHandler = operations.GetRepositoryRepositoryHandlerFunc(func(params operations.GetRepositoryRepositoryParams) middleware.Responder {
		return h.GetRepository(params)
	})
	api.GetRepositoryRepositoryRefRefHandler = operations.GetRepositoryRepositoryRefRefHandlerFunc(func(params operations.GetRepositoryRepositoryRefRefParams) middleware.Responder {
		return middleware.NotImplemented("operation .GetRepositoryRepositoryRefRef has not yet been implemented")
	})
	api.GetRepositoryRepositoryRefRefPackagePackageHandler = operations.GetRepositoryRepositoryRefRefPackagePackageHandlerFunc(func(params operations.GetRepositoryRepositoryRefRefPackagePackageParams) middleware.Responder {
		return middleware.NotImplemented("operation .GetRepositoryRepositoryRefRefPackagePackage has not yet been implemented")
	})
	api.GetSearchHandler = operations.GetSearchHandlerFunc(func(params operations.GetSearchParams) middleware.Responder {
		return middleware.NotImplemented("operation .GetSearch has not yet been implemented")
	})

	api.ServerShutdown = func() {}

	return setupGlobalMiddleware(api.Serve(setupMiddlewares))
}

// The TLS configuration before HTTPS server starts.
func configureTLS(tlsConfig *tls.Config) {
	// Make all necessary changes to the TLS configuration here.
}

// As soon as server is initialized but not run yet, this function will be called.
// If you need to modify a config, store server instance to stop it individually later, this is the place.
// This function can be called multiple times, depending on the number of serving schemes.
// scheme value will be set accordingly: "http", "https" or "unix"
func configureServer(s *graceful.Server, scheme, addr string) {
}

// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
// The middleware executes after routing but before authentication, binding and validation
func setupMiddlewares(handler http.Handler) http.Handler {
	return handler
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics
func setupGlobalMiddleware(handler http.Handler) http.Handler {
	return handler
}
