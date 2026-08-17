from pathlib import Path

path = Path("apps/api/internal/httpapi/router.go")
text = path.read_text()
text = text.replace(
'''type StreamService interface {
\tPreview(context.Context, string) (streams.Metadata, error)
\tRegister(context.Context, string) (streams.Stream, error)
\tList(context.Context, streams.ListOptions) ([]streams.Stream, error)
\tGet(context.Context, uuid.UUID) (streams.Stream, error)
}''',
'''type StreamService interface {
\tPreview(context.Context, string) (streams.Metadata, error)
\tRegister(context.Context, string) (streams.Stream, error)
\tList(context.Context, streams.ListOptions) ([]streams.Stream, error)
\tListItems(context.Context, streams.ListOptions) ([]streams.ListItem, error)
\tGet(context.Context, uuid.UUID) (streams.Stream, error)
}''')
text = text.replace(
'''\tlisted, err := server.streams.List(ctx, streams.ListOptions{Limit: limit, Offset: offset})
\tif err != nil {
\t\tproblem, status := problemFor(err)
\t\treturn openapiv1.ListStreamsdefaultApplicationProblemPlusJSONResponse{
\t\t\tBody: problem, StatusCode: status,
\t\t}, nil
\t}
\titems := make([]openapiv1.Stream, 0, len(listed))
\tfor _, stream := range listed {
\t\titems = append(items, streamResponse(stream))
\t}
\treturn openapiv1.ListStreams200JSONResponse{
\t\tItems: items, Limit: limit, Offset: offset,
\t}, nil''',
'''\tlisted, err := server.streams.ListItems(ctx, streams.ListOptions{Limit: limit, Offset: offset})
\tif err != nil {
\t\tproblem, status := problemFor(err)
\t\treturn openapiv1.ListStreamsdefaultApplicationProblemPlusJSONResponse{
\t\t\tBody: problem, StatusCode: status,
\t\t}, nil
\t}
\titems := make([]openapiv1.StreamListItem, 0, len(listed))
\tfor _, item := range listed {
\t\titems = append(items, streamListItemResponse(item))
\t}
\treturn openapiv1.ListStreams200JSONResponse{
\t\tItems: items, Limit: limit, Offset: offset,
\t}, nil''')
needle = '''func durationMilliseconds(duration *time.Duration) *int64 {'''
mapper = '''func streamListItemResponse(item streams.ListItem) openapiv1.StreamListItem {
\tresponse := openapiv1.StreamListItem{
\t\tId:                item.ID.String(),
\t\tYoutubeVideoId:    item.YouTubeVideoID,
\t\tCanonicalUrl:      item.CanonicalURL,
\t\tTitle:             item.Title,
\t\tChannelId:         item.ChannelID,
\t\tChannelTitle:      item.ChannelTitle,
\t\tThumbnailUrl:      item.ThumbnailURL,
\t\tScheduledStartAt:  item.ScheduledStartAt,
\t\tActualStartAt:     item.ActualStartAt,
\t\tActualEndAt:       item.ActualEndAt,
\t\tDurationMs:        durationMilliseconds(item.Duration),
\t\tLifecycleStatus:   openapiv1.StreamLifecycleStatus(item.LifecycleStatus),
\t\tMetadataFetchedAt: item.MetadataFetchedAt,
\t\tCreatedAt:         item.CreatedAt,
\t\tUpdatedAt:         item.UpdatedAt,
\t\tChatMessageCount:  item.ChatMessageCount,
\t}
\tif item.CollectionStatus != nil {
\t\tstatus := openapiv1.CollectionJobStatus(*item.CollectionStatus)
\t\tresponse.CollectionStatus = &status
\t}
\treturn response
}

'''
if mapper not in text:
    text = text.replace(needle, mapper + needle)
path.write_text(text)
