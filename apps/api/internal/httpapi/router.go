package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/collections"
	openapiv1 "github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/generated/openapiv1"
	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/reservations"
	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/streams"
)

const (
	defaultListLimit  = 20
	defaultListOffset = 0
	maxCommandBody    = 4096
)

type StreamService interface {
	Preview(context.Context, string) (streams.Metadata, error)
	Register(context.Context, string) (streams.Stream, error)
	List(context.Context, streams.ListOptions) ([]streams.Stream, error)
	ListItems(context.Context, streams.ListOptions) ([]streams.ListItem, error)
	Get(context.Context, uuid.UUID) (streams.Stream, error)
}

type CollectionService interface {
	Start(context.Context, uuid.UUID) (collections.Job, error)
	Latest(context.Context, uuid.UUID) (collections.Job, error)
	Retry(context.Context, uuid.UUID) (collections.Job, error)
	ListMessages(context.Context, uuid.UUID, int, string) (collections.MessagePage, error)
	SearchMessages(context.Context, uuid.UUID, string, int, string) (collections.MessagePage, error)
}

type ChatActivityService interface {
	ChatActivity(context.Context, uuid.UUID, int) (collections.Activity, error)
}

type ReservationService interface {
	Create(context.Context, string) (reservations.Reservation, error)
	List(context.Context, reservations.ListOptions) ([]reservations.Reservation, int, error)
	Get(context.Context, uuid.UUID) (reservations.Reservation, error)
	Cancel(context.Context, uuid.UUID) (reservations.Reservation, error)
}

type server struct {
	streams      StreamService
	collections  CollectionService
	reservations ReservationService
}

var _ openapiv1.StrictServerInterface = (*server)(nil)
var _ StreamService = (*streams.Service)(nil)
var _ CollectionService = (*collections.Service)(nil)
var _ ReservationService = (*reservations.Service)(nil)

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

	listed, err := server.streams.ListItems(ctx, streams.ListOptions{Limit: limit, Offset: offset})
	if err != nil {
		problem, status := problemFor(err)
		return openapiv1.ListStreamsdefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: status,
		}, nil
	}
	items := make([]openapiv1.StreamListItem, 0, len(listed))
	for _, item := range listed {
		items = append(items, streamListItemResponse(item))
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

func (server *server) StartCollection(
	ctx context.Context,
	request openapiv1.StartCollectionRequestObject,
) (openapiv1.StartCollectionResponseObject, error) {
	streamID, err := uuid.Parse(string(request.StreamId))
	if err != nil {
		problem := requestProblem()
		return openapiv1.StartCollectiondefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: problem.Status,
		}, nil
	}
	job, err := server.collections.Start(ctx, streamID)
	if err != nil {
		problem, status := collectionProblemFor(err)
		return openapiv1.StartCollectiondefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: status,
		}, nil
	}
	location := "/v1/streams/" + job.StreamID.String() + "/collections/latest"
	return openapiv1.StartCollection202JSONResponse{
		Body: collectionJobResponse(job),
		Headers: openapiv1.StartCollection202ResponseHeaders{
			Location: &location,
		},
	}, nil
}

func (server *server) GetLatestCollection(
	ctx context.Context,
	request openapiv1.GetLatestCollectionRequestObject,
) (openapiv1.GetLatestCollectionResponseObject, error) {
	streamID, err := uuid.Parse(string(request.StreamId))
	if err != nil {
		problem := requestProblem()
		return openapiv1.GetLatestCollectiondefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: problem.Status,
		}, nil
	}
	job, err := server.collections.Latest(ctx, streamID)
	if err != nil {
		problem, status := collectionProblemFor(err)
		return openapiv1.GetLatestCollectiondefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: status,
		}, nil
	}
	return openapiv1.GetLatestCollection200JSONResponse(collectionJobResponse(job)), nil
}

func (server *server) RetryCollection(
	ctx context.Context,
	request openapiv1.RetryCollectionRequestObject,
) (openapiv1.RetryCollectionResponseObject, error) {
	jobID, err := uuid.Parse(string(request.JobId))
	if err != nil {
		problem := requestProblem()
		return openapiv1.RetryCollectiondefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: problem.Status,
		}, nil
	}
	job, err := server.collections.Retry(ctx, jobID)
	if err != nil {
		problem, status := collectionProblemFor(err)
		return openapiv1.RetryCollectiondefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: status,
		}, nil
	}
	location := "/v1/streams/" + job.StreamID.String() + "/collections/latest"
	return openapiv1.RetryCollection202JSONResponse{
		Body: collectionJobResponse(job),
		Headers: openapiv1.RetryCollection202ResponseHeaders{
			Location: &location,
		},
	}, nil
}

func (server *server) GetChatActivity(
	ctx context.Context,
	request openapiv1.GetChatActivityRequestObject,
) (openapiv1.GetChatActivityResponseObject, error) {
	streamID, err := uuid.Parse(string(request.StreamId))
	if err != nil {
		problem := requestProblem()
		return openapiv1.GetChatActivitydefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: problem.Status,
		}, nil
	}
	bucketSeconds := 10
	if request.Params.BucketSeconds != nil {
		bucketSeconds = int(*request.Params.BucketSeconds)
	}
	service, ok := server.collections.(ChatActivityService)
	if !ok {
		problem, status := collectionProblemFor(collections.ErrInvalidRequest)
		return openapiv1.GetChatActivitydefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: status,
		}, nil
	}
	activity, err := service.ChatActivity(ctx, streamID, bucketSeconds)
	if err != nil {
		problem, status := collectionProblemFor(err)
		return openapiv1.GetChatActivitydefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: status,
		}, nil
	}
	items := make([]openapiv1.ChatActivityBucket, 0, len(activity.Items))
	for _, item := range activity.Items {
		items = append(items, openapiv1.ChatActivityBucket{
			StartOffsetMilliseconds: item.StartOffsetMilliseconds,
			MessageCount:            item.MessageCount,
		})
	}
	return openapiv1.GetChatActivity200JSONResponse{
		BucketSeconds: openapiv1.ChatActivityBucketSeconds(activity.BucketSeconds),
		Items:         items,
	}, nil
}

func (server *server) ListChatMessages(
	ctx context.Context,
	request openapiv1.ListChatMessagesRequestObject,
) (openapiv1.ListChatMessagesResponseObject, error) {
	streamID, err := uuid.Parse(string(request.StreamId))
	if err != nil {
		problem := requestProblem()
		return openapiv1.ListChatMessagesdefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: problem.Status,
		}, nil
	}
	limit := collections.DefaultMessageLimit
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	cursor := ""
	if request.Params.Cursor != nil {
		cursor = *request.Params.Cursor
	}
	page, err := server.collections.ListMessages(ctx, streamID, limit, cursor)
	if err != nil {
		problem, status := collectionProblemFor(err)
		return openapiv1.ListChatMessagesdefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: status,
		}, nil
	}
	items := make([]openapiv1.ChatMessage, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, openapiv1.ChatMessage{
			Id:                 item.ID.String(),
			AuthorChannelId:    item.AuthorChannelID,
			AuthorDisplayName:  item.AuthorDisplayName,
			MessageText:        item.MessageText,
			PublishedAt:        item.PublishedAt,
			OffsetMilliseconds: item.OffsetMilliseconds,
			MessageType:        openapiv1.ChatMessageMessageType(item.MessageType),
		})
	}
	return openapiv1.ListChatMessages200JSONResponse{
		Items: items, NextCursor: page.NextCursor,
	}, nil
}

func (server *server) SearchChatMessages(
	ctx context.Context,
	request openapiv1.SearchChatMessagesRequestObject,
) (openapiv1.SearchChatMessagesResponseObject, error) {
	streamID, err := uuid.Parse(string(request.StreamId))
	if err != nil {
		problem := requestProblem()
		return openapiv1.SearchChatMessagesdefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: problem.Status,
		}, nil
	}
	limit := collections.DefaultMessageLimit
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	cursor := ""
	if request.Params.Cursor != nil {
		cursor = *request.Params.Cursor
	}
	page, err := server.collections.SearchMessages(ctx, streamID, request.Params.Q, limit, cursor)
	if err != nil {
		problem, status := collectionProblemFor(err)
		return openapiv1.SearchChatMessagesdefaultApplicationProblemPlusJSONResponse{
			Body: problem, StatusCode: status,
		}, nil
	}
	items := make([]openapiv1.ChatMessage, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, openapiv1.ChatMessage{
			Id:                 item.ID.String(),
			AuthorChannelId:    item.AuthorChannelID,
			AuthorDisplayName:  item.AuthorDisplayName,
			MessageText:        item.MessageText,
			PublishedAt:        item.PublishedAt,
			OffsetMilliseconds: item.OffsetMilliseconds,
			MessageType:        openapiv1.ChatMessageMessageType(item.MessageType),
		})
	}
	return openapiv1.SearchChatMessages200JSONResponse{
		Items: items, NextCursor: page.NextCursor,
	}, nil
}

func (server *server) CreateReservation(
	ctx context.Context,
	request openapiv1.CreateReservationRequestObject,
) (openapiv1.CreateReservationResponseObject, error) {
	if request.Body == nil {
		problem, status := reservationProblemFor(reservations.ErrInvalidURL)
		return openapiv1.CreateReservationdefaultApplicationProblemPlusJSONResponse{Body: problem, StatusCode: status}, nil
	}
	created, err := server.reservations.Create(ctx, request.Body.Url)
	if err != nil {
		problem, status := reservationProblemFor(err)
		return openapiv1.CreateReservationdefaultApplicationProblemPlusJSONResponse{Body: problem, StatusCode: status}, nil
	}
	location := "/v1/reservations/" + created.ID.String()
	return openapiv1.CreateReservation201JSONResponse{
		Body:    reservationResponse(created),
		Headers: openapiv1.CreateReservation201ResponseHeaders{Location: &location},
	}, nil
}

func (server *server) ListReservations(
	ctx context.Context,
	request openapiv1.ListReservationsRequestObject,
) (openapiv1.ListReservationsResponseObject, error) {
	limit, offset := defaultListLimit, defaultListOffset
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	if request.Params.Offset != nil {
		offset = *request.Params.Offset
	}
	listed, total, err := server.reservations.List(ctx, reservations.ListOptions{Limit: limit, Offset: offset})
	if err != nil {
		problem, status := reservationProblemFor(err)
		return openapiv1.ListReservationsdefaultApplicationProblemPlusJSONResponse{Body: problem, StatusCode: status}, nil
	}
	items := make([]openapiv1.Reservation, 0, len(listed))
	for _, reservation := range listed {
		items = append(items, reservationResponse(reservation))
	}
	return openapiv1.ListReservations200JSONResponse{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (server *server) GetReservation(
	ctx context.Context,
	request openapiv1.GetReservationRequestObject,
) (openapiv1.GetReservationResponseObject, error) {
	id, err := uuid.Parse(request.ReservationId)
	if err != nil {
		problem := requestProblem()
		return openapiv1.GetReservationdefaultApplicationProblemPlusJSONResponse{Body: problem, StatusCode: problem.Status}, nil
	}
	reservation, err := server.reservations.Get(ctx, id)
	if err != nil {
		problem, status := reservationProblemFor(err)
		return openapiv1.GetReservationdefaultApplicationProblemPlusJSONResponse{Body: problem, StatusCode: status}, nil
	}
	return openapiv1.GetReservation200JSONResponse(reservationResponse(reservation)), nil
}

func (server *server) CancelReservation(
	ctx context.Context,
	request openapiv1.CancelReservationRequestObject,
) (openapiv1.CancelReservationResponseObject, error) {
	id, err := uuid.Parse(request.ReservationId)
	if err != nil {
		problem := requestProblem()
		return openapiv1.CancelReservationdefaultApplicationProblemPlusJSONResponse{Body: problem, StatusCode: problem.Status}, nil
	}
	reservation, err := server.reservations.Cancel(ctx, id)
	if err != nil {
		problem, status := reservationProblemFor(err)
		return openapiv1.CancelReservationdefaultApplicationProblemPlusJSONResponse{Body: problem, StatusCode: status}, nil
	}
	return openapiv1.CancelReservation200JSONResponse(reservationResponse(reservation)), nil
}

func NewHandler(streamService StreamService, collectionServices ...CollectionService) http.Handler {
	var collectionService CollectionService
	if len(collectionServices) > 0 {
		collectionService = collectionServices[0]
	}
	return newHandler(&server{streams: streamService, collections: collectionService})
}

func NewHandlerWithReservations(streamService StreamService, collectionService CollectionService, reservationService ReservationService) http.Handler {
	return newHandler(&server{streams: streamService, collections: collectionService, reservations: reservationService})
}

func newHandler(implementation *server) http.Handler {
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
	handler := openapiv1.HandlerWithOptions(strict, openapiv1.StdHTTPServerOptions{
		ErrorHandlerFunc: func(writer http.ResponseWriter, _ *http.Request, _ error) {
			writeProblem(writer, requestProblem())
		},
	})
	return rejectCollectorOptions(handler)
}

func reservationResponse(reservation reservations.Reservation) openapiv1.Reservation {
	response := openapiv1.Reservation{
		Id: reservation.ID, YoutubeVideoId: reservation.YouTubeVideoID, SourceUrl: reservation.SourceURL,
		State: openapiv1.ReservationState(reservation.State), ScheduledStartAt: reservation.ScheduledStartAt,
		ActualStartAt: reservation.ActualStartAt, ActualEndAt: reservation.ActualEndAt,
		NextCheckAt: reservation.NextCheckAt, LastCheckedAt: reservation.LastCheckedAt,
		MonitorAttempt: reservation.MonitorAttempt, LastErrorCode: reservation.LastErrorCode,
		LastErrorMessage: reservation.LastErrorMessage, LastErrorRetryable: reservation.LastErrorRetryable,
		StreamId: reservation.StreamID, CollectionJobId: reservation.CollectionJobID,
		CanCancel: reservation.CanCancel(), CreatedAt: reservation.CreatedAt, UpdatedAt: reservation.UpdatedAt,
	}
	if reservation.CollectionStatus != nil {
		status := openapiv1.CollectionJobStatus(*reservation.CollectionStatus)
		response.CollectionStatus = &status
	}
	if reservation.CollectionErrorCode != nil {
		safe := collections.PublicErrorFor(*reservation.CollectionErrorCode)
		response.CollectionError = &openapiv1.CollectionError{Code: safe.Code, Message: safe.Message, Retryable: safe.Retryable}
	}
	return response
}

func collectionJobResponse(job collections.Job) openapiv1.CollectionJob {
	response := openapiv1.CollectionJob{
		Id:             job.ID.String(),
		StreamId:       job.StreamID.String(),
		Kind:           openapiv1.CollectionJobKind(job.Kind),
		Status:         openapiv1.CollectionJobStatus(job.PublicStatus()),
		Attempt:        job.Attempt,
		ProcessedCount: job.ProcessedCount,
		SkippedCount:   job.SkippedCount,
		RequestedAt:    job.RequestedAt,
		StartedAt:      job.StartedAt,
		UpdatedAt:      job.UpdatedAt,
		FinishedAt:     job.FinishedAt,
	}
	if step := job.CurrentStep(); step != nil {
		value := openapiv1.CollectionJobCurrentStep(*step)
		response.CurrentStep = &value
	}
	if job.Error != nil {
		response.Error = &openapiv1.CollectionError{
			Code: job.Error.Code, Message: job.Error.Message, Retryable: job.Error.Retryable,
		}
	}
	return response
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

func streamListItemResponse(item streams.ListItem) openapiv1.StreamListItem {
	response := openapiv1.StreamListItem{
		Id:                item.ID.String(),
		YoutubeVideoId:    item.YouTubeVideoID,
		CanonicalUrl:      item.CanonicalURL,
		Title:             item.Title,
		ChannelId:         item.ChannelID,
		ChannelTitle:      item.ChannelTitle,
		ThumbnailUrl:      item.ThumbnailURL,
		ScheduledStartAt:  item.ScheduledStartAt,
		ActualStartAt:     item.ActualStartAt,
		ActualEndAt:       item.ActualEndAt,
		DurationMs:        durationMilliseconds(item.Duration),
		LifecycleStatus:   openapiv1.StreamLifecycleStatus(item.LifecycleStatus),
		MetadataFetchedAt: item.MetadataFetchedAt,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
		ChatMessageCount:  item.ChatMessageCount,
	}
	if item.CollectionStatus != nil {
		status := openapiv1.CollectionJobStatus(*item.CollectionStatus)
		response.CollectionStatus = &status
	}
	return response
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

func collectionProblemFor(err error) (openapiv1.ProblemDetails, int) {
	switch {
	case errors.Is(err, collections.ErrInvalidRequest):
		problem := requestProblem()
		return problem, problem.Status
	case errors.Is(err, collections.ErrStreamNotFound):
		return openapiv1.ProblemDetails{
			Code: "STREAM_NOT_FOUND", Detail: "The requested stream does not exist.",
			Status: http.StatusNotFound, Title: "Stream not found",
		}, http.StatusNotFound
	case errors.Is(err, collections.ErrJobNotFound):
		return openapiv1.ProblemDetails{
			Code: "COLLECTION_JOB_NOT_FOUND", Detail: "The requested collection job does not exist.",
			Status: http.StatusNotFound, Title: "Collection job not found",
		}, http.StatusNotFound
	case errors.Is(err, collections.ErrNotRetryable):
		return openapiv1.ProblemDetails{
			Code: "COLLECTION_NOT_RETRYABLE", Detail: "This collection job cannot be retried.",
			Status: http.StatusConflict, Title: "Collection is not retryable",
		}, http.StatusConflict
	case errors.Is(err, collections.ErrActiveJob):
		return openapiv1.ProblemDetails{
			Code: "COLLECTION_ALREADY_ACTIVE", Detail: "A collection job is already queued or running.",
			Status: http.StatusConflict, Title: "Collection already active",
		}, http.StatusConflict
	default:
		problem := internalProblem()
		return problem, problem.Status
	}
}

func reservationProblemFor(err error) (openapiv1.ProblemDetails, int) {
	switch {
	case errors.Is(err, reservations.ErrInvalidURL):
		return openapiv1.ProblemDetails{Code: "INVALID_RESERVATION_URL", Detail: "Provide a supported YouTube video URL.", Status: http.StatusBadRequest, Title: "Invalid reservation URL"}, http.StatusBadRequest
	case errors.Is(err, reservations.ErrAlreadyActive):
		return openapiv1.ProblemDetails{Code: "RESERVATION_ALREADY_ACTIVE", Detail: "This YouTube video already has an active reservation.", Status: http.StatusConflict, Title: "Reservation already active"}, http.StatusConflict
	case errors.Is(err, reservations.ErrNotFound):
		return openapiv1.ProblemDetails{Code: "RESERVATION_NOT_FOUND", Detail: "The requested reservation does not exist.", Status: http.StatusNotFound, Title: "Reservation not found"}, http.StatusNotFound
	case errors.Is(err, reservations.ErrNotCancellable):
		return openapiv1.ProblemDetails{Code: "RESERVATION_NOT_CANCELLABLE", Detail: "This reservation can no longer be canceled.", Status: http.StatusConflict, Title: "Reservation is not cancellable"}, http.StatusConflict
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

func rejectCollectorOptions(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost ||
			(!strings.HasSuffix(request.URL.Path, "/collections") &&
				!strings.HasSuffix(request.URL.Path, "/retry")) {
			next.ServeHTTP(writer, request)
			return
		}

		body, err := io.ReadAll(io.LimitReader(request.Body, maxCommandBody+1))
		if err != nil || len(body) > maxCommandBody || !emptyCommandBody(body) {
			writeProblem(writer, requestProblem())
			return
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(writer, request)
	})
}

func emptyCommandBody(body []byte) bool {
	if len(bytes.TrimSpace(body)) == 0 {
		return true
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	var object map[string]json.RawMessage
	if err := decoder.Decode(&object); err != nil || object == nil || len(object) != 0 {
		return false
	}
	var trailing any
	return errors.Is(decoder.Decode(&trailing), io.EOF)
}
