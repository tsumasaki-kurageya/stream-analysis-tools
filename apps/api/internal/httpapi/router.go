package httpapi

import (
	"context"
	"net/http"

	openapiv1 "github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/generated/openapiv1"
)

type server struct{}

var _ openapiv1.StrictServerInterface = (*server)(nil)

func (*server) GetHealth(
	context.Context,
	openapiv1.GetHealthRequestObject,
) (openapiv1.GetHealthResponseObject, error) {
	return openapiv1.GetHealth200JSONResponse{
		Component: "main-api",
		Status:    openapiv1.Ok,
	}, nil
}

func NewHandler() http.Handler {
	return openapiv1.Handler(openapiv1.NewStrictHandler(&server{}, nil))
}
