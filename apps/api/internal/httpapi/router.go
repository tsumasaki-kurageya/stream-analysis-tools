package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	openapiv1 "github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/generated/openapiv1"
	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/streams"
)

const (
	defaultListLimit  = 20
	defaultListOffset = 0
)

type StreamService interface {
	Preview(context.Context, string) (streams.Metadata, error)
	Register(context.Context, string) (streams.Stream, error)
	List(context.Context, streams.ListOptions) ([]streams.Stream, error)
	Get(context.Context, uuid.UUID) (streams.Stream, error)
}

type server struct {
	streams StreamService
}

var _ openapiv1.StrictServerInterface = (*server)(nil)
var _ StreamService = (*streams.Service)(nil)

func (*server) GetHealth(
	context.Context,
	openapiv1.GetHealthRequestObject,
) (openapiv1.GetHealthResponseObject, error) {
	return openapiv1.GetHealth200JSONResponse{
		Component: "main-api",
		Status:    openapiv1.Ok,
	}, nil
}

func (server *server) PreviewStream(
	ctx context.Context,
	request openapiv1.PreviewStreamRequestObject,
) (openapiv1.PreviewStreamResponseObject, error) {
	if request.Body == nil {
		problem, status := problemFor(streams.ErrInvalidYouTubeURL)
		return openapiv1.PreviewStreamdefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: status,
		}, nil
	}

	preview, err := server.streams.Preview(ctx, request.Body.Url)
	if err != nil {
		problem, status := problemFor(err)
		return openapiv1.PreviewStreamdefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: status,
		}, nil
	}
	return openapiv1.PreviewStream200JSONResponse(metadataResponse(preview)), nil
}

func (server *server) CreateStream(
	ctx context.Context,
	request openapiv1.CreateStreamRequestObject,
) (openapiv1.CreateStreamResponseObject, error) {
	if request.Body == nil {
		problem, status := problemFor(streams.ErrInvalidYouTubeURL)
		return openapiv1.CreateStreamdefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: status,
		}, nil
	}

	stream, err := server.streams.Register(ctx, request.Body.Url)
	if err != nil {
		problem, status := problemFor(err)
		return openapiv1.CreateStreamdefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: status,
		}, nil
	}
	location := "/v1/streams/" + stream.ID.String()
	return openapiv1.CreateStream201JSONResponse{
		Body: streamResponse(stream),
		Headers: openapiv1.CreateStream201ResponseHeaders{
			Location: &location,
		},
	}, nil
}

func (server *server) ListStreams(
	ctx context.Context,
	request openapiv1.ListStreamsRequestObject,
) (openapiv1.ListStreamsResponseObject, error) {
	limit := defaultListLimit
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	offset := defaultListOffset
	if request.Params.Offset != nil {
		offset = *request.Params.Offset
	}

	listed, err := server.streams.List(ctx, streams.ListOptions{Limit: limit, Offset: offset})
	if err != nil {
		problem, status := problemFor(err)
		return openapiv1.ListStreamsdefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: status,
		}, nil
	}
	items := make([]openapiv1.Stream, 0, len(listed))
	for _, stream := range listed {
		items = append(items, streamResponse(stream))
	}
	return openapiv1.ListStreams200JSONResponse{
		Items: items, Limit: limit, Offset: offset,
	}, nil
}

func (server *server) GetStream(
	ctx context.Context,
	request openapiv1.GetStreamRequestObject,
) (openapiv1.GetStreamResponseObject, error) {
	streamID, err := uuid.Parse(request.StreamId)
	if err != nil {
		problem := requestProblem()
		return openapiv1.GetStreamdefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: problem.Status,
		}, nil
	}
	stream, err := server.streams.Get(ctx, streamID)
	if err != nil {
		problem, status := problemFor(err)
		return openapiv1.GetStreamdefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: status,
		}, nil
	}
	return openapiv1.GetStream200JSONResponse(streamResponse(stream)), nil
}

func NewHandler(streamService StreamService) http.Handler {
	implementation := &server{streams: streamService}
	strict := openapiv1.NewStrictHandlerWithOptions(
		implementation,
		nil,
		openapiv1.StrictHTTPServerOptions{
			RequestErrorHandlerFunc: func(writer http.ResponseWriter, _ *http.Request, _ error) {
				writeProblem(writer, requestProblem())
			},
			ResponseErrorHandlerFunc: func(writer http.ResponseWriter, _ *http.Request, _ error) {
				writeProblem(writer, internalProblem())
			},
		},
	)
	return openapiv1.HandlerWithOptions(strict, openapiv1.StdHTTPServerOptions{
		ErrorHandlerFunc: func(writer http.ResponseWriter, _ *http.Request, _ error) {
			writeProblem(writer, requestProblem())
		},
	})
}

func metadataResponse(metadata streams.Metadata) openapiv1.StreamPreview {
	return openapiv1.StreamPreview{
		YoutubeVideoId:    metadata.YouTubeVideoID,
		CanonicalUrl:      metadata.CanonicalURL,
		Title:             metadata.Title,
		ChannelId:         metadata.ChannelID,
		ChannelTitle:      metadata.ChannelTitle,
		ThumbnailUrl:      metadata.ThumbnailURL,
		ScheduledStartAt:  metadata.ScheduledStartAt,
		ActualStartAt:     metadata.ActualStartAt,
		ActualEndAt:       metadata.ActualEndAt,
		DurationMs:        durationMilliseconds(metadata.Duration),
		LifecycleStatus:   openapiv1.StreamLifecycleStatus(metadata.LifecycleStatus),
		MetadataFetchedAt: metadata.MetadataFetchedAt,
	}
}

func streamResponse(stream streams.Stream) openapiv1.Stream {
	return openapiv1.Stream{
		Id:                stream.ID.String(),
		YoutubeVideoId:    stream.YouTubeVideoID,
		CanonicalUrl:      stream.CanonicalURL,
		Title:             stream.Title,
		ChannelId:         stream.ChannelID,
		ChannelTitle:      stream.ChannelTitle,
		ThumbnailUrl:      stream.ThumbnailURL,
		ScheduledStartAt:  stream.ScheduledStartAt,
		ActualStartAt:     stream.ActualStartAt,
		ActualEndAt:       stream.ActualEndAt,
		DurationMs:        durationMilliseconds(stream.Duration),
		LifecycleStatus:   openapiv1.StreamLifecycleStatus(stream.LifecycleStatus),
		MetadataFetchedAt: stream.MetadataFetchedAt,
		CreatedAt:         stream.CreatedAt,
		UpdatedAt:         stream.UpdatedAt,
	}
}

func durationMilliseconds(duration *time.Duration) *int64 {
	if duration == nil {
		return nil
	}
	milliseconds := duration.Milliseconds()
	return &milliseconds
}

func problemFor(err error) (openapiv1.ProblemDetails, int) {
	switch {
	case errors.Is(err, streams.ErrInvalidYouTubeURL):
		return openapiv1.ProblemDetails{
			Code: "INVALID_YOUTUBE_URL", Detail: "Provide a supported YouTube video URL.",
			Status: http.StatusBadRequest, Title: "Invalid YouTube URL",
		}, http.StatusBadRequest
	case errors.Is(err, streams.ErrVideoUnavailable):
		return openapiv1.ProblemDetails{
			Code: "VIDEO_UNAVAILABLE", Detail: "The YouTube live stream is unavailable or unsupported.",
			Status: http.StatusUnprocessableEntity, Title: "Video unavailable",
		}, http.StatusUnprocessableEntity
	case errors.Is(err, streams.ErrYouTubeVideoIDExists):
		return openapiv1.ProblemDetails{
			Code: "STREAM_ALREADY_REGISTERED", Detail: "This YouTube stream is already registered.",
			Status: http.StatusConflict, Title: "Stream already registered",
		}, http.StatusConflict
	case errors.Is(err, streams.ErrNotFound):
		return openapiv1.ProblemDetails{
			Code: "STREAM_NOT_FOUND", Detail: "The requested stream does not exist.",
			Status: http.StatusNotFound, Title: "Stream not found",
		}, http.StatusNotFound
	case errors.Is(err, streams.ErrInvalidStream):
		problem := requestProblem()
		return problem, problem.Status
	case errors.Is(err, streams.ErrMetadataProviderUnavailable):
		return openapiv1.ProblemDetails{
			Code: "METADATA_PROVIDER_UNAVAILABLE", Detail: "YouTube metadata could not be retrieved. Try again later.",
			Status: http.StatusServiceUnavailable, Title: "Metadata provider unavailable",
		}, http.StatusServiceUnavailable
	default:
		problem := internalProblem()
		return problem, problem.Status
	}
}

func requestProblem() openapiv1.ProblemDetails {
	return openapiv1.ProblemDetails{
		Code: "INVALID_REQUEST", Detail: "The request could not be parsed or validated.",
		Status: http.StatusBadRequest, Title: "Invalid request",
	}
}

func internalProblem() openapiv1.ProblemDetails {
	return openapiv1.ProblemDetails{
		Code: "INTERNAL_ERROR", Detail: "The request could not be completed.",
		Status: http.StatusInternalServerError, Title: "Internal server error",
	}
}

func writeProblem(writer http.ResponseWriter, problem openapiv1.ProblemDetails) {
	writer.Header().Set("Content-Type", "application/problem+json")
	writer.WriteHeader(problem.Status)
	_ = json.NewEncoder(writer).Encode(problem)
}
