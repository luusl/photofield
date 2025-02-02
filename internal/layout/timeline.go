package layout

import (
	"log"
	"time"

	"github.com/hako/durafmt"
	"github.com/tdewolff/canvas"

	"photofield/internal/collection"
	"photofield/internal/image"
	"photofield/internal/metrics"
	"photofield/internal/render"
)

type TimelineEvent struct {
	StartTime  time.Time
	EndTime    time.Time
	First      bool
	FirstOnDay bool
	LastOnDay  bool
	Section    Section
}

func LayoutTimelineEvent(layout Layout, rect render.Rect, event *TimelineEvent, scene *render.Scene, source *image.Source) render.Rect {

	// log.Println("layout event", len(event.Section.photos), rect.X, rect.Y)

	textHeight := 30.
	textBounds := render.Rect{
		X: rect.X,
		Y: rect.Y,
		W: rect.W,
		H: textHeight,
	}

	startTimeFormat := "Mon, Jan 2"
	if event.StartTime.Year() != time.Now().Year() {
		startTimeFormat += ", 2006"
	}

	startTimeFormat += "   15:04"

	headerText := event.StartTime.Format(startTimeFormat)

	duration := event.EndTime.Sub(event.StartTime)
	if duration >= 1*time.Minute {
		dur := durafmt.Parse(duration)
		headerText += "   " + dur.LimitFirstN(1).String()
	}

	font := scene.Fonts.Main.Face(40, canvas.Black, canvas.FontRegular, canvas.FontNormal)

	scene.Texts = append(scene.Texts,
		render.NewTextFromRect(
			textBounds,
			&font,
			headerText,
		),
	)
	rect.Y += textHeight + 15

	photos := addSectionPhotos(&event.Section, scene, source)
	newBounds := layoutSectionPhotos(photos, rect, layout, scene, source)

	rect.Y = newBounds.Y + newBounds.H
	rect.Y += 10
	return rect
}

func LayoutTimeline(layout Layout, collection collection.Collection, scene *render.Scene, source *image.Source) {

	limit := collection.Limit

	infos := collection.GetInfos(source, image.ListOptions{
		OrderBy: image.DateDesc,
		Limit:   limit,
	})

	layout.ImageSpacing = 0.02 * layout.ImageHeight
	layout.LineSpacing = 0.02 * layout.ImageHeight

	sceneMargin := 10.

	scene.Bounds.W = layout.SceneWidth

	event := TimelineEvent{}
	eventCount := 0
	var lastPhotoTime time.Time

	rect := render.Rect{
		X: sceneMargin,
		Y: sceneMargin,
		W: scene.Bounds.W - sceneMargin*2,
		H: 0,
	}

	scene.Solids = make([]render.Solid, 0)
	scene.Texts = make([]render.Text, 0)

	layoutPlaced := metrics.Elapsed("layout placing")
	layoutCounter := metrics.Counter{
		Name:     "layout",
		Interval: 1 * time.Second,
	}

	index := 0
	for info := range infos {
		if limit > 0 && index >= limit {
			break
		}

		photoTime := info.DateTime
		elapsed := lastPhotoTime.Sub(photoTime)
		if elapsed > 30*time.Minute {
			event.StartTime = lastPhotoTime
			rect = LayoutTimelineEvent(layout, rect, &event, scene, source)
			eventCount++
			event = TimelineEvent{
				EndTime: photoTime,
			}
		}
		lastPhotoTime = photoTime

		event.Section.infos = append(event.Section.infos, info)

		layoutCounter.Set(index)
		index++
	}
	layoutPlaced()

	if len(event.Section.infos) > 0 {
		event.StartTime = lastPhotoTime
		rect = LayoutTimelineEvent(layout, rect, &event, scene, source)
		eventCount++
	}

	log.Printf("layout events %d\n", eventCount)

	scene.Bounds.H = rect.Y + sceneMargin
	scene.RegionSource = PhotoRegionSource{
		Source: source,
	}

}
